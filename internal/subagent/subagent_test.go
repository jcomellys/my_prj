package subagent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestRunner_WriteFileEndToEndWithMockBrain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes", "transistor.txt")
	reg := tools.NewRegistry()
	reg.Register(tools.NewWriteFile())

	b := &scriptedBrain{name: "mock", queue: []brain.Response{
		{ToolCalls: []brain.ToolCall{{
			ID:        "write-1",
			Name:      "write_file",
			Arguments: `{"path":"` + path + `","content":"uno\ndos\ntres"}`,
		}}},
		{Text: "Archivo creado con tres puntos."},
	}}
	r := New(b, reg, "sys", quiet())

	out, err := r.Run(context.Background(), "crea un archivo de prueba")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "Archivo creado") {
		t.Fatalf("expected final summary, got %q", out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected delegated write_file output: %v", err)
	}
	if string(got) != "uno\ndos\ntres" {
		t.Fatalf("file content = %q", string(got))
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
	if err == nil {
		t.Fatalf("expected MaxRounds error, got output %q", out)
	}
	if probe.called > 5 {
		t.Errorf("tool called %d times, want <= MaxRounds (5)", probe.called)
	}
	if !strings.Contains(err.Error(), "MaxRounds=5") {
		t.Errorf("expected clear MaxRounds error, got %v", err)
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

type blockingBrain struct{}

func (blockingBrain) Name() string { return "blocking" }
func (blockingBrain) Chat(ctx context.Context, _ []brain.Message, _ []brain.ToolSpec) (*brain.Response, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRunner_ContextCancelDuringBrainDoesNotHang(t *testing.T) {
	reg := tools.NewRegistry()
	r := New(blockingBrain{}, reg, "sys", quiet())

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := r.Run(ctx, "tarea larga")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Run returned too slowly after cancellation: %s", elapsed)
	}
}

func TestRunner_EmitsProgressEventsInOrder(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&probeTool{name: "do_thing", result: "done"})

	b := &scriptedBrain{name: "openai:gpt-5", queue: []brain.Response{
		{ToolCalls: []brain.ToolCall{{ID: "1", Name: "do_thing", Arguments: "{}"}}},
		{Text: "Resumen final."},
	}}
	r := New(b, reg, "sys", quiet())

	ch := make(chan ProgressEvent, 32)
	if _, err := r.RunWithProgress(context.Background(), "haz la cosa", ch); err != nil {
		t.Fatalf("RunWithProgress: %v", err)
	}
	close(ch)
	var kinds []ProgressKind
	var toolName string
	for ev := range ch {
		kinds = append(kinds, ev.Kind)
		if ev.Kind == ProgressToolOK {
			toolName = ev.Tool
		}
	}
	want := []ProgressKind{ProgressStarted, ProgressRound, ProgressToolOK, ProgressRound, ProgressDone}
	if len(kinds) != len(want) {
		t.Fatalf("got %d events %v, want %v", len(kinds), kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("event %d = %s, want %s", i, kinds[i], want[i])
		}
	}
	if toolName != "do_thing" {
		t.Errorf("tool_ok carried tool %q, want do_thing", toolName)
	}
}

func TestRunner_FullProgressChannelNeverBlocksTask(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&probeTool{name: "do_thing", result: "done"})

	b := &scriptedBrain{name: "openai:gpt-5", queue: []brain.Response{
		{ToolCalls: []brain.ToolCall{{ID: "1", Name: "do_thing", Arguments: "{}"}}},
		{Text: "Resumen."},
	}}
	r := New(b, reg, "sys", quiet())

	ch := make(chan ProgressEvent) // unbuffered, NO consumer: every send must drop
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := r.RunWithProgress(context.Background(), "haz la cosa", ch); err != nil {
			t.Errorf("RunWithProgress: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runner blocked on a progress send — emit must be non-blocking")
	}
}

func TestRunner_BudgetCheckStopsTask(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&probeTool{name: "do_thing", result: "done"})

	b := &scriptedBrain{name: "openai:gpt-5", queue: []brain.Response{
		{ToolCalls: []brain.ToolCall{{ID: "1", Name: "do_thing", Arguments: "{}"}}},
		{Text: "no debería llegar aquí"},
	}}
	r := New(b, reg, "sys", quiet())
	calls := 0
	r.BudgetCheck = func() error {
		calls++
		if calls > 1 { // first round allowed, then budget exhausted
			return fmt.Errorf("se alcanzó el presupuesto mensual")
		}
		return nil
	}

	_, err := r.Run(context.Background(), "tarea cara")
	if err == nil || !strings.Contains(err.Error(), "presupuesto") {
		t.Fatalf("expected budget-stop error, got %v", err)
	}
	if b.calls != 1 {
		t.Errorf("brain must not be called after the budget stop, got %d calls", b.calls)
	}
}

func TestCompactTaskHistory_ShrinksOldKeepsRecent(t *testing.T) {
	big := strings.Repeat("contenido de PDF ", 3000) // ~50 KB
	h := []brain.Message{
		{Role: brain.RoleSystem, Content: "sys"},
		{Role: brain.RoleUser, Content: "la tarea"},
		{Role: brain.RoleAssistant, ToolCalls: []brain.ToolCall{{ID: "a", Name: "read_pdf"}}},
		{Role: brain.RoleTool, ToolCallID: "a", Name: "read_pdf", Content: big},
	}
	// Pad with enough recent messages to push the big result out of the tail.
	for i := 0; i < taskKeepRecentMsgs; i++ {
		h = append(h, brain.Message{Role: brain.RoleAssistant, Content: "paso"})
	}
	recent := strings.Repeat("x", 2000)
	h[len(h)-1] = brain.Message{Role: brain.RoleTool, ToolCallID: "z", Name: "read_file", Content: recent}

	before, after := compactTaskHistory(h)
	if after >= before {
		t.Fatalf("expected compaction: before=%d after=%d", before, after)
	}
	if len(h[3].Content) > taskToolResultKeep+100 {
		t.Errorf("old tool result not shrunk: %d bytes", len(h[3].Content))
	}
	if !strings.Contains(h[3].Content, "recortado") {
		t.Error("shrunk result must carry the elision marker")
	}
	if len(h[len(h)-1].Content) != len(recent) {
		t.Error("recent tail must remain untouched")
	}
	if h[0].Content != "sys" || h[1].Content != "la tarea" {
		t.Error("system prompt and task must never change")
	}
}

func TestCompactTaskHistory_UnderLimitUntouched(t *testing.T) {
	h := []brain.Message{
		{Role: brain.RoleSystem, Content: "sys"},
		{Role: brain.RoleUser, Content: "tarea"},
		{Role: brain.RoleTool, ToolCallID: "a", Content: "corto"},
	}
	before, after := compactTaskHistory(h)
	if before != after || h[2].Content != "corto" {
		t.Errorf("history under the limit must not change (before=%d after=%d)", before, after)
	}
}
