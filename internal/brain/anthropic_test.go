package brain

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropic_HappyPathWithCaching(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing/wrong x-api-key: %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Errorf("missing anthropic-version header")
		}

		var got antRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		// System prompt must be sent as a cacheable block.
		if len(got.System) != 1 {
			t.Fatalf("expected 1 system block, got %d", len(got.System))
		}
		if got.System[0].CacheControl == nil || got.System[0].CacheControl.Type != "ephemeral" {
			t.Error("expected system block marked cache_control=ephemeral")
		}

		// Last tool must carry cache_control so the whole tool block is cached.
		if len(got.Tools) == 0 {
			t.Fatal("expected at least one tool in request")
		}
		last := got.Tools[len(got.Tools)-1]
		if last.CacheControl == nil {
			t.Error("expected last tool to carry cache_control marker")
		}

		// The user role split must work: no role=system in messages.
		for _, m := range got.Messages {
			if m.Role == "system" {
				t.Error("system message leaked into messages array")
			}
		}

		_, _ = io.WriteString(w, `{
			"content":[
				{"type":"text","text":"OK"},
				{"type":"tool_use","id":"tu_1","name":"open_app","input":{"name":"Chrome"}}
			],
			"stop_reason":"tool_use",
			"usage":{
				"input_tokens":50,
				"output_tokens":12,
				"cache_creation_input_tokens":30,
				"cache_read_input_tokens":100
			}
		}`)
	}))
	defer srv.Close()

	b := NewAnthropic("test-key", "claude-opus-4-7")
	b.BaseURL = srv.URL

	resp, err := b.Chat(context.Background(),
		[]Message{
			{Role: RoleSystem, Content: "You are an assistant."},
			{Role: RoleUser, Content: "abre Chrome"},
		},
		[]ToolSpec{
			{Name: "noop", Description: "noop", Schema: map[string]any{"type": "object"}},
			{Name: "open_app", Description: "open", Schema: map[string]any{"type": "object"}},
		},
	)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "OK" {
		t.Errorf("expected text OK, got %q", resp.Text)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "open_app" {
		t.Errorf("expected open_app tool call, got %+v", resp.ToolCalls)
	}
	if !strings.Contains(resp.ToolCalls[0].Arguments, "Chrome") {
		t.Errorf("expected Chrome in args, got %q", resp.ToolCalls[0].Arguments)
	}
	if resp.Usage.CachedInputTokens != 100 {
		t.Errorf("expected cached input tokens = 100, got %d", resp.Usage.CachedInputTokens)
	}
}

func TestAnthropic_NoAPIKey(t *testing.T) {
	b := NewAnthropic("", "claude-haiku-4-5")
	_, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error when API key is empty")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("expected ANTHROPIC_API_KEY mention, got %v", err)
	}
}

func TestAnthropic_APIErrorIsSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"type":"invalid_request_error","message":"oops"}}`)
	}))
	defer srv.Close()

	b := NewAnthropic("test-key", "claude-haiku-4-5")
	b.BaseURL = srv.URL

	_, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "oops") {
		t.Errorf("expected oops in error, got %v", err)
	}
}

func TestAnthropic_ToolResultRoundTrip(t *testing.T) {
	// Verifies that a prior tool_use + tool result are converted to the
	// Anthropic message shape (user message with tool_result block).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got antRequest
		_ = json.NewDecoder(r.Body).Decode(&got)

		if len(got.Messages) < 3 {
			t.Fatalf("expected >=3 messages, got %d", len(got.Messages))
		}
		// Last message should be a user message with one tool_result block.
		last := got.Messages[len(got.Messages)-1]
		if last.Role != "user" {
			t.Errorf("expected last message role=user, got %s", last.Role)
		}
		if len(last.Content) != 1 || last.Content[0].Type != "tool_result" {
			t.Errorf("expected last message to be a tool_result, got %+v", last.Content)
		}
		if last.Content[0].ToolUseID != "tu_1" {
			t.Errorf("expected tool_use_id=tu_1, got %s", last.Content[0].ToolUseID)
		}

		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"done"}],"stop_reason":"end_turn","usage":{}}`)
	}))
	defer srv.Close()

	b := NewAnthropic("test-key", "claude-haiku-4-5")
	b.BaseURL = srv.URL

	_, err := b.Chat(context.Background(),
		[]Message{
			{Role: RoleUser, Content: "do it"},
			{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "tu_1", Name: "open_app", Arguments: `{"name":"Chrome"}`}}},
			{Role: RoleTool, ToolCallID: "tu_1", Name: "open_app", Content: "Opened Chrome."},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
}
