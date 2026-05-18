package cost

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Tracker persists every brain call with token counts and USD cost.
//
// Storage is append-only NDJSON (one JSON object per line) rather than
// SQLite, deliberately:
//   - zero dependencies, stays in a single Go binary
//   - atomic O_APPEND writes survive crashes
//   - human-readable and grep-able
//   - trivially portable on a USB stick
//
// At scale a database would be better; if we ever need indexed queries
// or aggregation across years, swap this for modernc.org/sqlite behind
// the same interface.
type Tracker struct {
	mu   sync.Mutex
	path string
}

// Entry is one logged brain call.
type Entry struct {
	Timestamp         time.Time `json:"ts"`
	SessionID         string    `json:"session"`
	BrainName         string    `json:"brain"`
	InputTokens       int       `json:"in"`
	OutputTokens      int       `json:"out"`
	CachedInputTokens int       `json:"cached_in"`
	USD               float64   `json:"usd"`
}

// NewTracker opens (or creates) the cost log at path. Parent dirs are
// created if they don't exist.
func NewTracker(path string) (*Tracker, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create cost dir: %w", err)
	}
	// Touch the file to surface permission errors early.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open cost log: %w", err)
	}
	_ = f.Close()
	return &Tracker{path: path}, nil
}

// Record appends one entry to the log. Safe for concurrent callers.
func (t *Tracker) Record(e Entry) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	buf, err := json.Marshal(e)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	f, err := os.OpenFile(t.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(buf, '\n')); err != nil {
		return err
	}
	return nil
}

// Summary aggregates entries whose timestamp is >= since.
type Summary struct {
	Entries     int     `json:"entries"`
	InputTokens int     `json:"in"`
	OutputTokens int    `json:"out"`
	CachedTokens int    `json:"cached_in"`
	USD         float64 `json:"usd"`
}

// Since reads the log and returns a Summary of entries at or after `since`.
func (t *Tracker) Since(since time.Time) (Summary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	f, err := os.Open(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Summary{}, nil
		}
		return Summary{}, err
	}
	defer f.Close()

	var s Summary
	scanner := bufio.NewScanner(f)
	// Allow long lines; default 64k buffer is fine for our entries but harden it.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue // skip malformed lines rather than fail aggregation
		}
		if e.Timestamp.Before(since) {
			continue
		}
		s.Entries++
		s.InputTokens += e.InputTokens
		s.OutputTokens += e.OutputTokens
		s.CachedTokens += e.CachedInputTokens
		s.USD += e.USD
	}
	if err := scanner.Err(); err != nil {
		return Summary{}, err
	}
	return s, nil
}

// Today summarizes entries logged since 00:00 today (local time).
func (t *Tracker) Today() (Summary, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return t.Since(start)
}

// Month summarizes entries logged since the 1st of the current month (local).
func (t *Tracker) Month() (Summary, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return t.Since(start)
}
