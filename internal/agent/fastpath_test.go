package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/tools"
)

func TestSectionRequestRe_Matches(t *testing.T) {
	// want is the CLEANED section name — what the tool will receive. Natural
	// speech filler ("de", "por favor", "en inglés"…) must not reach the
	// heading matcher, or "la sección de resultados" never finds "Results".
	cases := []struct {
		in   string
		want string
	}{
		{"léeme la sección introducción", "introducción"},
		{"Léeme la sección Introducción.", "Introducción"},
		{"leeme la seccion introducción", "introducción"}, // STT drops accents
		{"Lee la sección de resultados", "resultados"},
		{"Léame la sección conclusiones", "conclusiones"}, // formal usted
		{"léeme la sección 3", "3"},
		{"léeme la sección introducción por favor", "introducción"},
		{"léeme la sección de la introducción del pdf por favor", "introducción"},
		{"léeme la sección resumen en su idioma original", "resumen"},
		{"léeme la sección métodos en inglés", "métodos"},
	}
	for _, c := range cases {
		m := sectionRequestRe.FindStringSubmatch(c.in)
		if m == nil {
			t.Errorf("expected match on %q", c.in)
			continue
		}
		got := cleanSectionName(m[1])
		if got != c.want {
			t.Errorf("for %q: got section %q, want %q", c.in, got, c.want)
		}
	}
}

// TestCleanSectionName_AllFiller: an utterance whose "section" is pure filler
// must clean to empty so the fast path declines instead of querying "".
func TestCleanSectionName_AllFiller(t *testing.T) {
	if got := cleanSectionName("del pdf por favor"); got != "" {
		t.Errorf("expected empty after cleaning pure filler, got %q", got)
	}
}

func TestSectionRequestRe_NoMatch(t *testing.T) {
	// Common requests that must NOT trigger the PDF fast path.
	misses := []string{
		"abre Mensajes",
		"qué hora es",
		"léeme el último mensaje",
		"léeme esta página",
		"escribe en el chat",
		"explícame la sección que estoy leyendo", // verb is explícame, not léeme/lee
	}
	for _, in := range misses {
		if sectionRequestRe.FindStringSubmatch(in) != nil {
			t.Errorf("must NOT match %q (fast path would misfire)", in)
		}
	}
}

// TestFastPath_OneBrainRoundInsteadOfTwo is the latency contract: when the
// fast path fires, the brain is called ONCE (the response round) instead of
// twice (router + response). This is the ~2s savings on a ~5s turn that the
// live capture identified.
func TestFastPath_OneBrainRoundInsteadOfTwo(t *testing.T) {
	reg := tools.NewRegistry()
	probe := &programmableTool{
		name:   "read_open_pdf",
		result: "Introduction\nTransistors amplify signals…",
	}
	reg.Register(probe)

	// One brain response is enough — the response round.
	b := &scriptedBrain{queue: []brain.Response{
		{Text: "La introducción dice que los transistores amplifican señales."},
	}}
	orch := New(nil, b, reg, "system", quietLogger())

	reply, err := orch.HandleUtterance(context.Background(), "léeme la sección introducción")
	if err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if probe.called != 1 {
		t.Errorf("fast path must call the tool exactly once, got %d", probe.called)
	}
	if b.calls != 1 {
		t.Errorf("must be only 1 brain round (the response), got %d — the routing round was not eliminated", b.calls)
	}
	if !strings.Contains(reply, "transistores") {
		t.Errorf("expected the brain's response based on the tool result, got %q", reply)
	}
}

// TestFastPath_UniqueToolCallIDs: hitting the fast path twice in one session
// must produce distinct synthetic tool-call ids — Anthropic rejects a history
// where two tool_use blocks share an id.
func TestFastPath_UniqueToolCallIDs(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&programmableTool{name: "read_open_pdf", result: "texto"})
	b := &scriptedBrain{queue: []brain.Response{{Text: "uno"}, {Text: "dos"}}}
	orch := New(nil, b, reg, "system", quietLogger())

	for _, u := range []string{"léeme la sección introducción", "léeme la sección resumen"} {
		if _, err := orch.HandleUtterance(context.Background(), u); err != nil {
			t.Fatalf("HandleUtterance(%q): %v", u, err)
		}
	}
	seen := map[string]bool{}
	for _, m := range orch.history {
		for _, tc := range m.ToolCalls {
			if seen[tc.ID] {
				t.Errorf("duplicate tool-call id %q in history", tc.ID)
			}
			seen[tc.ID] = true
		}
	}
}

// TestFastPath_DoesNotFireOnUnrelated guards against false positives — a
// non-matching utterance must leave routing to the brain (normal 2 rounds
// when the brain itself asks for a tool).
func TestFastPath_DoesNotFireOnUnrelated(t *testing.T) {
	reg := tools.NewRegistry()
	probe := &programmableTool{name: "read_open_pdf", result: "should not run"}
	reg.Register(probe)

	b := &scriptedBrain{queue: []brain.Response{{Text: "Listo."}}}
	orch := New(nil, b, reg, "system", quietLogger())

	if _, err := orch.HandleUtterance(context.Background(), "abre Mensajes"); err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if probe.called != 0 {
		t.Errorf("fast path fired on unrelated utterance, tool called %d times", probe.called)
	}
}

// TestFastPath_DegradesWhenToolMissing: if read_open_pdf isn't registered the
// fast path must silently no-op so the orchestrator can fall back to the
// normal brain loop instead of hanging or panicking.
func TestFastPath_DegradesWhenToolMissing(t *testing.T) {
	reg := tools.NewRegistry() // no read_open_pdf
	b := &scriptedBrain{queue: []brain.Response{{Text: "no puedo leer PDFs aquí"}}}
	orch := New(nil, b, reg, "system", quietLogger())

	reply, err := orch.HandleUtterance(context.Background(), "léeme la sección introducción")
	if err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if reply == "" {
		t.Error("expected the normal loop to still produce a reply")
	}
}
