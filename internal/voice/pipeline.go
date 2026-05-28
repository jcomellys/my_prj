package voice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

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
	Log       *slog.Logger
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

// WithLogger attaches a structured logger (used for barge-in events).
func (p *Pipeline) WithLogger(l *slog.Logger) *Pipeline {
	p.Log = l
	return p
}

func (p *Pipeline) log() *slog.Logger {
	if p.Log != nil {
		return p.Log
	}
	return slog.Default()
}

func (p *Pipeline) Name() string {
	return fmt.Sprintf("pipeline(stt=%s,tts=%s,act=%s)", p.STT.Name(), p.TTS.Name(), p.Activator.Name())
}

func (p *Pipeline) Start(ctx context.Context, h Handler) error {
	// skipActivation is set after a barge-in: the interrupting gesture IS the
	// activation for the next turn, so we go straight to listening.
	skipActivation := false

	for {
		if err := ctx.Err(); err != nil {
			// A cancelled parent context is a clean shutdown, not a failure.
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		if !skipActivation {
			// Wait for the user to activate the agent (hotkey, wake word, etc).
			if err := p.Activator.WaitForActivation(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return fmt.Errorf("activator: %w", err)
			}
		}
		skipActivation = false

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
				// Make the skip visible so the operator/user doesn't think
				// the agent hung; the Captured cue already played above.
				fmt.Println("(no se detectó voz; turno omitido)")
				continue
			}
			return fmt.Errorf("stt: %w", err)
		}
		if text == "" {
			continue
		}

		// Surface the transcribed utterance so the user (and any reviewing
		// AI) can verify what the STT actually heard.
		fmt.Printf("you> %s\n", text)

		barged, err := p.processTurn(ctx, h, text)
		if err != nil {
			// Only a cancelled context (shutdown / barge ending the program)
			// stops the loop. Recoverable turn errors are handled inside
			// processTurn (spoken + swallowed) so the agent stays alive — a
			// non-sighted user cannot restart it from a keyboard.
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		if barged {
			// The interrupting gesture starts the next listen turn directly.
			skipActivation = true
		}
	}
}

// processTurn runs the brain and speaks the reply. When the activator
// supports barge-in, a single watcher covers both the thinking and the
// speaking phases: pressing the activator cancels the in-flight brain call
// and/or TTS within ~1s (only our own processes, via context cancellation —
// no killall) and returns barged=true so the caller goes straight back to
// listening.
func (p *Pipeline) processTurn(ctx context.Context, h Handler, text string) (barged bool, err error) {
	if !p.Activator.SupportsBargeIn() {
		return false, p.handleAndSpeak(ctx, h, text)
	}

	procCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	bargeCh := make(chan struct{}, 1)
	go func() {
		// One activation gesture during processing = barge-in.
		if werr := p.Activator.WaitForActivation(procCtx); werr == nil {
			select {
			case bargeCh <- struct{}{}:
			default:
			}
			cancel() // interrupt the brain HTTP call and/or the TTS process
		}
	}()

	didBarge := func() bool {
		select {
		case <-bargeCh:
			return true
		default:
			return false
		}
	}

	reply, herr := h.HandleUtterance(procCtx, text)
	if didBarge() {
		p.log().Info("voice.barge_in", "phase", "thinking")
		return true, nil
	}
	if herr != nil {
		// Shutdown (parent ctx cancelled) is fatal; everything else is a
		// recoverable turn error: speak a helpful message and keep the loop
		// alive so the user can simply try again.
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		p.log().Warn("turn.error", "err", herr)
		_ = p.TTS.Speak(ctx, spokenError(herr))
		return false, nil
	}

	if reply != "" {
		serr := p.TTS.Speak(procCtx, reply)
		if didBarge() {
			p.log().Info("voice.barge_in", "phase", "speaking")
			return true, nil
		}
		if serr != nil {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			// A TTS hiccup must not kill the agent either; log and continue.
			p.log().Warn("tts.error", "err", serr)
			return false, nil
		}
	}
	return false, nil
}

// handleAndSpeak is the simple, non-interruptible path for activators that
// don't support barge-in (e.g. always-on / stdin dev mode).
func (p *Pipeline) handleAndSpeak(ctx context.Context, h Handler, text string) error {
	reply, err := h.HandleUtterance(ctx, text)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		p.log().Warn("turn.error", "err", err)
		_ = p.TTS.Speak(ctx, spokenError(err))
		return nil
	}
	if reply != "" {
		if err := p.TTS.Speak(ctx, reply); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			p.log().Warn("tts.error", "err", err)
			return nil
		}
	}
	return nil
}

// spokenError maps a recoverable turn error to a short Spanish sentence the
// user hears. The agent stays alive after speaking it: a transient brain or
// network failure must never terminate an agent that a non-sighted user
// cannot restart from a keyboard.
func spokenError(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "rate limit") || strings.Contains(msg, "429") ||
		strings.Contains(msg, "overloaded") || strings.Contains(msg, "503"):
		return "El servicio está saturado en este momento. Espera unos segundos y vuelve a intentarlo."
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") ||
		strings.Contains(msg, "connection") || strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "dial ") || strings.Contains(msg, "eof") ||
		strings.Contains(msg, "network") || strings.Contains(msg, "tls"):
		return "Tuve un problema de conexión. Inténtalo de nuevo en un momento."
	case strings.Contains(msg, "api key") || strings.Contains(msg, "apikey") ||
		strings.Contains(msg, "api_key") || strings.Contains(msg, "401") ||
		strings.Contains(msg, "unauthorized") || strings.Contains(msg, "invalid_api"):
		return "Hay un problema con la clave de acceso al modelo. Avisa a quien configuró el agente."
	case strings.Contains(msg, "not on allowlist") || strings.Contains(msg, "permission") ||
		strings.Contains(msg, "refusing") || strings.Contains(msg, "not allowed"):
		return "No tengo permiso para hacer eso."
	default:
		return "Hubo un problema procesando tu solicitud, pero sigo aquí. Inténtalo otra vez."
	}
}
