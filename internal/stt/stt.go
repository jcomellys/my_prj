// Package stt defines the speech-to-text provider interface.
package stt

import "context"

// STT transcribes audio to text. Implementations may be local (whisper.cpp,
// macOS Speech framework) or cloud (OpenAI Whisper API, Deepgram).
type STT interface {
	Name() string

	// Listen blocks until the user finishes speaking and returns the transcript.
	// The activator decides when to start a Listen; STT decides when it ends
	// (silence detection, max duration, or explicit stop signal via ctx).
	Listen(ctx context.Context) (string, error)
}
