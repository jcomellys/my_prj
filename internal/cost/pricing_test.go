package cost

import "testing"

func TestOverride_WinsOverDefaults(t *testing.T) {
	defer func() { overrides = map[string]Pricing{} }() // isolate

	Override("openai:gpt-5", Pricing{InputPer1M: 1.25, OutputPer1M: 10})
	p, ok := Lookup("openai:gpt-5")
	if !ok || p.InputPer1M != 1.25 || p.OutputPer1M != 10 {
		t.Errorf("override must win over defaults, got %+v ok=%v", p, ok)
	}

	Override("ollama:", Pricing{}) // provider-wide prefix
	if p, ok := Lookup("ollama:llama9"); !ok || p.InputPer1M != 0 {
		t.Errorf("prefix override must match, got %+v ok=%v", p, ok)
	}

	// Unrelated models still resolve from the default table.
	if _, ok := Lookup("openai:gpt-5-mini"); !ok {
		t.Error("defaults must still resolve when no override matches")
	}
}
