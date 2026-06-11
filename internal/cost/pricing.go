// Package cost tracks token usage and dollar cost per brain call.
package cost

import (
	"strings"
)

// Pricing is per 1M tokens, in USD. Cached input is the discounted rate
// applied to cached_input_tokens (Anthropic: ~10% of input; OpenAI: ~10%).
type Pricing struct {
	InputPer1M       float64
	OutputPer1M      float64
	CachedInputPer1M float64
}

// USD computes the dollar cost of one brain call given the token breakdown.
// Cached input tokens are billed at CachedInputPer1M, regular input at
// InputPer1M; output always at OutputPer1M.
func (p Pricing) USD(inputTokens, outputTokens, cachedInputTokens int) float64 {
	regularInput := inputTokens - cachedInputTokens
	if regularInput < 0 {
		// Some APIs report input as the total and cached as a subset;
		// others may report them separately. Clamp to avoid negatives.
		regularInput = 0
	}
	const million = 1_000_000.0
	return (float64(regularInput)*p.InputPer1M +
		float64(cachedInputTokens)*p.CachedInputPer1M +
		float64(outputTokens)*p.OutputPer1M) / million
}

// DefaultPrices is a best-effort table at the time of writing. Prices change.
// Production deployments should override via config; treat these as ballparks,
// not contracts. Keys are matched case-insensitively against brain.Name(),
// which is provider:model (e.g., "openai:gpt-5"). Lookup falls back to a
// prefix match, then to a zero-pricing entry that warns in logs.
var DefaultPrices = map[string]Pricing{
	// OpenAI — verify at https://openai.com/api/pricing
	"openai:gpt-5":      {InputPer1M: 5.00, OutputPer1M: 20.00, CachedInputPer1M: 0.50},
	"openai:gpt-5-mini": {InputPer1M: 0.25, OutputPer1M: 1.00, CachedInputPer1M: 0.025},

	// Anthropic — verify at https://www.anthropic.com/pricing
	"anthropic:claude-opus-4-7":   {InputPer1M: 15.00, OutputPer1M: 75.00, CachedInputPer1M: 1.50},
	"anthropic:claude-sonnet-4-6": {InputPer1M: 3.00, OutputPer1M: 15.00, CachedInputPer1M: 0.30},
	"anthropic:claude-haiku-4-5":  {InputPer1M: 1.00, OutputPer1M: 5.00, CachedInputPer1M: 0.10},

	// Local — free
	"ollama:": {InputPer1M: 0, OutputPer1M: 0, CachedInputPer1M: 0},

	// Mock — free
	"mock": {InputPer1M: 0, OutputPer1M: 0, CachedInputPer1M: 0},
}

// overrides holds user-supplied pricing from config. Written once at startup
// (before any brain call), read afterwards; checked before DefaultPrices so
// the user wins when prices change.
var overrides = map[string]Pricing{}

// Override registers (or replaces) the pricing for a brain name. Call at
// startup only — Lookup reads the map without locking once the agent runs.
func Override(brainName string, p Pricing) {
	overrides[strings.ToLower(strings.TrimSpace(brainName))] = p
}

// Lookup resolves a brain.Name() like "openai:gpt-5" to a Pricing entry.
// User overrides win, then exact default match, then a prefix match (so a
// provider with multiple model versions can share a row). Returns zero
// Pricing and false if no rule matches — caller should log a warning.
func Lookup(brainName string) (Pricing, bool) {
	name := strings.ToLower(brainName)
	if p, ok := overrides[name]; ok {
		return p, true
	}
	for prefix, p := range overrides {
		if strings.HasSuffix(prefix, ":") && strings.HasPrefix(name, prefix) {
			return p, true
		}
	}
	if p, ok := DefaultPrices[name]; ok {
		return p, true
	}
	for prefix, p := range DefaultPrices {
		if strings.HasSuffix(prefix, ":") && strings.HasPrefix(name, prefix) {
			return p, true
		}
		if prefix == name {
			return p, true
		}
	}
	return Pricing{}, false
}
