// Package osadapter is the OS-specific control surface. Mac, Windows, Linux
// will each provide an implementation. Only this layer needs to be rewritten
// when porting; the rest of the agent is portable.
//
// The interface deliberately exposes high-level operations (open an app,
// run a script) rather than low-level primitives (click x,y, send keystroke).
// High-level ops are more reliable, faster, and far more accessible-friendly
// than pixel-based automation.
package osadapter

import "context"

type Adapter interface {
	Name() string

	// OpenApp launches an app by name (e.g., "Google Chrome").
	OpenApp(ctx context.Context, name string) error

	// RunShell executes a shell command and returns combined stdout+stderr.
	// The caller is responsible for command-injection safety. The Tool layer
	// enforces an allowlist before calling this.
	RunShell(ctx context.Context, command string) (string, error)

	// RunAppleScript executes an AppleScript via osascript. On non-darwin
	// adapters this returns an error.
	RunAppleScript(ctx context.Context, script string) (string, error)
}
