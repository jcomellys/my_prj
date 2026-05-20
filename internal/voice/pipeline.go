package voice

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jcomellys/voice-mac-agent/internal/activator"
	"github.com/jcomellys/voice-mac-agent/internal/stt"
	"github.com/jcomellys/voice-mac-agent/internal/tts"
)

// Cues plays audible state-transition earcons. Defined as an interface here
// so the voice package stays decoupled from any concrete audio backend.
type Cues interface {
	Listening() // mic is open, user should speak now
	Captured()  // mic closed, speech captured, processing
}

type noopCues struct{}

func (noopCues) Listening() {}
func (noopCues) Captured()  {}

// Pipeline is the cheap, modular voice provider: STT -> Brain -> TTS.
// Each component is swappable; the orchestrator does not know or care.
type Pipeline struct {
	STT       stt.STT
	TTS       tts.TTS
	Activator activator.Activator
	Cues      Cues
}

func NewPipeline(s stt.STT, t tts.TTS, a activator.Activator) *Pipeline {
	return &Pipeline{STT: s, TTS: t, Activator: a, Cues: noopCues{}}
}

// WithCues attaches an earcon player. Passing nil keeps the no-op default.
func (p *Pipeline) WithCues(c Cues) *Pipeline {
	if c != nil {
		p.Cues = c
	}
	return p
}

func (p *Pipeline) Name() string {
	return fmt.Sprintf("pipeline(stt=%s,tts=%s,act=%s)", p.STT.Name(), p.TTS.Name(), p.Activator.Name())
}

func (p *Pipeline) Start(ctx context.Context, h Handler) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Wait for the user to activate the agent (hotkey, wake word, etc).
		if err := p.Activator.WaitForActivation(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("activator: %w", err)
		}

		// Earcon: mic is opening. Blocks until the tone finishes so it does
		// not bleed into the recording. Tells a non-sighted user "speak now".
		p.Cues.Listening()

		// Listen for one user utterance.
		text, err := p.STT.Listen(ctx)

		// Earcon: mic closed regardless of outcome — the user must know the
		// listening window ended, even if nothing was captured.
		p.Cues.Captured()

		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			if stt.IsSilent(err) {
				// User activated but didn't speak (or only background noise).
				// Skip this turn without bothering the brain.
				continue
			}
			return fmt.Errorf("stt: %w", err)
		}
		if text == "" {
			continue
		}

		// Surface the transcribed utterance so the user (and any reviewing
		// AI) can verify what the STT actually heard. Without this the only
		// signal is whether the brain did the right thing, which makes
		// debugging "the agent ignored me" cases nearly impossible.
		fmt.Printf("you> %s\n", text)

		// Run the brain + tool rounds.
		reply, err := h.HandleUtterance(ctx, text)
		if err != nil {
			// Speak the error so the user knows something went wrong without
			// having to look at a screen.
			_ = p.TTS.Speak(ctx, "Hubo un problema procesando tu solicitud.")
			return fmt.Errorf("handler: %w", err)
		}

		if reply != "" {
			if err := p.TTS.Speak(ctx, reply); err != nil {
				return fmt.Errorf("tts: %w", err)
			}
		}
	}
}
