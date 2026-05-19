package brain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestOpenAI_AttachesImageInToolResult verifies that an ImageBlob on a
// RoleTool message is sent as a structured content array with an
// image_url data URL — the shape OpenAI's Chat Completions API expects
// for vision input.
func TestOpenAI_AttachesImageInToolResult(t *testing.T) {
	wantBytes := []byte{0x89, 0x50, 0x4E, 0x47, 1, 2, 3, 4, 5}
	gotURL := ""

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		msgs, _ := raw["messages"].([]any)
		for _, m := range msgs {
			mm, _ := m.(map[string]any)
			if mm["role"] != "tool" {
				continue
			}
			parts, ok := mm["content"].([]any)
			if !ok {
				continue
			}
			for _, p := range parts {
				pm, _ := p.(map[string]any)
				if pm["type"] == "image_url" {
					iu, _ := pm["image_url"].(map[string]any)
					gotURL, _ = iu["url"].(string)
				}
			}
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{}}`)
	}))
	defer srv.Close()

	b := NewOpenAI("k", "gpt-5")
	b.BaseURL = srv.URL

	msgs := []Message{
		{Role: RoleUser, Content: "qué hay en pantalla"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "screenshot", Arguments: "{}"}}},
		{Role: RoleTool, ToolCallID: "c1", Name: "screenshot",
			Content: "Captured.",
			Images:  []ImageBlob{{MediaType: "image/png", Data: wantBytes}}},
	}
	if _, err := b.Chat(context.Background(), msgs, nil); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	wantB64 := base64.StdEncoding.EncodeToString(wantBytes)
	if !strings.HasPrefix(gotURL, "data:image/png;base64,") {
		t.Fatalf("expected data URL prefix, got %q", gotURL)
	}
	if !strings.HasSuffix(gotURL, wantB64) {
		t.Errorf("base64 payload mismatch")
	}
}

// TestAnthropic_AttachesImageInToolResult verifies the equivalent shape
// for Anthropic: tool_result with a nested image block carrying base64.
func TestAnthropic_AttachesImageInToolResult(t *testing.T) {
	wantBytes := []byte{0x89, 0x50, 0x4E, 0x47, 9, 8, 7, 6}
	foundImage := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		msgs, _ := raw["messages"].([]any)
		for _, m := range msgs {
			mm, _ := m.(map[string]any)
			content, ok := mm["content"].([]any)
			if !ok {
				continue
			}
			for _, blk := range content {
				bm, _ := blk.(map[string]any)
				if bm["type"] != "tool_result" {
					continue
				}
				inner, ok := bm["content"].([]any)
				if !ok {
					continue
				}
				for _, inblk := range inner {
					inm, _ := inblk.(map[string]any)
					if inm["type"] == "image" {
						foundImage = true
						src, _ := inm["source"].(map[string]any)
						if src["type"] != "base64" {
							t.Errorf("expected source.type=base64, got %v", src["type"])
						}
						if src["media_type"] != "image/png" {
							t.Errorf("expected media_type=image/png, got %v", src["media_type"])
						}
						if got, _ := src["data"].(string); got != base64.StdEncoding.EncodeToString(wantBytes) {
							t.Errorf("base64 payload mismatch")
						}
					}
				}
			}
		}
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{}}`)
	}))
	defer srv.Close()

	b := NewAnthropic("k", "claude-opus-4-7")
	b.BaseURL = srv.URL

	msgs := []Message{
		{Role: RoleUser, Content: "qué hay en pantalla"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "tu_1", Name: "screenshot", Arguments: "{}"}}},
		{Role: RoleTool, ToolCallID: "tu_1", Name: "screenshot",
			Content: "Captured.",
			Images:  []ImageBlob{{MediaType: "image/png", Data: wantBytes}}},
	}
	if _, err := b.Chat(context.Background(), msgs, nil); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !foundImage {
		t.Error("did not find an image block inside the tool_result")
	}
}
