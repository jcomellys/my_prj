// Package brain defines the LLM provider interface and shared types.
//
// A Brain takes the current conversation and a set of tools, and returns
// either a text reply, one or more tool calls to execute, or both.
// Implementations live in sibling files (openai.go, ollama.go, ...).
package brain

import "context"

// Role identifies who produced a Message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one turn of the conversation.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`  // assistant turns may have these
	ToolCallID string     `json:"tool_call_id,omitempty"` // for role=tool, the id this answers
	Name       string     `json:"name,omitempty"`         // for role=tool, the tool name
}

// ToolSpec describes a tool the model may call. Schema is a JSON Schema object.
type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

// ToolCall is the model's request to execute a tool.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // raw JSON string, validated by the tool
}

// Response is what a Brain returns from Chat.
type Response struct {
	Text      string     // assistant text (may be empty if only tool calls)
	ToolCalls []ToolCall // tools the model wants executed
	Usage     Usage      // token accounting for cost tracking
}

// Usage records token counts for cost tracking.
type Usage struct {
	InputTokens       int
	OutputTokens      int
	CachedInputTokens int
}

// Brain is the LLM-agnostic interface.
//
// Implementations must be safe for concurrent use by one orchestrator.
type Brain interface {
	// Name returns a short identifier like "openai:gpt-5" for logs.
	Name() string

	// Chat sends the conversation and returns the model's response.
	// The system prompt should be the first Message with RoleSystem; the
	// implementation is responsible for caching it where supported.
	Chat(ctx context.Context, messages []Message, tools []ToolSpec) (*Response, error)
}
