package brain

import (
	"context"
	"strings"
)

// Mock is a deterministic Brain for unit tests and offline dev.
// It echoes the last user message and optionally invokes a hard-coded tool
// when the user says "abre <app>".
type Mock struct{}

func NewMock() *Mock { return &Mock{} }

func (Mock) Name() string { return "mock" }

func (Mock) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (*Response, error) {
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
