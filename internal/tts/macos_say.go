package tts

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// MacOSSay uses the macOS `say` command. Free, on-device, decent quality.
// On macOS Tahoe (26+) the bundled voices are noticeably better than prior
// versions; users can pick a Premium/Personal voice in System Settings.
type MacOSSay struct {
	Voice string // empty = system default (e.g., "Mónica", "Samantha")
	Rate  int    // words per minute; 0 = system default
}

func NewMacOSSay(voice string, rate int) *MacOSSay {
	return &MacOSSay{Voice: voice, Rate: rate}
}

func (m *MacOSSay) Name() string { return "macos_say" }

func (m *MacOSSay) Speak(ctx context.Context, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	args := []string{}
	if m.Voice != "" {
		args = append(args, "-v", m.Voice)
	}
	if m.Rate > 0 {
		args = append(args, "-r", strconv.Itoa(m.Rate))
	}
	args = append(args, text)

	cmd := exec.CommandContext(ctx, "say", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("say: %w", err)
	}
	return nil
}
