package brain

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Anthropic is a Brain backed by Claude.
//
// Two cost optimizations are wired in from day 1:
//
//  1. Prompt caching: the system prompt AND the tool definitions are sent
//     with cache_control. Anthropic charges 10% of the input price for
//     cached tokens; after the first call, every subsequent call within
//     the cache TTL pays a fraction of the original.
//  2. Usage tracking exposes cache_read / cache_creation tokens so cost
//     reports show the savings honestly.
//
// We hit the Messages API directly over HTTP — no SDK — to keep the
// portable binary small and the deps auditable.
type Anthropic struct {
	APIKey      string
	Model       string // e.g. "claude-opus-4-7", "claude-haiku-4-5", "claude-sonnet-4-6"
	BaseURL     string // default https://api.anthropic.com
	Version     string // default "2023-06-01"
	Temperature float64
	MaxTokens   int
	HTTPClient  *http.Client
}

func NewAnthropic(apiKey, model string) *Anthropic {
	return &Anthropic{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    "https://api.anthropic.com",
		Version:    "2023-06-01",
		MaxTokens:  4096,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *Anthropic) Name() string { return "anthropic:" + a.Model }

// --- wire types -------------------------------------------------------------

type antCacheControl struct {
	Type string `json:"type"`
}

type antTool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"input_schema"`
	CacheControl *antCacheControl `json:"cache_control,omitempty"`
}

type antContentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	// Content can be either a plain string OR an array of inner blocks
	// (e.g., for tool_result that includes an image). We use any to let
	// json/Marshal pick the right shape.
	Content any           `json:"content,omitempty"`
	Source  *antImageSrc  `json:"source,omitempty"`
}

type antImageSrc struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/png" / "image/jpeg"
	Data      string `json:"data"`       // base64-encoded
}

type antMessage struct {
	Role    string            `json:"role"`
	Content []antContentBlock `json:"content"`
}

type antSystemBlock struct {
	Type         string           `json:"type"`
	Text         string           `json:"text"`
	CacheControl *antCacheControl `json:"cache_control,omitempty"`
}

type antRequest struct {
	Model       string           `json:"model"`
	System      []antSystemBlock `json:"system,omitempty"`
	Messages    []antMessage     `json:"messages"`
	Tools       []antTool        `json:"tools,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens"`
}

type antResponse struct {
	Content    []antContentBlock `json:"content"`
	StopReason string            `json:"stop_reason"`
	Usage      struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ----------------------------------------------------------------------------

func (a *Anthropic) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (*Response, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	system, conv := splitSystem(messages)

	req := antRequest{
		Model:       a.Model,
		System:      systemBlocks(system),
		Messages:    toAntMessages(conv),
		Tools:       toAntTools(tools),
		Temperature: a.Temperature,
		MaxTokens:   a.MaxTokens,
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}

	buf, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.APIKey)
	httpReq.Header.Set("anthropic-version", a.Version)

	resp, err := a.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out antResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w (body=%s)", err, truncate(string(raw), 400))
	}
	if out.Error != nil {
		return nil, fmt.Errorf("anthropic: %s (%s)", out.Error.Message, out.Error.Type)
	}

	r := &Response{
		Usage: Usage{
			InputTokens:       out.Usage.InputTokens,
			OutputTokens:      out.Usage.OutputTokens,
			CachedInputTokens: out.Usage.CacheReadInputTokens,
		},
	}
	for _, blk := range out.Content {
		switch blk.Type {
		case "text":
			r.Text += blk.Text
		case "tool_use":
			args, _ := json.Marshal(blk.Input)
			r.ToolCalls = append(r.ToolCalls, ToolCall{
				ID:        blk.ID,
				Name:      blk.Name,
				Arguments: string(args),
			})
		}
	}
	return r, nil
}

// splitSystem extracts leading system messages (concatenated) from a Message
// slice and returns the rest. Anthropic puts the system prompt in a separate
// top-level field, not as a "role: system" message.
func splitSystem(in []Message) (system string, conv []Message) {
	for _, m := range in {
		if m.Role == RoleSystem {
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
			continue
		}
		conv = append(conv, m)
	}
	return system, conv
}

func systemBlocks(system string) []antSystemBlock {
	if system == "" {
		return nil
	}
	// Mark the system prompt as cacheable. After the first call, repeats
	// of this exact prefix are charged at 10% of the input price.
	return []antSystemBlock{{
		Type:         "text",
		Text:         system,
		CacheControl: &antCacheControl{Type: "ephemeral"},
	}}
}

func toAntTools(in []ToolSpec) []antTool {
	if len(in) == 0 {
		return nil
	}
	out := make([]antTool, 0, len(in))
	// Mark the LAST tool with cache_control so the whole tool block is
	// cached as one prefix. Anthropic caches everything up to and including
	// the marked element.
	for i, t := range in {
		at := antTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Schema,
		}
		if i == len(in)-1 {
			at.CacheControl = &antCacheControl{Type: "ephemeral"}
		}
		out = append(out, at)
	}
	return out
}

func toAntMessages(in []Message) []antMessage {
	out := make([]antMessage, 0, len(in))
	for _, m := range in {
		switch m.Role {
		case RoleUser:
			blocks := []antContentBlock{}
			if m.Content != "" {
				blocks = append(blocks, antContentBlock{Type: "text", Text: m.Content})
			}
			for _, img := range m.Images {
				blocks = append(blocks, imageBlock(img))
			}
			out = append(out, antMessage{Role: "user", Content: blocks})
		case RoleAssistant:
			blocks := []antContentBlock{}
			if m.Content != "" {
				blocks = append(blocks, antContentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var input map[string]any
				_ = json.Unmarshal([]byte(tc.Arguments), &input)
				blocks = append(blocks, antContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			out = append(out, antMessage{Role: "assistant", Content: blocks})
		case RoleTool:
			// Anthropic represents tool results as a user message with a
			// tool_result content block referencing the prior tool_use id.
			// If the tool attached images, the tool_result's content
			// becomes an array of inner blocks (text + image).
			tr := antContentBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
			}
			if len(m.Images) > 0 {
				inner := []antContentBlock{}
				if m.Content != "" {
					inner = append(inner, antContentBlock{Type: "text", Text: m.Content})
				}
				for _, img := range m.Images {
					inner = append(inner, imageBlock(img))
				}
				tr.Content = inner
			} else {
				tr.Content = m.Content
			}
			out = append(out, antMessage{Role: "user", Content: []antContentBlock{tr}})
		}
	}
	return out
}

func imageBlock(img ImageBlob) antContentBlock {
	mt := img.MediaType
	if mt == "" {
		mt = "image/png"
	}
	return antContentBlock{
		Type: "image",
		Source: &antImageSrc{
			Type:      "base64",
			MediaType: mt,
			Data:      base64.StdEncoding.EncodeToString(img.Data),
		},
	}
}
