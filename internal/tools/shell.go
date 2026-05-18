package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/osadapter"
)

// Shell runs a shell command. To prevent the Brain from running arbitrary
// destructive commands silently, an allowlist of prefixes can be enforced.
// If AllowUnrestricted is false, only commands beginning with one of
// AllowPrefixes are executed.
type Shell struct {
	OS                osadapter.Adapter
	AllowUnrestricted bool
	AllowPrefixes     []string
}

func NewShell(os osadapter.Adapter, allowUnrestricted bool, allowPrefixes []string) *Shell {
	return &Shell{OS: os, AllowUnrestricted: allowUnrestricted, AllowPrefixes: allowPrefixes}
}

func (Shell) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name:        "run_shell",
		Description: "Run a shell command on the user's Mac. Use this for filesystem reads, simple utilities, and `open <url>`. Avoid destructive commands.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The exact shell command to run.",
				},
			},
			"required":             []string{"command"},
			"additionalProperties": false,
		},
	}
}

func (t *Shell) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := UnmarshalArgs(argsJSON, &args); err != nil {
		return "", err
	}
	cmd := strings.TrimSpace(args.Command)
	if cmd == "" {
		return "", fmt.Errorf("empty command")
	}
	if !t.AllowUnrestricted && !t.allowed(cmd) {
		return "", fmt.Errorf("command not on allowlist: %q", cmd)
	}
	out, err := t.OS.RunShell(ctx, cmd)
	out = strings.TrimSpace(out)
	if err != nil {
		if out != "" {
			return "", fmt.Errorf("%w: %s", err, out)
		}
		return "", err
	}
	if len(out) > 2000 {
		// Keep model-facing output bounded; long results burn tokens.
		out = out[:2000] + "\n…(truncated)"
	}
	return out, nil
}

func (t *Shell) allowed(cmd string) bool {
	for _, p := range t.AllowPrefixes {
		if strings.HasPrefix(cmd, p) {
			return true
		}
	}
	return false
}
