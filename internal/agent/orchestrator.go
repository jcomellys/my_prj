// Package agent is the orchestrator. It glues the Voice provider to the Brain
// and Tools, runs the conversation loop, and tracks cost.
//
// The orchestrator is intentionally small: each piece (Voice, Brain, Tools)
// is an interface. Swapping a provider is a config change, not a code change.
package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/cost"
	"github.com/jcomellys/voice-mac-agent/internal/tools"
	"github.com/jcomellys/voice-mac-agent/internal/voice"
)

// Orchestrator is the runtime composed in main.go.
type Orchestrator struct {
	Voice     voice.Provider
	Brain     brain.Brain
	Tools     *tools.Registry
	System    string // system prompt
	MaxRounds int    // hard cap on tool-call rounds per user turn (prevents loops)
	Log       *slog.Logger

	// Cost is optional; if nil, no logging is performed.
	Cost      *cost.Tracker
	Pricing   cost.Pricing // resolved from cost.Lookup at construction
	SessionID string

	// running conversation state — kept short by SummarizeIfLarge later
	history []brain.Message
}

func New(v voice.Provider, b brain.Brain, reg *tools.Registry, system string, log *slog.Logger) *Orchestrator {
	o := &Orchestrator{
		Voice:     v,
		Brain:     b,
		Tools:     reg,
		System:    system,
		MaxRounds: 6,
		Log:       log,
		SessionID: newSessionID(),
	}
	o.reset()
	return o
}

// WithCost attaches a cost tracker. Pricing is resolved from the brain's
// Name(); if unknown, cost is recorded as 0 and a warning is logged.
func (o *Orchestrator) WithCost(t *cost.Tracker) *Orchestrator {
	o.Cost = t
	if p, ok := cost.Lookup(o.Brain.Name()); ok {
		o.Pricing = p
	} else {
		o.Log.Warn("cost.pricing.unknown",
			"brain", o.Brain.Name(),
			"hint", "add an entry in internal/cost/pricing.go to track this brain's cost",
		)
	}
	return o
}

func newSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (o *Orchestrator) reset() {
	o.history = []brain.Message{{Role: brain.RoleSystem, Content: o.System}}
}

// Run starts the voice loop. Blocks until ctx is cancelled.
func (o *Orchestrator) Run(ctx context.Context) error {
	return o.Voice.Start(ctx, o)
}

// HandleUtterance implements voice.Handler.
//
// Loop: append user msg -> Brain -> if tool calls, execute and feed back -> repeat.
// Stops when the Brain returns text and no tool calls (final reply for the user).
func (o *Orchestrator) HandleUtterance(ctx context.Context, userText string) (string, error) {
	o.Log.Info("user.utterance", "text", userText)
	o.history = append(o.history, brain.Message{Role: brain.RoleUser, Content: userText})
	specs := o.Tools.Specs()

	for round := 0; round < o.MaxRounds; round++ {
		resp, err := o.Brain.Chat(ctx, o.history, specs)
		if err != nil {
			return "", fmt.Errorf("brain: %w", err)
		}
		usd := o.Pricing.USD(resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CachedInputTokens)
		o.Log.Info("brain.response",
			"round", round,
			"text_len", len(resp.Text),
			"tool_calls", len(resp.ToolCalls),
			"in_tokens", resp.Usage.InputTokens,
			"out_tokens", resp.Usage.OutputTokens,
			"cached_in_tokens", resp.Usage.CachedInputTokens,
			"usd", usd,
		)
		if o.Cost != nil {
			_ = o.Cost.Record(cost.Entry{
				SessionID:         o.SessionID,
				BrainName:         o.Brain.Name(),
				InputTokens:       resp.Usage.InputTokens,
				OutputTokens:      resp.Usage.OutputTokens,
				CachedInputTokens: resp.Usage.CachedInputTokens,
				USD:               usd,
			})
		}

		// Record the assistant turn (text + tool calls) in history.
		o.history = append(o.history, brain.Message{
			Role:      brain.RoleAssistant,
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})

		// No tools requested: final reply.
		if len(resp.ToolCalls) == 0 {
			return resp.Text, nil
		}

		// Execute each tool call and append its result (including any
		// attached images, e.g. from the screenshot tool).
		for _, tc := range resp.ToolCalls {
			res, err := o.runTool(ctx, tc)
			argsAudit := truncateForLog(tc.Arguments, 800)
			if err != nil {
				res = tools.Result{Text: "ERROR: " + err.Error()}
				o.Log.Warn("tool.error", "tool", tc.Name, "args", argsAudit, "err", err)
			} else {
				o.Log.Info("tool.ok", "tool", tc.Name, "args", argsAudit, "images", len(res.Images))
			}
			o.history = append(o.history, brain.Message{
				Role:       brain.RoleTool,
				Name:       tc.Name,
				ToolCallID: tc.ID,
				Content:    res.Text,
				Images:     res.Images,
			})
		}
	}

	return "Lo siento, no pude completar la tarea en un número razonable de pasos.", nil
}

func (o *Orchestrator) runTool(ctx context.Context, tc brain.ToolCall) (tools.Result, error) {
	t, ok := o.Tools.Get(tc.Name)
	if !ok {
		return tools.Result{}, fmt.Errorf("unknown tool: %s", tc.Name)
	}
	return t.Execute(ctx, tc.Arguments)
}

// truncateForLog shortens long tool arguments so structured logs stay
// readable but the AppleScript / shell command actually run is still
// visible for audit. Newlines collapsed to one-line for grep-friendliness.
func truncateForLog(s string, max int) string {
	if len(s) > max {
		s = s[:max] + "…"
	}
	// Collapse newlines so log lines stay scannable.
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' || c == '\r' {
			out = append(out, ' ')
		} else {
			out = append(out, c)
		}
	}
	return string(out)
}
