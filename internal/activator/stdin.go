package activator

import (
	"context"
	"fmt"
)

// AlwaysOn skips activation entirely — the agent listens continuously.
// Useful during early development when paired with stdin STT (the user is
// "activating" each turn by pressing Enter to send a line).
type AlwaysOn struct{}

func NewAlwaysOn() *AlwaysOn { return &AlwaysOn{} }

func (AlwaysOn) Name() string { return "always_on" }

func (AlwaysOn) WaitForActivation(ctx context.Context) error {
	// No-op activator: return immediately. Cooperate with ctx.
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// PrintOnce prints a banner the first time it's called, then behaves like AlwaysOn.
// Used to remind the user during dev that the agent is ready.
type PrintOnce struct {
	printed bool
	Banner  string
}

func NewPrintOnce(banner string) *PrintOnce {
	return &PrintOnce{Banner: banner}
}

func (p *PrintOnce) Name() string { return "print_once" }

func (p *PrintOnce) WaitForActivation(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !p.printed {
		fmt.Println(p.Banner)
		p.printed = true
	}
	return nil
}
