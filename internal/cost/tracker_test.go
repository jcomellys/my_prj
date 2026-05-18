package cost

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTracker_RecordAndSince(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cost.log")
	tr, err := NewTracker(path)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	entries := []Entry{
		{Timestamp: yesterday, BrainName: "openai:gpt-5", InputTokens: 100, OutputTokens: 50, USD: 0.005},
		{Timestamp: now, BrainName: "openai:gpt-5", InputTokens: 200, OutputTokens: 80, USD: 0.011},
		{Timestamp: now, BrainName: "anthropic:claude-haiku-4-5", InputTokens: 50, OutputTokens: 30, USD: 0.0002, CachedInputTokens: 40},
	}
	for _, e := range entries {
		if err := tr.Record(e); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	// "Today" should exclude yesterday's entry.
	s, err := tr.Since(now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("Since: %v", err)
	}
	if s.Entries != 2 {
		t.Errorf("expected 2 entries since 1h ago, got %d", s.Entries)
	}
	if s.InputTokens != 250 || s.OutputTokens != 110 || s.CachedTokens != 40 {
		t.Errorf("token aggregation wrong: %+v", s)
	}
	if absf(s.USD-(0.011+0.0002)) > 1e-9 {
		t.Errorf("usd aggregation wrong: %v", s.USD)
	}
}

func TestTracker_EmptyFileReturnsZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.log")
	tr, err := NewTracker(path)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	s, err := tr.Today()
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if s.Entries != 0 || s.USD != 0 {
		t.Errorf("expected zero summary, got %+v", s)
	}
}

func TestTracker_MalformedLinesSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cost.log")
	tr, err := NewTracker(path)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	// Hand-write a malformed line plus a good one.
	if err := writeRaw(path, "not-json\n{\"ts\":\""+time.Now().UTC().Format(time.RFC3339Nano)+"\",\"brain\":\"x\",\"in\":1,\"out\":2,\"usd\":0.1}\n"); err != nil {
		t.Fatalf("writeRaw: %v", err)
	}
	s, err := tr.Since(time.Time{})
	if err != nil {
		t.Fatalf("Since: %v", err)
	}
	if s.Entries != 1 || s.InputTokens != 1 {
		t.Errorf("expected one good entry, got %+v", s)
	}
}

func TestPricing_USD(t *testing.T) {
	p := Pricing{InputPer1M: 5, OutputPer1M: 20, CachedInputPer1M: 0.5}

	// 1000 input total, 200 of which cached. 500 output.
	// regular = 800. cost = (800*5 + 200*0.5 + 500*20) / 1e6
	got := p.USD(1000, 500, 200)
	want := (800.0*5 + 200.0*0.5 + 500.0*20) / 1_000_000.0
	if absf(got-want) > 1e-9 {
		t.Errorf("USD = %v, want %v", got, want)
	}
}

func TestPricing_LookupExactAndPrefix(t *testing.T) {
	if _, ok := Lookup("openai:gpt-5"); !ok {
		t.Error("expected openai:gpt-5 in default prices")
	}
	if _, ok := Lookup("ollama:llama3.3:8b"); !ok {
		t.Error("expected prefix match for ollama:")
	}
	if _, ok := Lookup("alien:model"); ok {
		t.Error("expected unknown brain to return false")
	}
}

// --- helpers ---

func absf(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func writeRaw(path, contents string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(contents)
	return err
}
