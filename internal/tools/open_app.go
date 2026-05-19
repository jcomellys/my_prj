package tools

import (
	"context"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/osadapter"
)

// OpenApp opens a named application via the OS adapter.
type OpenApp struct {
	OS osadapter.Adapter
}

func NewOpenApp(os osadapter.Adapter) *OpenApp { return &OpenApp{OS: os} }

func (OpenApp) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name:        "open_app",
		Description: "Open an application on the user's computer by name. Example names: 'Google Chrome', 'Microsoft Word', 'Terminal'.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Exact application name as it appears in the Applications folder.",
				},
			},
			"required":             []string{"name"},
			"additionalProperties": false,
		},
	}
}

func (t *OpenApp) Execute(ctx context.Context, argsJSON string) (Result, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := UnmarshalArgs(argsJSON, &args); err != nil {
		return Result{}, err
	}
	if err := t.OS.OpenApp(ctx, args.Name); err != nil {
		return Result{}, err
	}
	return Textf("Opened %s.", args.Name), nil
}
