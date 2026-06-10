// Package agent — narrator.go: voice progress for delegated tasks (fase 1
// incremento 2; design: docs/design/fase1-incr2-mid-task-narration.md).
//
// delegate_task blocks the frontal while the sub-agent works, which used to
// mean 30-60s of silence — for a non-sighted user, indistinguishable from a
// crash. NarratedDelegate wraps the sub-agent runner: a goroutine consumes
// the runner's progress events and speaks short FIXED Spanish phrases,
// rate-limited, costing zero extra tokens. Barge-in keeps working because
// every Speak uses the turn's ctx — the hotkey cancels task and narration
// together.
package agent

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jcomellys/voice-mac-agent/internal/subagent"
)

// Speaker is the TTS surface the narrator needs. voice.Pipeline implements it.
type Speaker interface {
	Speak(ctx context.Context, text string) error
}

// progressRunner is what NarratedDelegate needs from subagent.Runner;
// an interface so tests can fake long tasks without a brain.
type progressRunner interface {
	RunWithProgress(ctx context.Context, task string, progress chan<- subagent.ProgressEvent) (string, error)
}

// NarratedDelegate implements tools.TaskDelegate. With a nil Voice it is a
// transparent pass-through to the runner.
type NarratedDelegate struct {
	Runner progressRunner
	Voice  Speaker
	Log    *slog.Logger

	// MinGap is the minimum silence between narrated phrases (and before the
	// first one), so short tasks stay quiet and long ones aren't chatty.
	// Default 9s — inside the 8-12s band the design doc set.
	MinGap time.Duration
}

const defaultNarrationGap = 9 * time.Second

func (n *NarratedDelegate) Run(ctx context.Context, task string) (string, error) {
	if n.Voice == nil {
		return n.Runner.RunWithProgress(ctx, task, nil)
	}

	ch := make(chan subagent.ProgressEvent, 16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		n.narrateLoop(ctx, ch)
	}()

	out, err := n.Runner.RunWithProgress(ctx, task, ch)
	close(ch) // runner finished; never closes the channel itself
	<-done    // narrator drained and exited — no goroutine leaks
	return out, err
}

// narrateLoop speaks at most one phrase per MinGap. Two sources wake it:
// progress events (which pick a specific phrase) and a ticker fallback (a
// brain round can take 60s with no events — exactly the silence this feature
// kills). It exits when the channel closes or the ctx dies.
func (n *NarratedDelegate) narrateLoop(ctx context.Context, ch <-chan subagent.ProgressEvent) {
	gap := n.MinGap
	if gap <= 0 {
		gap = defaultNarrationGap
	}
	last := time.Now() // start the clock at delegation: quiet until first gap
	ticker := time.NewTicker(gap)
	defer ticker.Stop()

	speak := func(phrase string, minSilence time.Duration) {
		if phrase == "" || time.Since(last) < minSilence {
			return
		}
		last = time.Now()
		if err := n.Voice.Speak(ctx, phrase); err != nil && n.Log != nil {
			n.Log.Debug("narrator.speak_failed", "err", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			speak(progressPhrase(ev), gap)
		case <-ticker.C:
			// Fallback for a long silent brain round: only when nothing has
			// been narrated for 2×gap, so event phrases keep priority.
			speak("Sigo trabajando en tu tarea.", 2*gap)
		}
	}
}

// progressPhrase maps an event to a fixed phrase, or "" to stay quiet.
// started/done are silent: the frontal already speaks its own preamble and
// final summary, and double-talk is worse than silence.
func progressPhrase(ev subagent.ProgressEvent) string {
	switch ev.Kind {
	case subagent.ProgressToolOK:
		switch {
		case ev.Tool == "write_file":
			return "Sigo trabajando; ya escribí un archivo."
		case strings.HasPrefix(ev.Tool, "read_"):
			return "Sigo trabajando; estoy leyendo información."
		default:
			return "Sigo trabajando en tu tarea."
		}
	case subagent.ProgressToolErr:
		return "Encontré un tropiezo; lo estoy resolviendo."
	default:
		return ""
	}
}
