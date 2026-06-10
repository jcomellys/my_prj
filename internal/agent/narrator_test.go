package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jcomellys/voice-mac-agent/internal/subagent"
)

// recordingSpeaker captures spoken phrases, thread-safe (the narrator speaks
// from its own goroutine).
type recordingSpeaker struct {
	mu      sync.Mutex
	phrases []string
}

func (r *recordingSpeaker) Speak(_ context.Context, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phrases = append(r.phrases, text)
	return nil
}

func (r *recordingSpeaker) spoken() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.phrases...)
}

// fakeRunner scripts a delegation: emits the given events spaced by step,
// then returns. Lets tests exercise the narrator without a brain.
type fakeRunner struct {
	events []subagent.ProgressEvent
	step   time.Duration
	result string
	err    error
}

func (f *fakeRunner) RunWithProgress(ctx context.Context, _ string, ch chan<- subagent.ProgressEvent) (string, error) {
	for _, ev := range f.events {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(f.step):
		}
		if ch != nil {
			select {
			case ch <- ev:
			default:
			}
		}
	}
	return f.result, f.err
}

func TestNarratedDelegate_LongTaskSpeaksProgress(t *testing.T) {
	spk := &recordingSpeaker{}
	events := []subagent.ProgressEvent{
		{Kind: subagent.ProgressStarted},
		{Kind: subagent.ProgressToolOK, Tool: "write_file"},
		{Kind: subagent.ProgressDone},
	}
	n := &NarratedDelegate{
		Runner: &fakeRunner{events: events, step: 30 * time.Millisecond, result: "listo"},
		Voice:  spk,
		MinGap: 10 * time.Millisecond, // tiny gap so the test is fast
	}

	out, err := n.Run(context.Background(), "tarea larga")
	if err != nil || out != "listo" {
		t.Fatalf("Run: out=%q err=%v", out, err)
	}
	got := spk.spoken()
	if len(got) == 0 {
		t.Fatal("a long task must produce at least one intermediate narration")
	}
	found := false
	for _, p := range got {
		if p == "Sigo trabajando; ya escribí un archivo." {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the write_file phrase, spoke %q", got)
	}
}

func TestNarratedDelegate_ShortTaskStaysQuiet(t *testing.T) {
	spk := &recordingSpeaker{}
	n := &NarratedDelegate{
		Runner: &fakeRunner{
			events: []subagent.ProgressEvent{{Kind: subagent.ProgressStarted}, {Kind: subagent.ProgressDone}},
			step:   time.Millisecond,
			result: "ya",
		},
		Voice:  spk,
		MinGap: time.Hour, // a short task never reaches the first gap
	}
	if _, err := n.Run(context.Background(), "tarea corta"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := spk.spoken(); len(got) != 0 {
		t.Errorf("short task must not narrate, spoke %q", got)
	}
}

func TestNarratedDelegate_RateLimitsCoalesces(t *testing.T) {
	spk := &recordingSpeaker{}
	// 20 rapid tool events; with a generous gap only the first should speak.
	events := make([]subagent.ProgressEvent, 20)
	for i := range events {
		events[i] = subagent.ProgressEvent{Kind: subagent.ProgressToolOK, Tool: "write_file"}
	}
	n := &NarratedDelegate{
		Runner: &fakeRunner{events: events, step: time.Millisecond, result: "ok"},
		Voice:  spk,
		MinGap: 5 * time.Millisecond,
	}
	if _, err := n.Run(context.Background(), "ráfaga"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := spk.spoken()
	if len(got) == 0 || len(got) >= 20 {
		t.Errorf("rate limit must coalesce the burst: spoke %d phrases", len(got))
	}
}

func TestNarratedDelegate_HeartbeatDuringSilentRound(t *testing.T) {
	spk := &recordingSpeaker{}
	// No events at all for a while — simulates one long brain round. The
	// ticker fallback must still produce the generic heartbeat.
	n := &NarratedDelegate{
		Runner: &fakeRunner{events: nil, step: 0, result: "fin"},
		Voice:  spk,
		MinGap: 15 * time.Millisecond,
	}
	slow := &slowRunner{inner: n.Runner.(*fakeRunner), delay: 80 * time.Millisecond}
	n.Runner = slow
	if _, err := n.Run(context.Background(), "ronda larga"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, p := range spk.spoken() {
		if p == "Sigo trabajando en tu tarea." {
			return
		}
	}
	t.Errorf("expected the heartbeat phrase during a silent stretch, spoke %q", spk.spoken())
}

// slowRunner delays before delegating to the inner runner — a silent stretch
// with no progress events.
type slowRunner struct {
	inner *fakeRunner
	delay time.Duration
}

func (s *slowRunner) RunWithProgress(ctx context.Context, task string, ch chan<- subagent.ProgressEvent) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(s.delay):
	}
	return s.inner.RunWithProgress(ctx, task, ch)
}

func TestNarratedDelegate_CancelStopsNarrationAndTask(t *testing.T) {
	spk := &recordingSpeaker{}
	n := &NarratedDelegate{
		Runner: &slowRunner{inner: &fakeRunner{result: "nunca"}, delay: 10 * time.Second},
		Voice:  spk,
		MinGap: time.Hour,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := n.Run(ctx, "tarea cancelada")
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel — narration goroutine leak?")
	}
}

func TestNarratedDelegate_NilVoicePassesThrough(t *testing.T) {
	n := &NarratedDelegate{
		Runner: &fakeRunner{result: "directo"},
	}
	out, err := n.Run(context.Background(), "sin voz")
	if err != nil || out != "directo" {
		t.Errorf("nil Voice must pass through: out=%q err=%v", out, err)
	}
}
