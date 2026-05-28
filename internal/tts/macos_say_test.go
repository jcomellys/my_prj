package tts

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// recordingRunner captures each command invocation and can be told to fail
// specific calls, so we can test the synth→play flow without real audio.
type recordingRunner struct {
	calls   [][]string // each entry: [name, arg, arg, ...]
	failOn  int        // 1-based call index to fail; 0 = never
	failErr error
}

func (r *recordingRunner) run(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.failOn == len(r.calls) {
		if r.failErr != nil {
			return r.failErr
		}
		return errors.New("forced failure")
	}
	return nil
}

func newSay(voice string, rate int, r *recordingRunner) *MacOSSay {
	return &MacOSSay{Voice: voice, Rate: rate, run: r.run}
}

func TestSpeak_SynthThenPlaysCompleteFile(t *testing.T) {
	r := &recordingRunner{}
	s := newSay("Mónica", 180, r)

	if err := s.Speak(context.Background(), "Hola, ¿cómo estás?"); err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if len(r.calls) != 2 {
		t.Fatalf("expected 2 commands (say -o, afplay), got %d: %v", len(r.calls), r.calls)
	}

	say, play := r.calls[0], r.calls[1]
	if say[0] != "say" {
		t.Errorf("first command should be say, got %q", say[0])
	}
	if !contains(say, "-o") {
		t.Errorf("say must synthesize to a file (-o), got %v", say)
	}
	if !contains(say, "-v") || !contains(say, "Mónica") {
		t.Errorf("say must include the configured voice, got %v", say)
	}
	if !contains(say, "-r") || !contains(say, "180") {
		t.Errorf("say must include the rate, got %v", say)
	}
	if play[0] != "afplay" {
		t.Errorf("second command should be afplay, got %q", play[0])
	}
	// afplay must play the same file say wrote.
	outPath := say[len(say)-2] // -o <path> <text>: path is second-to-last
	if play[1] != outPath {
		t.Errorf("afplay should play the synthesized file %q, got %q", outPath, play[1])
	}
}

func TestSpeak_EmptyTextDoesNothing(t *testing.T) {
	r := &recordingRunner{}
	s := newSay("", 0, r)
	if err := s.Speak(context.Background(), "   "); err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if len(r.calls) != 0 {
		t.Errorf("empty text must not invoke any command, got %v", r.calls)
	}
}

func TestSpeak_BargeInDuringSynthSkipsPlayback(t *testing.T) {
	r := &recordingRunner{failOn: 1, failErr: context.Canceled}
	s := newSay("Mónica", 0, r)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled: synth "fails" with ctx error

	err := s.Speak(ctx, "una respuesta larga")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Only the synth call should have run; afplay must be skipped.
	if len(r.calls) != 1 {
		t.Errorf("expected only the synth call, got %v", r.calls)
	}
}

func TestSpeak_BadVoiceFallsBackToDefault(t *testing.T) {
	// First say (with voice) fails; retry without voice succeeds; then afplay.
	r := &recordingRunner{failOn: 1, failErr: errors.New("Voice not found")}
	s := newSay("NoSuchVoice", 0, r)

	if err := s.Speak(context.Background(), "hola"); err != nil {
		t.Fatalf("Speak should recover from a bad voice, got %v", err)
	}
	if len(r.calls) != 3 {
		t.Fatalf("expected say(voice)→say(default)→afplay, got %d: %v", len(r.calls), r.calls)
	}
	if contains(r.calls[1], "-v") {
		t.Errorf("the retry must drop the voice flag, got %v", r.calls[1])
	}
	if r.calls[2][0] != "afplay" {
		t.Errorf("expected afplay after fallback synth, got %v", r.calls[2])
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want || strings.Contains(s, want) {
			return true
		}
	}
	return false
}
