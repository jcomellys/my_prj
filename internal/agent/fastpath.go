// Package agent — fastpath.go: skip a brain round on simple, recognizable
// intents.
//
// Live measurement: a "léeme la sección X" turn spent ~2s of its ~5s budget
// just letting the brain pick read_open_pdf — a routing round-trip with no
// reasoning content. When the intent is unambiguous, the orchestrator runs
// the tool itself and splices the (synthetic) tool_call + tool_result into
// history. The next brain call becomes the final response: one round, not two.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/tools"
)

// sectionRequestRe matches the "léeme/lee la sección X" family. It tolerates
// the accent on "léeme" (STT can swallow it), the formal "léame", and the
// accent on "sección", and ignores a trailing punctuation mark. Captures the
// section name.
var sectionRequestRe = regexp.MustCompile(
	`(?i)\b(?:l[eé][eé]me|l[eé]ame|lee)\s+la\s+secci[oó]n\s+([^\.\!\?]+?)\s*[\.\!\?]?\s*$`,
)

// Natural speech wraps the section name in filler the PDF heading does not
// have: "la sección DE introducción", "…POR FAVOR", "…EN INGLÉS". Strip the
// known wrappers so the tool receives the bare heading. Trailing modifiers
// can stack ("…del pdf por favor"), so they are stripped in a loop.
var sectionLeadInRe = regexp.MustCompile(`(?i)^(?:de\s+la\s+|de\s+los\s+|de\s+las\s+|del\s+|de\s+)`)
var sectionTrailerRe = regexp.MustCompile(
	`(?i)(?:^|\s+)(?:por\s+favor|en\s+su\s+idioma\s+original|en\s+ingl[eé]s|en\s+espa[ñn]ol|del\s+(?:pdf|documento|archivo|libro)|de\s+la\s+p[aá]gina|del\s+texto)\s*$`,
)

// cleanSectionName strips the speech wrappers around a captured section name.
// Punctuation left behind by a removed trailer ("introducción, por favor" →
// "introducción,") is pruned each pass. Returns "" when nothing remains (the
// utterance was all filler).
func cleanSectionName(s string) string {
	s = strings.TrimSpace(s)
	for {
		t := strings.TrimRight(strings.TrimSpace(s), ",;:")
		t = sectionTrailerRe.ReplaceAllString(t, "")
		if t == s {
			break
		}
		s = t
	}
	s = sectionLeadInRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// tryFastPath inspects userText for a known intent. On a match it runs the
// matching tool in Go and appends a synthetic (assistant tool_call + tool
// result) pair to history, so the brain loop's next call is the response
// round. Returns true if a fast path fired.
//
// Fast paths must be conservative — a false positive would put a stale tool
// result in front of the model. Only patterns that uniquely identify a tool
// belong here.
func (o *Orchestrator) tryFastPath(ctx context.Context, userText string) bool {
	if m := sectionRequestRe.FindStringSubmatch(userText); m != nil {
		section := cleanSectionName(m[1])
		if section == "" {
			return false
		}
		return o.runFastPath(ctx, "read_open_pdf", map[string]any{"section": section})
	}
	return false
}

func (o *Orchestrator) runFastPath(ctx context.Context, name string, args map[string]any) bool {
	t, ok := o.Tools.Get(name)
	if !ok {
		return false
	}
	argsJSON, _ := json.Marshal(args)
	// Unique per call: some APIs (Anthropic) reject a history where two
	// tool_use blocks share an id, and a session can hit the same fast path
	// many times.
	o.fastpathSeq++
	id := fmt.Sprintf("fastpath-%s-%d", name, o.fastpathSeq)

	start := time.Now()
	res, err := t.Execute(ctx, string(argsJSON))
	durMs := time.Since(start).Milliseconds()

	if err != nil {
		// A barge-in (hotkey) or shutdown cancels the ctx mid-tool. That is a
		// human action, not a tool failure: log it as a clean cancellation,
		// leave history untouched, and let the aborted turn die upstream.
		if ctx.Err() != nil {
			o.Log.Info("fastpath.cancelled",
				"tool", name, "dur_ms", durMs, "args", string(argsJSON))
			return false
		}
		res = tools.Result{Text: "ERROR: " + err.Error()}
		o.Log.Warn("fastpath.error",
			"tool", name, "dur_ms", durMs, "args", string(argsJSON), "err", err)
	} else {
		o.Log.Info("fastpath.ok",
			"tool", name, "dur_ms", durMs, "args", string(argsJSON))
	}

	o.history = append(o.history, brain.Message{
		Role:      brain.RoleAssistant,
		ToolCalls: []brain.ToolCall{{ID: id, Name: name, Arguments: string(argsJSON)}},
	})
	o.history = append(o.history, brain.Message{
		Role:       brain.RoleTool,
		Name:       name,
		ToolCallID: id,
		Content:    res.Text,
		Images:     res.Images,
	})
	return true
}
