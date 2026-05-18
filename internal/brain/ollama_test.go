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

func TestOllama_HappyPathTextOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got olRequest
		_ = json.NewDecoder(r.Body).Decode(&got)
		if got.Model != "llama3.3:8b" {
			t.Errorf("expected llama3.3:8b, got %s", got.Model)
		}
		if got.Stream {
			t.Error("expected stream=false")
		}
		_, _ = io.WriteString(w, `{
			"message":{"role":"assistant","content":"hola"},
			"done":true,
			"prompt_eval_count":42,
			"eval_count":7
		}`)
	}))
	defer srv.Close()

	b := NewOllama(srv.URL, "llama3.3:8b")
	resp, err := b.Chat(context.Background(),
		[]Message{
			{Role: RoleSystem, Content: "sys"},
			{Role: RoleUser, Content: "hola"},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "hola" {
		t.Errorf("expected hola, got %q", resp.Text)
	}
	if resp.Usage.InputTokens != 42 || resp.Usage.OutputTokens != 7 {
		t.Errorf("usage decoded incorrectly: %+v", resp.Usage)
	}
	if resp.Usage.CachedInputTokens != 0 {
		t.Errorf("local inference has no cache; expected 0 cached tokens, got %d", resp.Usage.CachedInputTokens)
	}
}

func TestOllama_ToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got olRequest
		_ = json.NewDecoder(r.Body).Decode(&got)
		if len(got.Tools) != 1 || got.Tools[0].Function.Name != "open_app" {
			t.Errorf("expected open_app tool in request, got %+v", got.Tools)
		}
		_, _ = io.WriteString(w, `{
			"message":{
				"role":"assistant",
				"content":"",
				"tool_calls":[{"function":{"name":"open_app","arguments":{"name":"Chrome"}}}]
			},
			"done":true,
			"prompt_eval_count":10,
			"eval_count":3
		}`)
	}))
	defer srv.Close()

	b := NewOllama(srv.URL, "llama3.3:8b")
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
		t.Fatalf("expected open_app tool call, got %+v", resp.ToolCalls)
	}
	if !strings.Contains(resp.ToolCalls[0].Arguments, "Chrome") {
		t.Errorf("expected Chrome in args, got %q", resp.ToolCalls[0].Arguments)
	}
}

func TestOllama_ErrorSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"error":"model not found"}`)
	}))
	defer srv.Close()

	b := NewOllama(srv.URL, "ghost-model")
	_, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("expected error message surfaced, got %v", err)
	}
}

func TestOllama_DefaultBaseURL(t *testing.T) {
	b := NewOllama("", "llama3.3:8b")
	if b.BaseURL != "http://localhost:11434" {
		t.Errorf("expected default localhost:11434, got %q", b.BaseURL)
	}
}
