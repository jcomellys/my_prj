package voice

import (
	"context"
	"testing"

	"github.com/jcomellys/voice-mac-agent/internal/stt"
)

// --- test doubles ---

type onceActivator struct{ calls int }

func (a *onceActivator) Name() string { return "once" }
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
