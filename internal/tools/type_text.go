package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/osadapter"
)

// TypeText types text into the frontmost app using real OS-level keystrokes
// (System Events `keystroke`), as if the user typed it. This works in ANY
// app — Claude/ChatGPT (whose composer is a contenteditable/React editor that
// ignores JS value-setting), Messages, Word, etc. — which a page-script
// approach cannot reliably do.
//
// Accessibility core: a user who cannot type dictates, and the agent types it
// for them. By contract (see the system prompt) the agent types WITHOUT
// sending, asks "¿lo envío?", and only presses Return on confirmation — so a
// message is never sent behind the user's back. The `submit` arg gates that.
//
// Needs macOS Accessibility permission (already required for the hotkey).
type TypeText struct {
	OS osadapter.Adapter
}

func NewTypeText(os osadapter.Adapter) *TypeText { return &TypeText{OS: os} }

func (TypeText) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name: "type_text",
		Description: "Type text into the frontmost app using real keystrokes (works in chat composers like Claude/ChatGPT, Messages, documents — anywhere the user can type). " +
			"Make sure the target app/window is focused first. By default it ONLY types and does NOT send: type the text with submit=false, ask the user '¿lo envío?', and only call again with submit=true (and empty text) to press Return once they confirm. " +
			"Do NOT use Chrome JavaScript to fill chat boxes — it fails on rich editors.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text":   map[string]any{"type": "string", "description": "Text to type. Leave empty to only press Return (when submit=true)."},
				"submit": map[string]any{"type": "boolean", "description": "If true, press Return after typing (sends/commits). Default false: only type, do not send."},
			},
			"required":             []string{"text"},
			"additionalProperties": false,
		},
	}
}

func (t *TypeText) Execute(ctx context.Context, argsJSON string) (Result, error) {
	var args struct {
		Text   string `json:"text"`
		Submit bool   `json:"submit"`
	}
	if err := UnmarshalArgs(argsJSON, &args); err != nil {
		return Result{}, err
	}
	if args.Text == "" && !args.Submit {
		return Result{}, fmt.Errorf("type_text: nothing to do (empty text and submit=false)")
	}

	var lines []string
	lines = append(lines, `tell application "System Events"`)
	if args.Text != "" {
		lines = append(lines, `	keystroke "`+escapeAppleScriptString(args.Text)+`"`)
	}
	if args.Submit {
		lines = append(lines, `	key code 36`) // Return
	}
	lines = append(lines, `end tell`)
	script := strings.Join(lines, "\n")

	if _, err := t.OS.RunAppleScript(ctx, script); err != nil {
		return Result{}, fmt.Errorf("type_text: %w", err)
	}

	switch {
	case args.Submit && args.Text != "":
		return Textf("Escrito y enviado."), nil
	case args.Submit:
		return Textf("Enviado."), nil
	default:
		return Textf("Texto escrito (sin enviar)."), nil
	}
}

// escapeAppleScriptString escapes a Go string for an AppleScript double-quoted
// literal. AppleScript supports \\, \", \n, \t, \r escape sequences.
func escapeAppleScriptString(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\t", `\t`,
		"\r", `\r`,
	)
	return r.Replace(s)
}
