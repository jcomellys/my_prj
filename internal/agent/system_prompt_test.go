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
		// Fase 0.3.3a v2: open_app boundary + STT-noise discipline.
		"NUNCA uses open_app",
		"Wikipedia",
		"Transcripción ambigua",
		"repetir",
		// Time should resolve via AppleScript, not run_shell.
		"current date",
		// Fase 0.4: English bundle names for open_app (localization fix).
		"Messages",
		"nombre natural en español",
		"Mensajes",
		// Fase 0.4.2A: bounded long answers in blocks.
		"por bloques",
		"700-1000 caracteres",
		"también breve",
		"sin listas largas",
		"¿Quieres que continúe?",
	}
	for _, s := range required {
		if !strings.Contains(strings.ToLower(DefaultSystemPrompt), strings.ToLower(s)) {
			t.Errorf("DefaultSystemPrompt missing required guidance: %q", s)
		}
	}
}
