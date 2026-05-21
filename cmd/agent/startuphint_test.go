package main

import (
	"strings"
	"testing"

	"github.com/jcomellys/voice-mac-agent/internal/agent"
)

func TestStartupHint_MatchesActivator(t *testing.T) {
	cases := []struct {
		ac       agent.ActivatorConfig
		wantSub  string
		wantNoEn bool // must NOT mention pressing Enter
	}{
		{agent.ActivatorConfig{Kind: "hotkey", Hotkey: agent.HotkeyConfig{Combo: "ctrl+alt+space"}}, "ctrl+alt+space", true},
		{agent.ActivatorConfig{Kind: "hotkey"}, "ctrl+option+space", true},
		{agent.ActivatorConfig{Kind: "usb_presence", USB: agent.USBPresenceConfig{VolumeName: "AGENT"}}, "USB AGENT", true},
		{agent.ActivatorConfig{Kind: "enter"}, "Enter", false},
		{agent.ActivatorConfig{Kind: "stdin"}, "Enter", false},
	}
	for _, c := range cases {
		got := startupHint(c.ac)
		if !strings.Contains(got, c.wantSub) {
			t.Errorf("kind=%q hint=%q, want substring %q", c.ac.Kind, got, c.wantSub)
		}
		if c.wantNoEn && strings.Contains(strings.ToLower(got), "enter") {
			t.Errorf("kind=%q must not tell the user to press Enter, got %q", c.ac.Kind, got)
		}
	}
}
