package subagent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/tools"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// scriptedBrain returns queued responses, then a default final text.
type scriptedBrain struct {
	name  string
	queue []brain.Response
	calls int
}

func (s *scriptedBrain) Name() string { return s.name }
func (s *scriptedBrain) Chat(_ context.Context, _ []brain.Message, _ []brain.ToolSpec) (*brain.Response, error) {
	if s.calls >= len(s.queue) {
		return &brain.Response{Text: "(fin)"}, nil
	}
	r := s.queue[s.calls]
	s.calls++
	return &r, nil
}

type probeTool struct {
	name   string
	result string
	called int
}

func (p *probeTool) Spec() brain.ToolSpec {
	return brain.ToolSpec{Name: p.name, Description: "probe", Schema: map[string]any{"type": "object"}}
}
func (p *probeTool) Execute(_ context.Context, _ string) (tools.Result, error) {
	p.called++
	return tools.Result{Text: p.result}, nil
}

func TestRunner_RunsToolsThenSummarizes(t *testing.T) {
	reg := tools.NewRegistry()
	probe := &probeTool{name: "do_thing", result: "done"}
	reg.Register(probe)

	b := &scriptedBrain{name: "openai:gpt-5", queue: []brain.Response{
		{ToolCalls: []brain.ToolCall{{ID: "1", Name: "do_thing", Arguments: "{}"}}},
		{Text: "Resumen: tarea completada."},
	}}
	r := New(b, reg, "sys", quiet())

	out, err := r.Run(context.Background(), "haz la cosa")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if probe.called != 1 {
		t.Errorf("tool called %d times, want 1", probe.called)
	}
	if !strings.Contains(out, "completada") {
		t.Errorf("expected final summary, got %q", out)
	}
}

func TestRunner_RespectsMaxRounds(t *testing.T) {
	reg := tools.NewRegistry()
	probe := &probeTool{name: "loop", result: "again"}
	reg.Register(probe)

	// Brain that always asks for the tool, never finishes.
	q := make([]brain.Response, 50)
	for i := range q {
		q[i] = brain.Response{ToolCalls: []brain.ToolCall{{ID: "x", Name: "loop", Arguments: "{}"}}}
	}
	b := &scriptedBrain{name: "mock", queue: q}
	r := New(b, reg, "sys", quiet())
	r.MaxRounds = 5

	out, err := r.Run(context.Background(), "loop forever")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if probe.called > 5 {
		t.Errorf("tool called %d times, want <= MaxRounds (5)", probe.called)
	}
	if !strings.Contains(strings.ToLower(out), "máximo de pasos") {
		t.Errorf("expected max-rounds message, got %q", out)
	}
}

func TestRunner_HonorsContextCancel(t *testing.T) {
	reg := tools.NewRegistry()
	b := &scriptedBrain{name: "mock", queue: []brain.Response{{Text: "no debería llegar"}}}
	r := New(b, reg, "sys", quiet())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err := r.Run(ctx, "tarea")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}
