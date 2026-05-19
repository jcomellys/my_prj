package agent

import (
	"strings"
	"testing"
)

// TestDefaultSystemPrompt_MentionsKeyTools ensures that adjustments to the
// system prompt do not drop the explicit guidance the brain needs to call
// show_cost on natural utterances like "cuánto llevo gastado hoy" without
// requiring the user to mention "OpenAI" or "API".
func TestDefaultSystemPrompt_MentionsKeyTools(t *testing.T) {
	required := []string{
		"show_cost",
		"open_app",
		"run_applescript",
		"screenshot",
		// Phrasings of the cost intent the brain should recognize.
		"gasto",
		"costo",
		// Phrasing of the visual-intent the brain should recognize.
		"pantalla",
		// Chrome-via-AppleScript patterns (fase 0.3.3a).
		"Chrome",
		"execute",
		"Apple Events",
	}
	for _, s := range required {
		if !strings.Contains(strings.ToLower(DefaultSystemPrompt), strings.ToLower(s)) {
			t.Errorf("DefaultSystemPrompt missing required guidance: %q", s)
		}
	}
}
