//go:build darwin

package activator

import (
	"context"
	"fmt"
	"strings"

	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
)

// Hotkey is a global system hotkey activator. On macOS this uses
// Cocoa via cgo (provided by golang.design/x/hotkey). Each press of
// the combination returns from WaitForActivation; the agent then
// records one voice turn.
//
// Important macOS quirk: the hotkey library MUST run on the main OS
// thread (Cocoa requirement). We use golang.design/x/hotkey/mainthread
// for that. Because of this, main.go imports a helper to wrap main().
type Hotkey struct {
	// Combo is the textual representation, e.g. "ctrl+option+space".
	// Parsed in Register().
	Combo string

	hk      *hotkey.Hotkey
	keydown <-chan hotkey.Event
}

func NewHotkey(combo string) *Hotkey {
	if combo == "" {
		combo = "ctrl+option+space"
	}
	return &Hotkey{Combo: combo}
}

func (Hotkey) Name() string { return "hotkey" }

// Register parses the combo string and registers the hotkey with the
// OS. Call once at startup, BEFORE the conversation loop starts.
func (h *Hotkey) Register() error {
	mods, key, err := parseCombo(h.Combo)
	if err != nil {
		return err
	}
	hk := hotkey.New(mods, key)
	if err := hk.Register(); err != nil {
		return fmt.Errorf("hotkey register %q: %w", h.Combo, err)
	}
	h.hk = hk
	h.keydown = hk.Keydown()
	return nil
}

func (h *Hotkey) WaitForActivation(ctx context.Context) error {
	if h.hk == nil {
		if err := h.Register(); err != nil {
			return err
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-h.keydown:
		return nil
	}
}

// parseCombo turns "ctrl+option+space" into the hotkey library's
// modifier slice + Key. Supports: ctrl, option (alt), shift, cmd
// (super), and letter/space/enter keys.
func parseCombo(combo string) ([]hotkey.Modifier, hotkey.Key, error) {
	parts := strings.Split(strings.ToLower(combo), "+")
	if len(parts) < 2 {
		return nil, 0, fmt.Errorf("hotkey combo needs at least one modifier and one key: %q", combo)
	}
	var mods []hotkey.Modifier
	var key hotkey.Key
	for i, p := range parts {
		p = strings.TrimSpace(p)
		last := i == len(parts)-1
		if last {
			switch p {
			case "space":
				key = hotkey.KeySpace
			case "enter", "return":
				key = hotkey.KeyReturn
			default:
				if len(p) == 1 && p[0] >= 'a' && p[0] <= 'z' {
					key = hotkey.Key(hotkey.KeyA) + hotkey.Key(p[0]-'a')
				} else {
					return nil, 0, fmt.Errorf("unsupported key %q", p)
				}
			}
			continue
		}
		switch p {
		case "ctrl", "control":
			mods = append(mods, hotkey.ModCtrl)
		case "option", "alt":
			mods = append(mods, hotkey.ModOption)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "cmd", "command", "super":
			mods = append(mods, hotkey.ModCmd)
		default:
			return nil, 0, fmt.Errorf("unsupported modifier %q", p)
		}
	}
	return mods, key, nil
}

// RunWithMainThread wraps the agent's main loop so the macOS Cocoa
// event pump can run on the OS main thread. Pass the agent's main
// loop function; this blocks until it returns.
func RunWithMainThread(loop func()) {
	mainthread.Init(loop)
}

// SupportsBargeIn: a global hotkey is the canonical interrupt gesture.
func (*Hotkey) SupportsBargeIn() bool { return true }
