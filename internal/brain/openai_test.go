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

// TestOpenAI_HappyPath sends one tool-using assistant turn through a fake
// OpenAI server and verifies the response is decoded correctly.
func TestOpenAI_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("missing/incorrect Authorization header: %q", got)
		}

		var got oaiRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got.Model != "gpt-5" {
			t.Errorf("expected model gpt-5, got %s", got.Model)
		}
		if len(got.Tools) != 1 || got.Tools[0].Function.Name != "open_app" {
			t.Errorf("expected single open_app tool, got %+v", got.Tools)
		}

		_, _ = io.WriteString(w, `{
			"choices":[{
				"message":{
					"role":"assistant",
					"content":"",
					"tool_calls":[{
						"id":"call_1",
						"type":"function",
						"function":{"name":"open_app","arguments":"{\"name\":\"Chrome\"}"}
					}]
				},
				"finish_reason":"tool_calls"
			}],
			"usage":{
				"prompt_tokens":120,
				"completion_tokens":15,
				"prompt_tokens_details":{"cached_tokens":80}
			}
		}`)
	}))
	defer srv.Close()

	b := NewOpenAI("test-key", "gpt-5")
	b.BaseURL = srv.URL

	resp, err := b.Chat(context.Background(),
		[]Message{{Role: RoleUser, Content: "abre Chrome"}},
		[]ToolSpec{{
			Name:        "open_app",
			Description: "open an app",
			Schema:      map[string]any{"type": "object"},
		}},
	)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "open_app" {
		t.Fatalf("expected one open_app tool call, got %+v", resp.ToolCalls)
	}
	if !strings.Contains(resp.ToolCalls[0].Arguments, "Chrome") {
		t.Errorf("expected Chrome in args, got %q", resp.ToolCalls[0].Arguments)
	}
	if resp.Usage.InputTokens != 120 || resp.Usage.OutputTokens != 15 || resp.Usage.CachedInputTokens != 80 {
		t.Errorf("usage decoded incorrectly: %+v", resp.Usage)
	}
}

func TestOpenAI_NoAPIKey(t *testing.T) {
	b := NewOpenAI("", "gpt-5")
	_, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error when API key is empty")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Errorf("expected error to mention OPENAI_API_KEY, got %v", err)
	}
}

func TestOpenAI_APIErrorIsSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"bad request","type":"invalid_request_error"}}`)
	}))
	defer srv.Close()

	b := NewOpenAI("test-key", "gpt-5")
	b.BaseURL = srv.URL

	_, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected an error from API error response")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected error to surface API message, got %v", err)
	}
}
