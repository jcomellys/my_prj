//go:build darwin

package osadapter

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// MacOS is the Adapter for macOS. It uses `open`, `osascript`, and shell.
//
// AX (Accessibility) tree reads land in fase 0.3; for now we lean on
// AppleScript and `open` which together cover most app launches and the
// majority of in-app commands for the apps the agent will care about.
type MacOS struct{}

func NewMacOS() *MacOS { return &MacOS{} }

func (MacOS) Name() string { return "macos" }

func (MacOS) OpenApp(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("OpenApp: empty name")
	}
	// `open -a "App Name"` is the safe, escape-clean form.
	cmd := exec.CommandContext(ctx, "open", "-a", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("open -a %q: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (MacOS) RunShell(ctx context.Context, command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("RunShell: empty command")
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (MacOS) RunAppleScript(ctx context.Context, script string) (string, error) {
	if strings.TrimSpace(script) == "" {
		return "", fmt.Errorf("RunAppleScript: empty script")
	}
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
