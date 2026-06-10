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

// ProgressKind classifies a ProgressEvent.
type ProgressKind string

const (
	ProgressStarted ProgressKind = "started"
	ProgressRound   ProgressKind = "round"
	ProgressToolOK  ProgressKind = "tool_ok"
	ProgressToolErr ProgressKind = "tool_error"
	ProgressDone    ProgressKind = "done"
)

// ProgressEvent is a structured, token-free signal of what the sub-agent is
// doing mid-task. The frontal narrates a subset with fixed phrases so the
// user isn't left in silence during a long delegation (fase 1 incremento 2;
// design: docs/design/fase1-incr2-mid-task-narration.md).
type ProgressEvent struct {
	Kind  ProgressKind
	Round int
	Tool  string
}

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
	return r.RunWithProgress(ctx, task, nil)
}

// RunWithProgress is Run with an optional progress channel. Sends are
// non-blocking: a slow or absent consumer never stalls the task. The caller
// owns the channel's lifetime (the runner never closes it).
func (r *Runner) RunWithProgress(ctx context.Context, task string, progress chan<- ProgressEvent) (string, error) {
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
	emit(ctx, progress, ProgressEvent{Kind: ProgressStarted})

	for round := 0; round < maxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		emit(ctx, progress, ProgressEvent{Kind: ProgressRound, Round: round})

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
			emit(ctx, progress, ProgressEvent{Kind: ProgressDone, Round: round})
			return resp.Text, nil
		}

		for _, tc := range resp.ToolCalls {
			res, terr := r.runTool(ctx, tc)
			if terr != nil {
				res = tools.Result{Text: "ERROR: " + terr.Error()}
				log.Warn("subagent.tool.error", "tool", tc.Name, "err", terr)
				emit(ctx, progress, ProgressEvent{Kind: ProgressToolErr, Round: round, Tool: tc.Name})
			} else {
				log.Info("subagent.tool.ok", "tool", tc.Name)
				emit(ctx, progress, ProgressEvent{Kind: ProgressToolOK, Round: round, Tool: tc.Name})
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

	return "", fmt.Errorf("subagent: se alcanzo MaxRounds=%d sin completar la tarea", maxRounds)
}

// emit sends a progress event without ever blocking the task: if the consumer
// is slow (buffer full) or the ctx died, the event is dropped. Narration is
// best-effort by design — losing a phrase is fine, stalling the task is not.
func emit(ctx context.Context, ch chan<- ProgressEvent, ev ProgressEvent) {
	if ch == nil {
		return
	}
	select {
	case ch <- ev:
	case <-ctx.Done():
	default:
	}
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
