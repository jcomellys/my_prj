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

// TestOpenAI_ReasoningModelDropsTemperature covers the fase 0.1 bug on Mac
// where GPT-5 returned "Unsupported value: temperature does not support 0.3
// with this model. Only the default (1) value is supported."
func TestOpenAI_ReasoningModelDropsTemperature(t *testing.T) {
	var sentTemp float64
	var temperaturePresent bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode into a generic map so we can detect whether the field
		// was present at all in the JSON body.
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		if v, ok := raw["temperature"]; ok {
			temperaturePresent = true
			if f, ok := v.(float64); ok {
				sentTemp = f
			}
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{}}`)
	}))
	defer srv.Close()

	cases := []struct {
		model     string
		userTemp  float64
		wantSent  bool
		wantValue float64
	}{
		// GPT-5 with non-default temp: must NOT be sent.
		{"gpt-5", 0.3, false, 0},
		{"gpt-5-mini", 0.7, false, 0},
		// GPT-5 with default temp (1): may be sent or omitted; we accept omit.
		{"gpt-5", 1.0, true, 1.0},
		// o-series: same treatment.
		{"o3-mini", 0.5, false, 0},
		// Non-reasoning model (e.g., a hypothetical 4-class): honor user's value.
		{"gpt-4o", 0.3, true, 0.3},
	}
	for _, c := range cases {
		sentTemp = 0
		temperaturePresent = false

		b := NewOpenAI("k", c.model)
		b.BaseURL = srv.URL
		b.Temperature = c.userTemp

		if _, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil); err != nil {
			t.Fatalf("Chat(%s, %v): %v", c.model, c.userTemp, err)
		}
		if temperaturePresent != c.wantSent {
			t.Errorf("model=%s temp=%v: present=%v want %v", c.model, c.userTemp, temperaturePresent, c.wantSent)
		}
		if c.wantSent && sentTemp != c.wantValue {
			t.Errorf("model=%s: sent temp=%v want %v", c.model, sentTemp, c.wantValue)
		}
	}
}

// TestOpenAI_ReasoningEffort verifies reasoning_effort is sent for reasoning
// models (the big voice-latency lever) and omitted otherwise.
func TestOpenAI_ReasoningEffort(t *testing.T) {
	var got string
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		if v, ok := raw["reasoning_effort"]; ok {
			present = true
			got, _ = v.(string)
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{}}`)
	}))
	defer srv.Close()

	cases := []struct {
		model       string
		effort      string
		wantPresent bool
		wantValue   string
	}{
		{"gpt-5", "low", true, "low"},
		{"gpt-5-mini", "minimal", true, "minimal"},
		{"o3-mini", "high", true, "high"},
		{"gpt-5", "", false, ""},      // empty => omit (API default)
		{"gpt-4o", "low", false, ""},  // non-reasoning model => never sent
	}
	for _, c := range cases {
		present, got = false, ""
		b := NewOpenAI("k", c.model)
		b.BaseURL = srv.URL
		b.ReasoningEffort = c.effort
		if _, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil); err != nil {
			t.Fatalf("Chat(%s,%s): %v", c.model, c.effort, err)
		}
		if present != c.wantPresent {
			t.Errorf("model=%s effort=%q: present=%v want %v", c.model, c.effort, present, c.wantPresent)
		}
		if c.wantPresent && got != c.wantValue {
			t.Errorf("model=%s: sent effort=%q want %q", c.model, got, c.wantValue)
		}
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
