package brain

import (
	"context"
	"strings"
)

// Mock is a deterministic Brain for unit tests and offline dev.
//
// Behavior:
//   - If the last message is a tool result, return a short text reply
//     that closes the round (so the orchestrator does not loop calling
//     the same tool again).
//   - Else, if the most recent user message starts with "abre"/"abrir"/
//     "open" <app>, request the open_app tool.
//   - Otherwise, echo the user.
type Mock struct{}

func NewMock() *Mock { return &Mock{} }

func (Mock) Name() string { return "mock" }

func (Mock) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (*Response, error) {
	// If we just got back a tool result, summarize and end. This prevents
	// the test brain from re-issuing the same tool call forever, which the
	// orchestrator would otherwise treat as a runaway loop.
	if len(messages) > 0 && messages[len(messages)-1].Role == RoleTool {
		return &Response{Text: strings.TrimSpace(messages[len(messages)-1].Content)}, nil
	}

	var lastUser string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			lastUser = messages[i].Content
			break
		}
	}
	lower := strings.ToLower(lastUser)

	for _, prefix := range []string{"abre ", "abrir ", "open "} {
		if strings.HasPrefix(lower, prefix) {
			app := strings.TrimSpace(lastUser[len(prefix):])
			if app == "" {
				continue
			}
			return &Response{
				Text: "",
				ToolCalls: []ToolCall{{
					ID:        "mock-call-1",
					Name:      "open_app",
					Arguments: `{"name":"` + app + `"}`,
				}},
			}, nil
		}
	}

	return &Response{Text: "Mock brain: " + lastUser}, nil
}
