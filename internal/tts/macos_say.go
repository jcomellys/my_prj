package tts

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// MacOSSay speaks with the macOS `say` command. Free, on-device.
//
// It synthesizes to a temporary AIFF and then plays the finished file with
// `afplay`, rather than speaking straight to the audio device. Speaking live
// to the device clips the last word when the device tears down a few ms
// early — the "se corta al final" symptom. Playing a complete file cannot
// clip the tail. Both steps honor ctx, so a barge-in still cuts speech < 1s
// (afplay is killed) and a barge-in during synthesis skips playback.
//
// Voice quality: the bundled compact voices sound robotic. Install an
// Enhanced/Premium Spanish voice (System Settings → Accessibility → Spoken
// Content → System Voice → Manage Voices) and name it in config, e.g.
// "Mónica" or "Paulina". If the named voice is missing, we fall back to the
// system default rather than going silent.
type MacOSSay struct {
	Voice string // empty = system default
	Rate  int    // words per minute; 0 = system default

	// run executes a command honoring ctx; overridable in tests.
	run func(ctx context.Context, name string, args ...string) error
}

func NewMacOSSay(voice string, rate int) *MacOSSay {
	return &MacOSSay{Voice: voice, Rate: rate, run: execRun}
}

func (m *MacOSSay) Name() string { return "macos_say" }

func execRun(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func (m *MacOSSay) Speak(ctx context.Context, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runner := m.run
	if runner == nil {
		runner = execRun
	}

	f, err := os.CreateTemp("", "vma-tts-*.aiff")
	if err != nil {
		// Can't stage a file — fall back to speaking directly. Still better
		// than failing the turn; the user hears the reply (possibly clipped).
		return m.sayDirect(ctx, runner, text)
	}
	tmp := f.Name()
	_ = f.Close()
	defer os.Remove(tmp)

	// Synthesize to the file.
	if err := runner(ctx, "say", m.synthArgs(tmp, text)...); err != nil {
		if ctx.Err() != nil {
			return ctx.Err() // barge-in during synthesis
		}
		// A bad/missing voice is the likely cause: retry once with the system
		// default so the user still hears the reply.
		if m.Voice != "" {
			m2 := &MacOSSay{Rate: m.Rate, run: runner}
			if err2 := runner(ctx, "say", m2.synthArgs(tmp, text)...); err2 == nil {
				goto play
			}
		}
		return fmt.Errorf("say synth: %w", err)
	}
play:
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Play the complete file — no end clipping.
	if err := runner(ctx, "afplay", tmp); err != nil {
		if ctx.Err() != nil {
			return ctx.Err() // barge-in during playback
		}
		return fmt.Errorf("afplay: %w", err)
	}
	return nil
}

// synthArgs builds the `say` arguments to write speech to outPath.
func (m *MacOSSay) synthArgs(outPath, text string) []string {
	args := []string{}
	if m.Voice != "" {
		args = append(args, "-v", m.Voice)
	}
	if m.Rate > 0 {
		args = append(args, "-r", strconv.Itoa(m.Rate))
	}
	return append(args, "-o", outPath, text)
}

// sayDirect is the fallback path when a temp file can't be created: speak
// straight to the device (may clip the final word, but never goes silent).
func (m *MacOSSay) sayDirect(ctx context.Context, runner func(context.Context, string, ...string) error, text string) error {
	args := []string{}
	if m.Voice != "" {
		args = append(args, "-v", m.Voice)
	}
	if m.Rate > 0 {
		args = append(args, "-r", strconv.Itoa(m.Rate))
	}
	args = append(args, text)
	if err := runner(ctx, "say", args...); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("say: %w", err)
	}
	return nil
}
