package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jcomellys/voice-mac-agent/internal/stt"
)

// --- test doubles ---

type onceActivator struct{ calls int }

func (a *onceActivator) Name() string          { return "once" }
func (a *onceActivator) SupportsBargeIn() bool { return false }
func (a *onceActivator) WaitForActivation(ctx context.Context) error {
	a.calls++
	if a.calls > 1 {
		return context.Canceled // end the loop after the first turn
	}
	return nil
}

type silentSTT struct{ calls int }

func (s *silentSTT) Name() string { return "silent" }
func (s *silentSTT) Listen(ctx context.Context) (string, error) {
	s.calls++
	return "", stt.ErrSilent
}

type recordingTTS struct{ spoke int }

func (t *recordingTTS) Name() string { return "rec-tts" }
func (t *recordingTTS) Speak(ctx context.Context, text string) error {
	t.spoke++
	return nil
}

type countingCues struct{ listening, captured int }

func (c *countingCues) Listening() { c.listening++ }
func (c *countingCues) Captured()  { c.captured++ }

type recordingHandler struct{ calls int }

func (h *recordingHandler) HandleUtterance(ctx context.Context, userText string) (string, error) {
	h.calls++
	return "no debería llamarse", nil
}

// TestPipeline_SilentTurnPlaysCapturedSkipsBrain verifies the accessibility
// contract: when STT reports silence (e.g. the listen timeout fired on an
// accidental activation), the user still hears the "mic closed" cue and the
// turn is skipped without bothering the brain or speaking a reply.
func TestPipeline_SilentTurnPlaysCapturedSkipsBrain(t *testing.T) {
	act := &onceActivator{}
	mic := &silentSTT{}
	speaker := &recordingTTS{}
	cues := &countingCues{}
	handler := &recordingHandler{}

	p := NewPipeline(mic, speaker, act).WithCues(cues)
	err := p.Start(context.Background(), handler)
	if err != nil {
		t.Fatalf("Start returned %v, want nil on cancellation", err)
	}

	if cues.listening != 1 {
		t.Errorf("listening cue played %d times, want 1", cues.listening)
	}
	if cues.captured != 1 {
		t.Errorf("captured cue played %d times, want 1 (must fire even on silence)", cues.captured)
	}
	if handler.calls != 0 {
		t.Errorf("brain handler called %d times, want 0 on a silent turn", handler.calls)
	}
	if speaker.spoke != 0 {
		t.Errorf("TTS spoke %d times, want 0 on a silent turn", speaker.spoke)
	}
	if mic.calls != 1 {
		t.Errorf("STT listened %d times, want 1", mic.calls)
	}
}

// --- barge-in test doubles ---

// bargeActivator returns immediately on the first call (start turn 1). On the
// second call (the in-turn interrupt watcher) it blocks until the test fires
// barge, then returns nil. Later calls block on ctx so the loop can be ended
// by cancelling the parent context.
type bargeActivator struct {
	mu    sync.Mutex
	calls int
	barge chan struct{}
}

func newBargeActivator() *bargeActivator { return &bargeActivator{barge: make(chan struct{})} }

func (a *bargeActivator) Name() string          { return "barge" }
func (a *bargeActivator) SupportsBargeIn() bool { return true }
func (a *bargeActivator) WaitForActivation(ctx context.Context) error {
	a.mu.Lock()
	a.calls++
	n := a.calls
	a.mu.Unlock()
	if n == 1 {
		return nil // start the first turn immediately
	}
	select {
	case <-a.barge:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// blockingTTS signals when speaking starts and blocks until ctx is cancelled,
// modelling a long utterance that barge-in must cut short.
type blockingTTS struct {
	started chan struct{}
	once    sync.Once
}

func newBlockingTTS() *blockingTTS { return &blockingTTS{started: make(chan struct{})} }

func (t *blockingTTS) Name() string { return "blocking-tts" }
func (t *blockingTTS) Speak(ctx context.Context, text string) error {
	t.once.Do(func() { close(t.started) })
	<-ctx.Done()
	return ctx.Err()
}

type textSTT struct {
	mu    sync.Mutex
	calls int
	first string
}

func (s *textSTT) Name() string { return "text-stt" }
func (s *textSTT) Listen(ctx context.Context) (string, error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	s.mu.Unlock()
	if n == 1 {
		return s.first, nil
	}
	// After the barge-driven relisten, block until the loop is cancelled.
	<-ctx.Done()
	return "", ctx.Err()
}

func (s *textSTT) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type fixedHandler struct{ reply string }

func (h *fixedHandler) HandleUtterance(ctx context.Context, _ string) (string, error) {
	return h.reply, nil
}

// TestPipeline_BargeInCutsSpeechAndRelistens verifies UX 0.4.1: while the
// agent is speaking, an activator gesture cancels the TTS within the loop and
// the pipeline returns to listening without exiting.
func TestPipeline_BargeInCutsSpeechAndRelistens(t *testing.T) {
	act := newBargeActivator()
	mic := &textSTT{first: "explícame algo largo"}
	speaker := newBlockingTTS()
	handler := &fixedHandler{reply: "una respuesta muy larga..."}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- p_start(NewPipeline(mic, speaker, act), ctx, handler) }()

	// Wait until TTS has started speaking, then barge in.
	select {
	case <-speaker.started:
	case <-time.After(2 * time.Second):
		t.Fatal("TTS never started speaking")
	}
	close(act.barge) // user presses the hotkey mid-speech

	// The pipeline should relisten (STT called a 2nd time) without exiting.
	deadline := time.After(2 * time.Second)
	for mic.callCount() < 2 {
		select {
		case <-deadline:
			t.Fatalf("pipeline did not relisten after barge-in (STT calls=%d)", mic.callCount())
		case <-time.After(10 * time.Millisecond):
		}
	}

	// End the loop.
	cancel()
	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Fatalf("Start returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline did not exit after ctx cancel")
	}
}

// p_start is a tiny indirection so the goroutine call reads clearly.
func p_start(p *Pipeline, ctx context.Context, h Handler) error {
	return p.Start(ctx, h)
}

// --- resilience test doubles ---

// alwaysActivator returns nil on every activation, so the turn loop is driven
// purely by the STT (which ends the test by returning Canceled).
type alwaysActivator struct{ calls int }

func (a *alwaysActivator) Name() string          { return "always" }
func (a *alwaysActivator) SupportsBargeIn() bool { return false }
func (a *alwaysActivator) WaitForActivation(context.Context) error {
	a.calls++
	return nil
}

// textThenCancelSTT returns a real utterance on the first listen, then cancels
// the loop on the second — modelling exactly one user turn before shutdown.
type textThenCancelSTT struct {
	calls int
	first string
}

func (s *textThenCancelSTT) Name() string { return "text-cancel-stt" }
func (s *textThenCancelSTT) Listen(ctx context.Context) (string, error) {
	s.calls++
	if s.calls == 1 {
		return s.first, nil
	}
	return "", context.Canceled
}

// erroringHandler always fails, modelling a transient brain/network error.
type erroringHandler struct {
	calls int
	err   error
}

func (h *erroringHandler) HandleUtterance(ctx context.Context, _ string) (string, error) {
	h.calls++
	return "", h.err
}

// TestPipeline_HandlerErrorStaysAlive is the core resilience contract: when the
// brain/handler returns a transient error, the agent must speak something and
// KEEP LISTENING — it must not terminate. A non-sighted user cannot restart it
// from a keyboard, so a network hiccup terminating the process is a critical
// accessibility failure.
func TestPipeline_HandlerErrorStaysAlive(t *testing.T) {
	act := &alwaysActivator{}
	mic := &textThenCancelSTT{first: "abre Mensajes"}
	speaker := &recordingTTS{}
	handler := &erroringHandler{err: fmt.Errorf("openai request: dial tcp: i/o timeout")}

	p := NewPipeline(mic, speaker, act)
	err := p.Start(context.Background(), handler)
	if err != nil {
		t.Fatalf("Start returned %v, want nil — a transient handler error must not kill the agent", err)
	}
	if handler.calls != 1 {
		t.Errorf("handler called %d times, want 1", handler.calls)
	}
	if speaker.spoke != 1 {
		t.Errorf("TTS spoke %d times, want 1 (the error must be voiced)", speaker.spoke)
	}
	// The loop must have come back around to listen again (2nd Listen is what
	// returns Canceled to end the test), proving it did not exit on the error.
	if mic.calls != 2 {
		t.Errorf("STT listened %d times, want 2 (agent must keep listening after an error)", mic.calls)
	}
}

// TestPipeline_ShutdownDuringHandlerExits verifies the opposite: when the
// parent context is cancelled (real shutdown), a handler error caused by that
// cancellation does end the loop cleanly instead of looping forever.
func TestPipeline_ShutdownDuringHandlerExits(t *testing.T) {
	act := &onceActivator{}
	mic := &textThenCancelSTT{first: "hola"}
	speaker := &recordingTTS{}
	handler := &erroringHandler{err: context.Canceled}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already shutting down

	err := p_start(NewPipeline(mic, speaker, act), ctx, handler)
	if err != nil {
		t.Fatalf("Start returned %v, want nil on cancellation", err)
	}
}

// TestSpokenError_ClassifiesByKind checks the user hears a message tailored to
// the failure (connection vs saturation vs auth vs permission), not one opaque
// sentence — important when the user must decide whether to retry or get help.
func TestSpokenError_ClassifiesByKind(t *testing.T) {
	cases := []struct {
		err  string
		want string
	}{
		{"openai request: dial tcp: i/o timeout", "conexión"},
		{"anthropic: overloaded_error (529)", "saturado"},
		{"openai: rate limit reached", "saturado"},
		{"openai: invalid_api_key (401 unauthorized)", "clave"},
		{"command not on allowlist: \"rm -rf /\"", "permiso"},
		{"something totally unexpected happened", "sigo aquí"},
	}
	for _, c := range cases {
		got := spokenError(errors.New(c.err))
		if !strings.Contains(got, c.want) {
			t.Errorf("spokenError(%q) = %q, want it to mention %q", c.err, got, c.want)
		}
	}
}
