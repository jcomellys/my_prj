// Package audiocue plays short non-verbal sounds ("earcons") that tell the
// user — especially a blind user who cannot see the terminal — which state
// the agent is in: when the microphone opens (speak now) and when it closes
// (heard you, processing).
//
// This is an accessibility-critical feature, so cues default to ON. On
// macOS it shells out to `afplay` with system sounds. On any platform
// where `afplay` is absent, the player silently no-ops, so the rest of
// the agent runs unchanged.
package audiocue

import (
	"os/exec"
)

// Player plays state-transition earcons.
type Player struct {
	enabled   bool
	bin       string
	listening string // played when the mic opens
	captured  string // played when the mic closes / speech captured
}

// Defaults are macOS system sounds. Tink is bright/ascending (inviting the
// user to speak); Pop is lower/definitive (speech captured, mic closed).
const (
	DefaultListeningSound = "/System/Library/Sounds/Tink.aiff"
	DefaultCapturedSound  = "/System/Library/Sounds/Pop.aiff"
)

// New builds a Player. If enabled is true but no audio backend (afplay) is
// found, the player disables itself silently — a missing speaker tool must
// never break the conversation loop. Empty sound paths fall back to defaults.
func New(enabled bool, listening, captured string) *Player {
	p := &Player{
		enabled:   enabled,
		bin:       "afplay",
		listening: orDefault(listening, DefaultListeningSound),
		captured:  orDefault(captured, DefaultCapturedSound),
	}
	if p.enabled {
		if _, err := exec.LookPath(p.bin); err != nil {
			p.enabled = false
		}
	}
	return p
}

// Listening plays the "mic open, speak now" cue. Blocks until the sound
// finishes so it never bleeds into the recording that follows.
func (p *Player) Listening() { p.play(p.listening) }

// Captured plays the "mic closed, processing" cue.
func (p *Player) Captured() { p.play(p.captured) }

// Enabled reports whether cues will actually play.
func (p *Player) Enabled() bool { return p.enabled }

func (p *Player) play(sound string) {
	if !p.enabled || sound == "" {
		return
	}
	// Blocking on purpose: the listening cue must finish before sox starts
	// recording, otherwise the mic captures the tail of the tone. Errors
	// are ignored — a failed cue must not interrupt the conversation.
	_ = exec.Command(p.bin, sound).Run()
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
