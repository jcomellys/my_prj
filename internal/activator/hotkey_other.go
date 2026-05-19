//go:build !darwin

package activator

import (
	"context"
	"fmt"
)

// Hotkey is a stub on non-darwin platforms so the rest of the code
// compiles in CI. The real implementation lives in hotkey_darwin.go.
type Hotkey struct {
	Combo string
}

func NewHotkey(combo string) *Hotkey { return &Hotkey{Combo: combo} }

func (Hotkey) Name() string { return "hotkey" }

func (Hotkey) Register() error {
	return fmt.Errorf("hotkey activator only supported on darwin in fase 0.3.2")
}

func (h *Hotkey) WaitForActivation(ctx context.Context) error {
	return fmt.Errorf("hotkey activator only supported on darwin")
}

// RunWithMainThread on non-darwin just calls loop directly — no main
// thread routing needed.
func RunWithMainThread(loop func()) { loop() }
