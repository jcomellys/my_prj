package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/tools"
)

// ---------- test doubles ----------

// scriptedBrain returns a queued response per call, so tests can describe
// "first call: ask for a tool, second call: produce final text".
type scriptedBrain struct {
	queue []brain.Response
	calls int
}

func (s *scriptedBrain) Name() string { return "scripted" }

func (s *scriptedBrain) Chat(_ context.Context, _ []brain.Message, _ []brain.ToolSpec) (*brain.Response, error) {
	if s.calls >= len(s.queue) {
		// Default: end the loop with empty text.
		return &brain.Response{Text: "(end)"}, nil
	}
	r := s.queue[s.calls]
	s.calls++
	return &r, nil
}

// programmableTool returns a fixed result for tests.
type programmableTool struct {
	name   string
	result string
	err    error
	called int
}

func (p *programmableTool) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name:        p.name,
		Description: "test tool",
		Schema:      map[string]any{"type": "object"},
	}
}

func (p *programmableTool) Execute(_ context.Context, _ string) (string, error) {
	p.called++
	return p.result, p.err
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------- tests ----------

func TestOrchestrator_PlainReplyEndsLoop(t *testing.T) {
	b := &scriptedBrain{queue: []brain.Response{
		{Text: "hola"},
	}}
	orch := New(nil /* voice */, b, tools.NewRegistry(), "system", quietLogger())
	reply, err := orch.HandleUtterance(context.Background(), "hi")
	if err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if reply != "hola" {
		t.Errorf("expected reply hola, got %q", reply)
	}
	if b.calls != 1 {
		t.Errorf("expected exactly one brain call, got %d", b.calls)
	}
}

func TestOrchestrator_RunsToolThenSummarizes(t *testing.T) {
	reg := tools.NewRegistry()
	probe := &programmableTool{name: "probe", result: "OK"}
	reg.Register(probe)

	b := &scriptedBrain{queue: []brain.Response{
		{ToolCalls: []brain.ToolCall{{ID: "c1", Name: "probe", Arguments: "{}"}}},
		{Text: "Listo."},
	}}
	orch := New(nil, b, reg, "system", quietLogger())
	reply, err := orch.HandleUtterance(context.Background(), "do it")
	if err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if reply != "Listo." {
		t.Errorf("expected final reply Listo., got %q", reply)
	}
	if probe.called != 1 {
		t.Errorf("expected tool called once, got %d", probe.called)
	}
	if b.calls != 2 {
		t.Errorf("expected two brain calls (tool round + final), got %d", b.calls)
	}
}

func TestOrchestrator_UnknownToolBubblesAsToolResult(t *testing.T) {
	reg := tools.NewRegistry()

	b := &scriptedBrain{queue: []brain.Response{
		{ToolCalls: []brain.ToolCall{{ID: "c1", Name: "nope", Arguments: "{}"}}},
		{Text: "I gave up."},
	}}
	orch := New(nil, b, reg, "system", quietLogger())
	reply, err := orch.HandleUtterance(context.Background(), "go")
	if err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if !strings.Contains(reply, "gave up") {
		t.Errorf("expected brain's recovery reply, got %q", reply)
	}
	// Verify the tool error was appended to history so the brain saw it.
	found := false
	for _, m := range orch.history {
		if m.Role == brain.RoleTool && strings.Contains(m.Content, "ERROR") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ERROR message appended for unknown tool")
	}
}

func TestOrchestrator_MaxRoundsCap(t *testing.T) {
	reg := tools.NewRegistry()
	probe := &programmableTool{name: "loop", result: "again"}
	reg.Register(probe)

	// Brain that keeps asking for the same tool forever.
	infiniteLoop := []brain.Response{}
	for i := 0; i < 20; i++ {
		infiniteLoop = append(infiniteLoop, brain.Response{
			ToolCalls: []brain.ToolCall{{ID: "x", Name: "loop", Arguments: "{}"}},
		})
	}
	b := &scriptedBrain{queue: infiniteLoop}
	orch := New(nil, b, reg, "system", quietLogger())
	reply, err := orch.HandleUtterance(context.Background(), "loop forever")
	if err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if !strings.Contains(reply, "razonable") {
		t.Errorf("expected give-up message, got %q", reply)
	}
	if probe.called > orch.MaxRounds {
		t.Errorf("tool called more than MaxRounds: %d > %d", probe.called, orch.MaxRounds)
	}
}

func TestOrchestrator_HistoryGrowsAcrossUtterances(t *testing.T) {
	b := &scriptedBrain{queue: []brain.Response{
		{Text: "a"},
		{Text: "b"},
	}}
	orch := New(nil, b, tools.NewRegistry(), "system", quietLogger())
	_, _ = orch.HandleUtterance(context.Background(), "one")
	_, _ = orch.HandleUtterance(context.Background(), "two")

	// system + 2 user + 2 assistant = 5
	if got := len(orch.history); got != 5 {
		t.Errorf("expected history length 5, got %d", got)
	}
}
