package agent

import (
	"strings"
	"testing"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/tools"
)

// seedTurn appends a full turn block (user → assistant tool_call → tool
// result → assistant text) to the orchestrator history, mimicking what
// HandleUtterance records for a tool-using turn.
func seedTurn(o *Orchestrator, n int, toolResult string) {
	id := "call-" + strings.Repeat("x", 1) + string(rune('a'+n))
	o.history = append(o.history,
		brain.Message{Role: brain.RoleUser, Content: "petición " + string(rune('a'+n))},
		brain.Message{Role: brain.RoleAssistant, ToolCalls: []brain.ToolCall{{ID: id, Name: "read_pdf", Arguments: "{}"}}},
		brain.Message{Role: brain.RoleTool, Name: "read_pdf", ToolCallID: id, Content: toolResult},
		brain.Message{Role: brain.RoleAssistant, Content: "listo " + string(rune('a'+n))},
	)
}

func TestCompactHistory_UnderLimitUntouched(t *testing.T) {
	o := New(nil, &scriptedBrain{}, tools.NewRegistry(), "system", quietLogger())
	seedTurn(o, 0, "resultado corto")
	before := len(o.history)
	o.compactHistory()
	if len(o.history) != before {
		t.Errorf("history under the limit must not change: %d -> %d msgs", before, len(o.history))
	}
	if o.history[3].Content != "resultado corto" {
		t.Errorf("tool result modified under the limit: %q", o.history[3].Content)
	}
}

func TestCompactHistory_ShrinksOldToolResults(t *testing.T) {
	o := New(nil, &scriptedBrain{}, tools.NewRegistry(), "system", quietLogger())
	big := strings.Repeat("página del PDF ", 3000) // ~45 KB, over the soft limit
	seedTurn(o, 0, big)
	for n := 1; n <= keepRecentUserTurns; n++ { // push turn 0 out of the window
		seedTurn(o, n, "corto")
	}

	o.compactHistory()

	old := o.history[3] // turn 0's tool result (0 = system, 1 = user, 2 = tool_call)
	if old.Role != brain.RoleTool {
		t.Fatalf("layout drift: expected tool msg at index 3, got role %s", old.Role)
	}
	if len(old.Content) > compactToolResultKeep+len(compactedMarker) {
		t.Errorf("old tool result not shrunk: %d bytes", len(old.Content))
	}
	if !strings.Contains(old.Content, "recortado") {
		t.Errorf("shrunk result must carry the elision marker, got %q…", old.Content[:40])
	}
	// The pair must stay intact: assistant tool_call still precedes it.
	if len(o.history[2].ToolCalls) != 1 || o.history[2].ToolCalls[0].ID != old.ToolCallID {
		t.Error("tool_call/result pair was broken by compaction")
	}
}

func TestCompactHistory_DropsOldestTurnsKeepsRecentAndSystem(t *testing.T) {
	o := New(nil, &scriptedBrain{}, tools.NewRegistry(), "system", quietLogger())
	// Many turns whose size survives pass-1 shrinking: user text itself is
	// heavy (no tool result to shrink), so pass 2 must drop whole turns.
	heavy := strings.Repeat("habla larga ", 400) // ~4.8 KB per turn
	total := 12
	for n := 0; n < total; n++ {
		o.history = append(o.history,
			brain.Message{Role: brain.RoleUser, Content: heavy},
			brain.Message{Role: brain.RoleAssistant, Content: "ok"},
		)
	}

	o.compactHistory()

	if o.history[0].Role != brain.RoleSystem || o.history[0].Content != "system" {
		t.Fatal("system message must always survive compaction")
	}
	remaining := len(userIndices(o.history))
	if remaining < keepRecentUserTurns {
		t.Errorf("protected window violated: %d user turns left, want >= %d", remaining, keepRecentUserTurns)
	}
	if remaining == total {
		t.Error("expected pass 2 to drop oldest turns, none were dropped")
	}
	// No orphaned tool messages anywhere.
	for i, m := range o.history {
		if m.Role == brain.RoleTool {
			if i == 0 || len(o.history[i-1].ToolCalls) == 0 {
				t.Errorf("orphaned tool message at index %d", i)
			}
		}
	}
}

func TestCompactHistory_RecentWindowNeverShrunk(t *testing.T) {
	o := New(nil, &scriptedBrain{}, tools.NewRegistry(), "system", quietLogger())
	big := strings.Repeat("sección actual ", 3000) // recent turn over the limit by itself
	seedTurn(o, 0, big)

	o.compactHistory()

	if !strings.Contains(o.history[3].Content, "sección actual") || len(o.history[3].Content) < len(big) {
		t.Error("the most recent turn's tool result must never be shrunk (follow-ups need it)")
	}
}

func TestTruncateUTF8_NeverSplitsRune(t *testing.T) {
	s := strings.Repeat("á", 100) // 2 bytes each
	got := truncateUTF8(s, 7)     // 7 falls mid-rune
	if len(got) != 6 {
		t.Errorf("expected cut at rune boundary (6 bytes), got %d", len(got))
	}
	for _, r := range got {
		if r != 'á' {
			t.Errorf("corrupted rune %q in output", r)
		}
	}
}
