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

// Pipeline is the cheap, modular voice provider: STT -> Brain -> TTS.
// Each component is swappable; the orchestrator does not know or care.
type Pipeline struct {
	STT       stt.STT
	TTS       tts.TTS
	Activator activator.Activator
}

func NewPipeline(s stt.STT, t tts.TTS, a activator.Activator) *Pipeline {
	return &Pipeline{STT: s, TTS: t, Activator: a}
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

		// Listen for one user utterance.
		text, err := p.STT.Listen(ctx)
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
