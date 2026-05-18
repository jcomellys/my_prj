// Package tts defines the text-to-speech provider interface.
package tts

import "context"

// TTS speaks the given text. Implementations are local (macOS say, Piper) or
// cloud (ElevenLabs, OpenAI). Speak should block until audio output starts;
// it may return before audio finishes if the implementation streams.
type TTS interface {
	Name() string

	// Speak speaks the text. Returns when output has started.
	// Cancelling ctx must stop audio output promptly.
	Speak(ctx context.Context, text string) error
}
