package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jcomellys/voice-mac-agent/internal/cost"
)

// TestShowCost_DescriptionCoversNaturalPhrasings is a guard so future edits
// don't accidentally narrow the trigger surface again. The brain ignored
// "cuánto llevo gastado hoy" once because the description over-emphasized
// the API/OpenAI angle.
func TestShowCost_DescriptionCoversNaturalPhrasings(t *testing.T) {
	spec := (&ShowCost{}).Spec()
	desc := strings.ToLower(spec.Description)

	for _, phrase := range []string{
		"cuánto llevo gastado",
		"cuánto he gastado",
		"how much have i spent",
		"spending",
		"cost",
	} {
		if !strings.Contains(desc, strings.ToLower(phrase)) {
			t.Errorf("show_cost description missing trigger phrase: %q\nfull desc: %s", phrase, spec.Description)
		}
	}
}

func TestShowCost_ExecuteTodayAndMonth(t *testing.T) {
	dir := t.TempDir()
	tr, err := cost.NewTracker(filepath.Join(dir, "cost.log"))
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	if err := tr.Record(cost.Entry{
		Timestamp:    time.Now(),
		BrainName:    "openai:gpt-5",
		InputTokens:  100,
		OutputTokens: 50,
		USD:          0.012,
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	sc := NewShowCost(tr)

	out, err := sc.Execute(context.Background(), `{"window":"today"}`)
	if err != nil {
		t.Fatalf("Execute today: %v", err)
	}
	if !strings.Contains(out, "today") || !strings.Contains(out, "0.0120") {
		t.Errorf("expected today summary with USD, got %q", out)
	}

	out, err = sc.Execute(context.Background(), `{"window":"month"}`)
	if err != nil {
		t.Fatalf("Execute month: %v", err)
	}
	if !strings.Contains(out, "month") {
		t.Errorf("expected month label, got %q", out)
	}

	_, err = sc.Execute(context.Background(), `{"window":"yesterday"}`)
	if err == nil {
		t.Error("expected error on invalid window")
	}
}

func TestShowCost_NilTracker(t *testing.T) {
	sc := NewShowCost(nil)
	out, err := sc.Execute(context.Background(), `{"window":"today"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "disabled") {
		t.Errorf("expected disabled message, got %q", out)
	}
}
