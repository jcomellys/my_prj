package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAI is a Brain backed by the OpenAI Chat Completions API.
//
// We use raw HTTP rather than a vendored SDK to keep the portable binary
// small and the dependency surface auditable. The API shape is stable.
type OpenAI struct {
	APIKey      string
	Model       string // e.g. "gpt-5", "gpt-5-mini"
	BaseURL     string // default https://api.openai.com/v1
	Temperature float64
	MaxTokens   int
	HTTPClient  *http.Client
}

func NewOpenAI(apiKey, model string) *OpenAI {
	return &OpenAI{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    "https://api.openai.com/v1",
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OpenAI) Name() string { return "openai:" + o.Model }

// --- wire types -------------------------------------------------------------

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiToolFunc `json:"function"`
}

type oaiToolFunc struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	Name       string        `json:"name,omitempty"`
}

type oaiRequest struct {
	Model       string       `json:"model"`
	Messages    []oaiMessage `json:"messages"`
	Tools       []oaiTool    `json:"tools,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_completion_tokens,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message oaiMessage `json:"message"`
		Finish  string     `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ----------------------------------------------------------------------------

func (o *OpenAI) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (*Response, error) {
	if o.APIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	body := oaiRequest{
		Model:       o.Model,
		Messages:    toOAIMessages(messages),
		Tools:       toOAITools(tools),
		Temperature: o.Temperature,
		MaxTokens:   o.MaxTokens,
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	resp, err := o.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out oaiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w (body=%s)", err, truncate(string(raw), 400))
	}
	if out.Error != nil {
		return nil, fmt.Errorf("openai: %s (%s)", out.Error.Message, out.Error.Type)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("openai: no choices in response (body=%s)", truncate(string(raw), 400))
	}

	msg := out.Choices[0].Message
	r := &Response{
		Text: msg.Content,
		Usage: Usage{
			InputTokens:       out.Usage.PromptTokens,
			OutputTokens:      out.Usage.CompletionTokens,
			CachedInputTokens: out.Usage.PromptTokensDetails.CachedTokens,
		},
	}
	for _, tc := range msg.ToolCalls {
		r.ToolCalls = append(r.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return r, nil
}

func toOAIMessages(in []Message) []oaiMessage {
	out := make([]oaiMessage, 0, len(in))
	for _, m := range in {
		om := oaiMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, oaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
		out = append(out, om)
	}
	return out
}

func toOAITools(in []ToolSpec) []oaiTool {
	out := make([]oaiTool, 0, len(in))
	for _, t := range in {
		out = append(out, oaiTool{
			Type: "function",
			Function: oaiToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Schema,
			},
		})
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
