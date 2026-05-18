// Package activator turns a user gesture (hotkey, wake word, USB presence,
// physical switch) into a "start listening" signal for the agent.
//
// The Activator interface is intentionally minimal: it blocks until the user
// indicates they want to talk, then returns. The orchestrator handles the
// rest. This lets us support many activation hardware options without
// touching the rest of the system.
package activator

import "context"

type Activator interface {
	Name() string

	// WaitForActivation blocks until the user signals readiness to talk.
	// Returns ctx.Err() if cancelled.
	WaitForActivation(ctx context.Context) error
}
