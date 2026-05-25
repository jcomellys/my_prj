// Package subagent runs an autonomous, multi-step task on a strong brain and
// returns a final summary. The frontal (conversational) agent delegates a
// hard task here via the delegate_task tool; the sub-agent loops brain+tools
// many times on its own, then reports back.
//
// It deliberately reuses the same brain.Brain and tools.Registry abstractions
// as the frontal agent, so the same OS control surface is available. Safety
// stays where it already is: the Shell tool's allowlist bounds command
// execution unless the operator widens it. The sub-agent honors context
// cancellation, so a barge-in (hotkey) on the frontal aborts a long task.
package subagent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/cost"
	"github.com/jcomellys/voice-mac-agent/internal/tools"
)

// Runner executes one delegated task to completion.
type Runner struct {
	Brain     brain.Brain     // typically the deep brain (gpt-5)
	Tools     *tools.Registry // tool surface available to the sub-agent
	System    string          // sub-agent system prompt
	MaxRounds int             // bounds worst-case cost/time; default 24
	Log       *slog.Logger

	// Cost is optional; if set, every brain call is recorded and attributed
	// to the sub-agent's brain name.
	Cost      *cost.Tracker
	SessionID string
}

func New(b brain.Brain, reg *tools.Registry, system string, log *slog.Logger) *Runner {
	return &Runner{
		Brain:     b,
		Tools:     reg,
		System:    system,
		MaxRounds: 24,
		Log:       log,
	}
}

// Run executes the task autonomously and returns the final text result.
// Honors ctx cancellation (barge-in / Ctrl-C) at every round.
func (r *Runner) Run(ctx context.Context, task string) (string, error) {
	maxRounds := r.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 24
	}
	log := r.Log
	if log == nil {
		log = slog.Default()
	}

	history := []brain.Message{
		{Role: brain.RoleSystem, Content: r.System},
		{Role: brain.RoleUser, Content: task},
	}
	specs := r.Tools.Specs()

	log.Info("subagent.start", "brain", r.Brain.Name(), "task", truncate(task, 200))

	for round := 0; round < maxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		resp, err := r.Brain.Chat(ctx, history, specs)
		if err != nil {
			return "", fmt.Errorf("subagent brain: %w", err)
		}

		if r.Cost != nil {
			pricing, _ := cost.Lookup(r.Brain.Name())
			usd := pricing.USD(resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CachedInputTokens)
			_ = r.Cost.Record(cost.Entry{
				SessionID:         r.SessionID,
				BrainName:         r.Brain.Name(),
				InputTokens:       resp.Usage.InputTokens,
				OutputTokens:      resp.Usage.OutputTokens,
				CachedInputTokens: resp.Usage.CachedInputTokens,
				USD:               usd,
			})
			log.Info("subagent.round", "round", round, "tool_calls", len(resp.ToolCalls), "usd", usd)
		}

		history = append(history, brain.Message{
			Role:      brain.RoleAssistant,
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})

		if len(resp.ToolCalls) == 0 {
			log.Info("subagent.done", "rounds", round+1)
			return resp.Text, nil
		}

		for _, tc := range resp.ToolCalls {
			res, terr := r.runTool(ctx, tc)
			if terr != nil {
				res = tools.Result{Text: "ERROR: " + terr.Error()}
				log.Warn("subagent.tool.error", "tool", tc.Name, "err", terr)
			} else {
				log.Info("subagent.tool.ok", "tool", tc.Name)
			}
			history = append(history, brain.Message{
				Role:       brain.RoleTool,
				Name:       tc.Name,
				ToolCallID: tc.ID,
				Content:    res.Text,
				Images:     res.Images,
			})
		}
	}

	return "No pude completar la tarea delegada en el número máximo de pasos. " +
		"Resumen de lo intentado puede no estar completo.", nil
}

func (r *Runner) runTool(ctx context.Context, tc brain.ToolCall) (tools.Result, error) {
	t, ok := r.Tools.Get(tc.Name)
	if !ok {
		return tools.Result{}, fmt.Errorf("unknown tool: %s", tc.Name)
	}
	return t.Execute(ctx, tc.Arguments)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
