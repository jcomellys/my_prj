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
		// The C-020 canonical and natural phrases, verbatim:
		{"Léeme la sección introducción.", "introducción"},
		{"Léeme la sección de la introducción, por favor.", "introducción"},
		{"Lee la sección de introducción.", "introducción"},
		// Case / accent / punctuation variants:
		{"léeme la sección introducción", "introducción"},
		{"LÉEME LA SECCIÓN INTRODUCCIÓN!", "INTRODUCCIÓN"},
		{"Léeme la sección Introducción.", "Introducción"},
		{"leeme la seccion introducción", "introducción"}, // STT drops accents
		{"leeme la seccion introduccion", "introduccion"}, // STT drops all accents
		{"Lee la sección de resultados", "resultados"},
		{"Léame la sección conclusiones", "conclusiones"}, // formal usted
		{"léeme la sección 3", "3"},
		{"léeme la sección introducción por favor", "introducción"},
		{"léeme la sección de la introducción del pdf por favor", "introducción"},
		{"léeme la sección resumen en su idioma original", "resumen"},
		{"léeme la sección métodos en inglés", "métodos"},
		{"Léeme la sección de la introducción, por favor", "introducción"}, // no final period
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

// TestFastPath_RoutesResponseToFastBrain: with a fast brain configured, the
// response round after a fast path runs on it — that round is a narration/
// translation job and was the 5.6s neck measured live (X-017).
func TestFastPath_RoutesResponseToFastBrain(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&programmableTool{name: "read_open_pdf", result: "Introduction\ntext"})

	main := &recordingBrain{name: "openai:gpt-5", queue: []brain.Response{{Text: "no debería responder yo"}}}
	fast := &recordingBrain{name: "openai:gpt-5-mini", queue: []brain.Response{{Text: "La introducción dice…"}}}
	orch := New(nil, main, reg, "system", quietLogger()).WithFastBrain(fast)

	reply, err := orch.HandleUtterance(context.Background(), "léeme la sección introducción")
	if err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if fast.calls != 1 {
		t.Errorf("fast brain must answer the fast-path turn, got %d calls", fast.calls)
	}
	if main.calls != 0 {
		t.Errorf("main brain must not run on a fast-path turn when fast is set, got %d calls", main.calls)
	}
	if !strings.Contains(reply, "introducción") {
		t.Errorf("unexpected reply %q", reply)
	}
}

// TestFastPath_NoFastBrainOnNormalTurn: a non-fast-path turn must stay on the
// main brain even when a fast brain is configured.
func TestFastPath_NoFastBrainOnNormalTurn(t *testing.T) {
	main := &recordingBrain{name: "openai:gpt-5", queue: []brain.Response{{Text: "listo"}}}
	fast := &recordingBrain{name: "openai:gpt-5-mini"}
	orch := New(nil, main, tools.NewRegistry(), "system", quietLogger()).WithFastBrain(fast)

	if _, err := orch.HandleUtterance(context.Background(), "abre Mensajes"); err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if main.calls != 1 || fast.calls != 0 {
		t.Errorf("normal turn must use the main brain: main=%d fast=%d", main.calls, fast.calls)
	}
}

// ctxErrTool fails with the context's error — what a real tool does when a
// barge-in cancels it mid-flight.
type ctxErrTool struct{ called int }

func (c *ctxErrTool) Spec() brain.ToolSpec {
	return brain.ToolSpec{Name: "read_open_pdf", Description: "t", Schema: map[string]any{"type": "object"}}
}
func (c *ctxErrTool) Execute(ctx context.Context, _ string) (tools.Result, error) {
	c.called++
	return tools.Result{}, ctx.Err()
}

// TestFastPath_BargeInIsCleanCancellation: a hotkey barge-in during the
// fast-path tool must NOT splice an ERROR tool result into history (it was a
// human action, not a tool failure) and must not count as a fired fast path.
func TestFastPath_BargeInIsCleanCancellation(t *testing.T) {
	reg := tools.NewRegistry()
	tool := &ctxErrTool{}
	reg.Register(tool)

	b := &scriptedBrain{queue: []brain.Response{{Text: "irrelevante"}}}
	orch := New(nil, b, reg, "system", quietLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // barge-in already happened
	_, _ = orch.HandleUtterance(ctx, "léeme la sección introducción")

	if tool.called != 1 {
		t.Fatalf("tool should have been attempted once, got %d", tool.called)
	}
	for _, m := range orch.history {
		if m.Role == brain.RoleTool {
			t.Errorf("cancelled fast path must not leave a tool result in history, found %q", m.Content)
		}
		if len(m.ToolCalls) != 0 {
			t.Errorf("cancelled fast path must not leave a synthetic tool_call in history")
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
