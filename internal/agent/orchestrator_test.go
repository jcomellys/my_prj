package agent

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/cost"
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

// recordingBrain is a scripted brain that also records, per call, whether
// the escalate tool spec was offered to it. Lets tests assert escalation
// routing and that escalate is offered to the cheap brain only.
type recordingBrain struct {
	name            string
	queue           []brain.Response
	calls           int
	sawEscalateSpec []bool
}

func (b *recordingBrain) Name() string { return b.name }

func (b *recordingBrain) Chat(_ context.Context, _ []brain.Message, specs []brain.ToolSpec) (*brain.Response, error) {
	saw := false
	for _, s := range specs {
		if s.Name == escalateToolName {
			saw = true
		}
	}
	b.sawEscalateSpec = append(b.sawEscalateSpec, saw)
	if b.calls >= len(b.queue) {
		return &brain.Response{Text: "(end)"}, nil
	}
	r := b.queue[b.calls]
	b.calls++
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

func (p *programmableTool) Execute(_ context.Context, _ string) (tools.Result, error) {
	p.called++
	return tools.Result{Text: p.result}, p.err
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

// TestOrchestrator_MockBrainOpenAppEndsCleanly reproduces the fase 0.1
// smoke test on the Mac: user says "abre Chrome", mock brain issues the
// open_app tool call, the tool runs, mock brain summarizes — single round,
// no loop. Guards against a bug seen 2026-05-19 where the mock kept
// re-issuing the same tool until MaxRounds.
func TestOrchestrator_MockBrainOpenAppEndsCleanly(t *testing.T) {
	reg := tools.NewRegistry()
	probe := &programmableTool{name: "open_app", result: "Opened Chrome."}
	reg.Register(probe)

	b := brain.NewMock()
	orch := New(nil, b, reg, "sys", quietLogger())
	reply, err := orch.HandleUtterance(context.Background(), "abre Chrome")
	if err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if probe.called != 1 {
		t.Errorf("expected tool called exactly once, got %d", probe.called)
	}
	if !strings.Contains(reply, "Opened Chrome") {
		t.Errorf("expected reply to mention tool result, got %q", reply)
	}
}

func TestOrchestrator_EscalatesToDeepBrain(t *testing.T) {
	reg := tools.NewRegistry()

	cheap := &recordingBrain{name: "openai:gpt-5-mini", queue: []brain.Response{
		{ToolCalls: []brain.ToolCall{{ID: "e1", Name: escalateToolName, Arguments: `{"reason":"hard"}`}},
			Usage: brain.Usage{InputTokens: 100, OutputTokens: 10}},
	}}
	deep := &recordingBrain{name: "openai:gpt-5", queue: []brain.Response{
		{Text: "Resuelto por el experto.", Usage: brain.Usage{InputTokens: 500, OutputTokens: 200}},
	}}

	dir := t.TempDir()
	tr, _ := cost.NewTracker(filepath.Join(dir, "cost.log"))
	orch := New(nil, cheap, reg, "sys", quietLogger()).WithDeepBrain(deep).WithCost(tr)

	reply, err := orch.HandleUtterance(context.Background(), "analiza este problema complejo")
	if err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if reply != "Resuelto por el experto." {
		t.Errorf("expected deep brain reply, got %q", reply)
	}
	if cheap.calls != 1 {
		t.Errorf("cheap brain called %d times, want 1 (just the escalate turn)", cheap.calls)
	}
	if deep.calls != 1 {
		t.Errorf("deep brain called %d times, want 1", deep.calls)
	}
	if !cheap.sawEscalateSpec[0] {
		t.Error("cheap brain should be offered the escalate tool")
	}
	if deep.sawEscalateSpec[0] {
		t.Error("deep brain must NOT be offered the escalate tool")
	}

	// Cost must attribute to both tiers by name.
	s, _ := tr.Since(time.Now().Add(-time.Hour))
	if s.Entries != 2 {
		t.Fatalf("expected 2 cost entries (cheap + deep), got %d", s.Entries)
	}
	if s.InputTokens != 600 || s.OutputTokens != 210 {
		t.Errorf("token aggregation across tiers wrong: %+v", s)
	}
}

func TestOrchestrator_NoEscalateSpecWhenSingleTier(t *testing.T) {
	b := &recordingBrain{name: "mock", queue: []brain.Response{{Text: "hola"}}}
	orch := New(nil, b, tools.NewRegistry(), "sys", quietLogger())
	if _, err := orch.HandleUtterance(context.Background(), "hi"); err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}
	if b.sawEscalateSpec[0] {
		t.Error("escalate tool must not be offered when no deep brain is configured")
	}
}

func TestOrchestrator_RecordsCostPerBrainCall(t *testing.T) {
	// Brain that names itself as a known-priced model.
	type priced struct {
		resp brain.Response
	}
	b := &pricedBrain{
		name: "openai:gpt-5",
		resp: brain.Response{
			Text: "done",
			Usage: brain.Usage{
				InputTokens:       1000,
				OutputTokens:      500,
				CachedInputTokens: 200,
			},
		},
	}

	dir := t.TempDir()
	tr, err := cost.NewTracker(filepath.Join(dir, "cost.log"))
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	orch := New(nil, b, tools.NewRegistry(), "sys", quietLogger()).WithCost(tr)

	if _, err := orch.HandleUtterance(context.Background(), "hi"); err != nil {
		t.Fatalf("HandleUtterance: %v", err)
	}

	s, err := tr.Since(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Since: %v", err)
	}
	if s.Entries != 1 {
		t.Fatalf("expected 1 cost entry, got %d", s.Entries)
	}
	if s.InputTokens != 1000 || s.OutputTokens != 500 || s.CachedTokens != 200 {
		t.Errorf("token aggregation wrong: %+v", s)
	}
	// Pricing for openai:gpt-5 in cost.DefaultPrices: in=$5/M, out=$20/M, cached=$0.50/M.
	// regular_in = 800. usd = (800*5 + 200*0.5 + 500*20) / 1e6 = 0.0141.
	wantUSD := (800.0*5 + 200.0*0.5 + 500.0*20) / 1_000_000.0
	if absf(s.USD-wantUSD) > 1e-9 {
		t.Errorf("USD aggregation wrong: got %v, want %v", s.USD, wantUSD)
	}
}

func absf(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// pricedBrain returns the same Response every call. Distinct from
// scriptedBrain so the cost test can pre-configure a Usage.
type pricedBrain struct {
	name string
	resp brain.Response
}

func (p *pricedBrain) Name() string { return p.name }
func (p *pricedBrain) Chat(_ context.Context, _ []brain.Message, _ []brain.ToolSpec) (*brain.Response, error) {
	r := p.resp
	return &r, nil
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
