package tools

import (
	"context"
	"fmt"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/cost"
)

// ShowCost exposes the local cost log to the brain. The brain can call it
// when the user asks "cuánto llevo gastado hoy" / "este mes" without us
// having to special-case those phrases.
type ShowCost struct {
	Tracker *cost.Tracker
}

func NewShowCost(t *cost.Tracker) *ShowCost { return &ShowCost{Tracker: t} }

func (ShowCost) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name: "show_cost",
		Description: "Returns how much money the user has spent using this voice agent (cumulative API usage cost). " +
			"Call this whenever the user asks about cost, spending, consumption, budget, or how much they have used — in any language and any phrasing. " +
			"Examples that should trigger this tool: \"cuánto llevo gastado hoy\", \"cuánto he gastado este mes\", \"how much have I spent\", " +
			"\"my usage\", \"cuánto consumí\", \"qué he gastado\". Window must be 'today' or 'month'.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"window": map[string]any{
					"type":        "string",
					"description": "Time window. 'today' = since 00:00 today. 'month' = since the 1st of this month.",
					"enum":        []string{"today", "month"},
				},
			},
			"required":             []string{"window"},
			"additionalProperties": false,
		},
	}
}

func (t *ShowCost) Execute(ctx context.Context, argsJSON string) (string, error) {
	if t.Tracker == nil {
		return "Cost tracking is disabled.", nil
	}
	var args struct {
		Window string `json:"window"`
	}
	if err := UnmarshalArgs(argsJSON, &args); err != nil {
		return "", err
	}

	var (
		s   cost.Summary
		err error
	)
	switch args.Window {
	case "today":
		s, err = t.Tracker.Today()
	case "month":
		s, err = t.Tracker.Month()
	default:
		return "", fmt.Errorf("invalid window %q (use today or month)", args.Window)
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s: %d calls, %d input tokens (%d cached), %d output tokens, $%.4f.",
		args.Window, s.Entries, s.InputTokens, s.CachedTokens, s.OutputTokens, s.USD), nil
}
