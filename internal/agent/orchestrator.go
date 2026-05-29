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
	"time"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/cost"
	"github.com/jcomellys/voice-mac-agent/internal/tools"
	"github.com/jcomellys/voice-mac-agent/internal/voice"
)

// escalateToolName is the synthetic tool the cheap brain calls to hand the
// rest of a turn to the deep brain. It is never dispatched to the registry;
// the orchestrator intercepts it.
const escalateToolName = "escalate"

// Orchestrator is the runtime composed in main.go.
type Orchestrator struct {
	Voice     voice.Provider
	Brain     brain.Brain // default (cheap) brain
	DeepBrain brain.Brain // optional stronger brain; nil disables escalation
	Tools     *tools.Registry
	System    string // system prompt
	MaxRounds int    // hard cap on tool-call rounds per user turn (prevents loops)
	Log       *slog.Logger

	// Cost is optional; if nil, no logging is performed. Pricing is resolved
	// per call from the active brain's Name() so two-tier costs attribute
	// correctly to cheap vs deep.
	Cost      *cost.Tracker
	SessionID string

	// Budget guard. MonthlyBudgetUSD <= 0 means no limit. When month-to-date
	// spend reaches the budget the agent refuses turns (spoken). When it
	// crosses WarnAtPct it appends a one-time spoken heads-up.
	MonthlyBudgetUSD float64
	WarnAtPct        int
	warnedBudget     bool

	// warnedPricing dedups the "unknown pricing" warning to once per brain
	// name so an unpriced model doesn't spam the log every turn.
	warnedPricing map[string]bool

	// running conversation state — kept short by SummarizeIfLarge later
	history []brain.Message
}

func New(v voice.Provider, b brain.Brain, reg *tools.Registry, system string, log *slog.Logger) *Orchestrator {
	o := &Orchestrator{
		Voice:         v,
		Brain:         b,
		Tools:         reg,
		System:        system,
		MaxRounds:     6,
		Log:           log,
		SessionID:     newSessionID(),
		warnedPricing: map[string]bool{},
	}
	o.reset()
	return o
}

// WithCost attaches a cost tracker.
func (o *Orchestrator) WithCost(t *cost.Tracker) *Orchestrator {
	o.Cost = t
	return o
}

// WithBudget sets a month-to-date spend cap. monthlyUSD <= 0 disables the
// hard stop. warnAtPct (1-100) triggers a one-time spoken heads-up when
// crossed; <= 0 disables the warning. Requires a cost tracker to do anything.
func (o *Orchestrator) WithBudget(monthlyUSD float64, warnAtPct int) *Orchestrator {
	o.MonthlyBudgetUSD = monthlyUSD
	o.WarnAtPct = warnAtPct
	return o
}

// WithDeepBrain enables two-tier routing: the default Brain handles simple
// turns cheaply and can escalate to deep via the `escalate` tool. Passing
// nil leaves single-tier behavior unchanged.
func (o *Orchestrator) WithDeepBrain(b brain.Brain) *Orchestrator {
	o.DeepBrain = b
	return o
}

// escalateSpec is offered to the cheap brain only. It lets the model itself
// decide a turn is too hard, keeping classification cost to a single cheap
// call instead of a separate router model.
func escalateSpec() brain.ToolSpec {
	return brain.ToolSpec{
		Name: escalateToolName,
		Description: "Cambia a un modelo experto más potente que continúa este turno. " +
			"DEBES escalar (llamar esta tool) en estos casos, aunque creas que podrías responder tú: " +
			"enseñar o explicar un concepto a fondo, analizar un libro/texto/circuito/imagen, " +
			"crear un plan de estudio o de trabajo, resolver matemática, escribir o depurar código, " +
			"razonamiento de varios pasos, investigación, o cualquier respuesta educativa larga. " +
			"NO escales para acciones simples: abrir apps, decir la hora, navegar una URL, cerrar " +
			"pestañas, preguntas de una sola frase. Esas resuélvelas tú directamente. " +
			"Ante la duda entre enseñar/analizar/planear: escala.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"reason": map[string]any{
					"type":        "string",
					"description": "Motivo breve de por qué necesita el experto.",
				},
			},
			"required":             []string{"reason"},
			"additionalProperties": false,
		},
	}
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

	// Budget hard stop: refuse before spending anything if month-to-date
	// spend has reached the cap. Spoken so a non-sighted user understands.
	if reply, stop := o.budgetHardStop(); stop {
		return reply, nil
	}

	o.history = append(o.history, brain.Message{Role: brain.RoleUser, Content: userText})

	activeBrain := o.Brain

	for round := 0; round < o.MaxRounds; round++ {
		// Offer the escalate tool only while running on the cheap brain.
		specs := o.Tools.Specs()
		if o.DeepBrain != nil && activeBrain != o.DeepBrain {
			specs = append(specs, escalateSpec())
		}

		brainStart := time.Now()
		resp, err := activeBrain.Chat(ctx, o.history, specs)
		brainMs := time.Since(brainStart).Milliseconds()
		if err != nil {
			return "", fmt.Errorf("brain: %w", err)
		}

		pricing, priced := cost.Lookup(activeBrain.Name())
		if !priced && !o.warnedPricing[activeBrain.Name()] {
			o.warnedPricing[activeBrain.Name()] = true
			o.Log.Warn("cost.pricing.unknown",
				"brain", activeBrain.Name(),
				"hint", "add an entry in internal/cost/pricing.go; cost recorded as $0 until then",
			)
		}
		usd := pricing.USD(resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CachedInputTokens)
		o.Log.Info("brain.response",
			"brain", activeBrain.Name(),
			"round", round,
			"dur_ms", brainMs,
			"text_len", len(resp.Text),
			"text", truncateForLog(resp.Text, 500),
			"tool_calls", len(resp.ToolCalls),
			"in_tokens", resp.Usage.InputTokens,
			"out_tokens", resp.Usage.OutputTokens,
			"cached_in_tokens", resp.Usage.CachedInputTokens,
			"usd", usd,
		)
		if o.Cost != nil {
			_ = o.Cost.Record(cost.Entry{
				SessionID:         o.SessionID,
				BrainName:         activeBrain.Name(),
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
			return o.maybeAppendBudgetWarning(resp.Text), nil
		}

		// Speak the model's preamble immediately (e.g. "Voy a leer esa
		// sección, dame un momento.") so a non-sighted user is not left in
		// silence while a slow tool (read_pdf, app scripting) runs. Uses the
		// turn's ctx, so a barge-in cuts it too.
		if resp.Text != "" {
			o.narrate(ctx, resp.Text)
		}

		// Execute each tool call. The synthetic `escalate` call is handled
		// by the orchestrator (switch to the deep brain) and never reaches
		// the registry.
		for _, tc := range resp.ToolCalls {
			if tc.Name == escalateToolName {
				if o.DeepBrain != nil && activeBrain != o.DeepBrain {
					o.Log.Info("brain.escalate",
						"from", activeBrain.Name(),
						"to", o.DeepBrain.Name(),
						"reason", truncateForLog(tc.Arguments, 200),
					)
					activeBrain = o.DeepBrain
				}
				o.history = append(o.history, brain.Message{
					Role:       brain.RoleTool,
					Name:       tc.Name,
					ToolCallID: tc.ID,
					Content:    "Eres ahora el modelo experto. Atiende la petición del usuario completamente en los siguientes pasos.",
				})
				continue
			}

			toolStart := time.Now()
			res, err := o.runTool(ctx, tc)
			toolMs := time.Since(toolStart).Milliseconds()
			argsAudit := truncateForLog(tc.Arguments, 800)
			if err != nil {
				res = tools.Result{Text: "ERROR: " + err.Error()}
				o.Log.Warn("tool.error", "tool", tc.Name, "dur_ms", toolMs, "args", argsAudit, "err", err)
			} else {
				o.Log.Info("tool.ok", "tool", tc.Name, "dur_ms", toolMs, "args", argsAudit, "images", len(res.Images))
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

// budgetHardStop reports whether month-to-date spend has reached the cap.
// When it has, it returns a spoken refusal and stop=true so the caller
// returns before invoking any brain.
func (o *Orchestrator) budgetHardStop() (reply string, stop bool) {
	if o.Cost == nil || o.MonthlyBudgetUSD <= 0 {
		return "", false
	}
	s, err := o.Cost.Month()
	if err != nil {
		o.Log.Warn("budget.read_failed", "err", err)
		return "", false
	}
	if s.USD >= o.MonthlyBudgetUSD {
		o.Log.Warn("budget.exceeded", "month_usd", s.USD, "budget_usd", o.MonthlyBudgetUSD)
		return fmt.Sprintf(
			"Has alcanzado tu presupuesto mensual de %.2f dólares (llevas %.2f). "+
				"No puedo continuar hasta que subas el límite en la configuración.",
			o.MonthlyBudgetUSD, s.USD), true
	}
	return "", false
}

// maybeAppendBudgetWarning adds a one-time spoken heads-up to a reply when
// month-to-date spend crosses WarnAtPct of the budget.
func (o *Orchestrator) maybeAppendBudgetWarning(reply string) string {
	if o.warnedBudget || o.Cost == nil || o.MonthlyBudgetUSD <= 0 || o.WarnAtPct <= 0 {
		return reply
	}
	s, err := o.Cost.Month()
	if err != nil {
		return reply
	}
	threshold := o.MonthlyBudgetUSD * float64(o.WarnAtPct) / 100.0
	if s.USD < threshold {
		return reply
	}
	o.warnedBudget = true
	pct := int(s.USD / o.MonthlyBudgetUSD * 100)
	o.Log.Warn("budget.warn", "month_usd", s.USD, "budget_usd", o.MonthlyBudgetUSD, "pct", pct)
	notice := fmt.Sprintf(" Aviso: llevas gastado el %d por ciento de tu presupuesto mensual.", pct)
	if reply == "" {
		return notice
	}
	return reply + notice
}

// narrate speaks an interim message mid-turn if the voice provider supports
// it (the Pipeline does). Best-effort: failures are ignored so narration
// never breaks the turn.
func (o *Orchestrator) narrate(ctx context.Context, text string) {
	if s, ok := o.Voice.(interface {
		Speak(context.Context, string) error
	}); ok {
		_ = s.Speak(ctx, text)
	}
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
