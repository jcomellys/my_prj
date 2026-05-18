//go:build !darwin

package osadapter

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Stub is a non-darwin Adapter used during development on Linux/CI.
// OpenApp and RunAppleScript return errors; RunShell works as a real shell
// so unit tests can exercise the Tool layer end-to-end.
type Stub struct{}

func NewMacOS() *Stub { return &Stub{} }

func (Stub) Name() string { return "stub" }

func (Stub) OpenApp(ctx context.Context, name string) error {
	return fmt.Errorf("OpenApp not supported on this OS; build for darwin")
}

func (Stub) RunShell(ctx context.Context, command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("RunShell: empty command")
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (Stub) RunAppleScript(ctx context.Context, script string) (string, error) {
	return "", fmt.Errorf("RunAppleScript not supported on this OS; build for darwin")
}
