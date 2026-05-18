// Package voice defines the high-level voice-I/O provider.
//
// Two modes are anticipated:
//
//   - Pipeline: separate STT + Brain + TTS components, each swappable. Cheap,
//     supports fully-local operation. Default for all tiers initially.
//   - Realtime: a single bidirectional connection (e.g., OpenAI Realtime API).
//     Lower latency, higher cost. Added in fase 1 as another VoiceProvider.
//
// The orchestrator only knows the VoiceProvider interface — it does not know
// which mode is active. Swapping providers is a config change.
package voice

import "context"

// Provider is the voice I/O abstraction over a full session loop.
//
// Start runs the loop until ctx is cancelled or a fatal error occurs.
// The provider drives the conversation: it captures user audio (via its STT),
// hands transcripts to the Brain via the supplied Handler, speaks the reply
// (via its TTS), and repeats. Tool calls are executed by the Handler; the
// provider just shuttles messages.
type Provider interface {
	Name() string
	Start(ctx context.Context, h Handler) error
}

// Handler is implemented by the orchestrator. It turns a user utterance into
// an assistant utterance, executing tools as needed.
type Handler interface {
	// HandleUtterance receives the user's transcribed turn and returns the
	// assistant's reply text. The handler may perform multiple Brain+tool
	// rounds internally before returning the final reply to speak.
	HandleUtterance(ctx context.Context, userText string) (reply string, err error)
}
