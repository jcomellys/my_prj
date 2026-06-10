// Package agent — history.go: keep the conversation prompt bounded.
//
// Every turn re-sends the whole history to the brain. Tool payloads dominate
// that size — a full read_pdf result can be 200 KiB (~50k tokens) and it gets
// re-billed on EVERY later turn of the session. For a voice product where
// token cost and round-trip latency are the product priorities, an unbounded
// history silently degrades both as a session ages.
//
// Compaction is deterministic (no extra LLM call, no extra cost, testable in
// CI) and runs in two passes before each new user turn:
//
//  1. Tool results older than the protected recent window are cut to a short
//     head and old images are dropped. Their value decays once the brain has
//     already spoken about them; the head keeps enough context to know what
//     happened.
//  2. If the history is still over budget, whole turns are dropped oldest
//     first, always at user-message boundaries so an assistant tool_call and
//     its tool result are never separated (providers reject orphaned pairs).
//
// The system message and the most recent turns are never touched, so "continúa
// leyendo" and other short-range follow-ups keep their full context.
package agent

import (
	"unicode/utf8"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
)

const (
	// historySoftLimitBytes triggers compaction (~6k tokens of prompt). Soft:
	// the protected recent window is kept even if it alone exceeds the limit.
	historySoftLimitBytes = 24 * 1024
	// keepRecentUserTurns is the protected window: this many most-recent user
	// turns are never compacted or dropped.
	keepRecentUserTurns = 4
	// Old tool results longer than compactToolResultMin are cut to a
	// compactToolResultKeep-byte head plus a marker.
	compactToolResultMin  = 600
	compactToolResultKeep = 300
)

// compactedMarker is appended to a shrunken tool result so the model knows the
// payload was elided rather than truly short.
const compactedMarker = "\n…(resultado antiguo recortado para ahorrar contexto)"

// compactHistory bounds o.history before a new user turn is appended.
func (o *Orchestrator) compactHistory() {
	before := historyBytes(o.history)
	if before <= historySoftLimitBytes {
		return
	}

	userIdx := userIndices(o.history)
	if len(userIdx) == 0 {
		return
	}

	// Pass 1: shrink heavy payloads older than the protected window.
	protectFrom := userIdx[0]
	if len(userIdx) > keepRecentUserTurns {
		protectFrom = userIdx[len(userIdx)-keepRecentUserTurns]
	}
	for i := 1; i < protectFrom; i++ { // 0 is the system message
		m := &o.history[i]
		m.Images = nil
		if m.Role == brain.RoleTool && len(m.Content) > compactToolResultMin {
			m.Content = truncateUTF8(m.Content, compactToolResultKeep) + compactedMarker
		}
	}

	// Pass 2: drop whole oldest turns while still over budget, preserving the
	// protected window. A turn block is [user_i, user_i+1) — tool_call/result
	// pairs live inside a block, so dropping at these boundaries never orphans
	// a pair.
	for historyBytes(o.history) > historySoftLimitBytes {
		userIdx = userIndices(o.history)
		if len(userIdx) <= keepRecentUserTurns {
			break
		}
		o.history = append(o.history[:userIdx[0]], o.history[userIdx[1]:]...)
	}

	if after := historyBytes(o.history); after < before {
		o.Log.Info("history.compact",
			"before_bytes", before, "after_bytes", after, "msgs", len(o.history))
	}
}

func userIndices(h []brain.Message) []int {
	var idx []int
	for i, m := range h {
		if m.Role == brain.RoleUser {
			idx = append(idx, i)
		}
	}
	return idx
}

// historyBytes estimates the prompt weight of a history. Images count their
// raw size — they dominate when present, and providers bill them heavily.
func historyBytes(h []brain.Message) int {
	n := 0
	for _, m := range h {
		n += len(m.Content)
		for _, tc := range m.ToolCalls {
			n += len(tc.Arguments) + len(tc.Name)
		}
		for _, img := range m.Images {
			n += len(img.Data)
		}
	}
	return n
}

// truncateUTF8 cuts s to at most max bytes without splitting a multibyte rune
// (Spanish text is full of them; an invalid tail confuses providers).
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
