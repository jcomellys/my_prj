package tools

import (
	"context"
	"strings"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/osadapter"
)

// AppleScript runs an AppleScript snippet via osascript. This is the highest-
// leverage Mac tool: most user-facing apps (Chrome, Word, Pages, Finder,
// Mail, Keynote, Numbers, Music) have rich AppleScript dictionaries.
type AppleScript struct {
	OS osadapter.Adapter
}

func NewAppleScript(os osadapter.Adapter) *AppleScript { return &AppleScript{OS: os} }

func (AppleScript) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name:        "run_applescript",
		Description: "Run an AppleScript on the Mac. Use this for in-app automation: opening URLs in Chrome, controlling Word/Pages, moving Finder windows, etc. Prefer this over run_shell when the target is a GUI app.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"script": map[string]any{
					"type":        "string",
					"description": "Full AppleScript source. Multi-line is fine.",
				},
			},
			"required":             []string{"script"},
			"additionalProperties": false,
		},
	}
}

func (t *AppleScript) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Script string `json:"script"`
	}
	if err := UnmarshalArgs(argsJSON, &args); err != nil {
		return "", err
	}
	out, err := t.OS.RunAppleScript(ctx, args.Script)
	out = strings.TrimSpace(out)
	if err != nil {
		return "", err
	}
	if out == "" {
		return "OK", nil
	}
	if len(out) > 2000 {
		out = out[:2000] + "\n…(truncated)"
	}
	return out, nil
}
