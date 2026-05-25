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

// Ollama is a Brain backed by a locally-running Ollama server, which serves
// the free tier of this product. On a Mac mini M4, Llama 3.3 8B or Qwen 2.5
// 7B runs at usable speeds (30-50 tok/s).
//
// Tool calling support varies by model. Llama 3.3, Qwen 2.5, and Mistral
// Small are known to handle tool calls well. The Ollama Chat API returns
// tool calls in a structured field; we map them onto our Brain interface.
//
// Important: usage data from Ollama does not include cached tokens; the
// CachedInputTokens field will always be 0 for this provider. That is
// correct, not a bug — local inference has no cache concept.
type Ollama struct {
	BaseURL     string // default "http://localhost:11434"
	Model       string // e.g. "llama3.3:8b", "qwen2.5:14b"
	Temperature float64
	NumPredict  int // Ollama's equivalent of max_tokens; 0 = model default
	HTTPClient  *http.Client
}

func NewOllama(baseURL, model string) *Ollama {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Ollama{
		BaseURL:    baseURL,
		Model:      model,
		HTTPClient: &http.Client{Timeout: 300 * time.Second}, // local inference can be slow
	}
}

func (o *Ollama) Name() string { return "ollama:" + o.Model }

// --- wire types -------------------------------------------------------------

type olMessage struct {
	Role      string       `json:"role"`
	Content   string       `json:"content,omitempty"`
	ToolCalls []olToolCall `json:"tool_calls,omitempty"`
	ToolName  string       `json:"tool_name,omitempty"`
}

type olToolCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

type olTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type olRequest struct {
	Model    string         `json:"model"`
	Messages []olMessage    `json:"messages"`
	Tools    []olTool       `json:"tools,omitempty"`
	Stream   bool           `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type olResponse struct {
	Message         olMessage `json:"message"`
	Done            bool      `json:"done"`
	PromptEvalCount int       `json:"prompt_eval_count"`
	EvalCount       int       `json:"eval_count"`
	Error           string    `json:"error,omitempty"`
}

// ----------------------------------------------------------------------------

func (o *Ollama) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (*Response, error) {
	req := olRequest{
		Model:    o.Model,
		Messages: toOlMessages(messages),
		Tools:    toOlTools(tools),
		Stream:   false,
	}
	if o.Temperature != 0 || o.NumPredict != 0 {
		req.Options = map[string]any{}
		if o.Temperature != 0 {
			req.Options["temperature"] = o.Temperature
		}
		if o.NumPredict != 0 {
			req.Options["num_predict"] = o.NumPredict
		}
	}

	buf, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/api/chat", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request (is `ollama serve` running?): %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out olResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w (body=%s)", err, truncate(string(raw), 400))
	}
	if out.Error != "" {
		return nil, fmt.Errorf("ollama: %s", out.Error)
	}

	r := &Response{
		Text: out.Message.Content,
		Usage: Usage{
			InputTokens:  out.PromptEvalCount,
			OutputTokens: out.EvalCount,
			// Local inference: no cache concept; leave CachedInputTokens=0.
		},
	}
	for i, tc := range out.Message.ToolCalls {
		args, _ := json.Marshal(tc.Function.Arguments)
		r.ToolCalls = append(r.ToolCalls, ToolCall{
			ID:        fmt.Sprintf("ol_call_%d", i),
			Name:      tc.Function.Name,
			Arguments: string(args),
		})
	}
	return r, nil
}

func toOlMessages(in []Message) []olMessage {
	out := make([]olMessage, 0, len(in))
	for _, m := range in {
		switch m.Role {
		case RoleSystem:
			out = append(out, olMessage{Role: "system", Content: m.Content})
		case RoleUser:
			out = append(out, olMessage{Role: "user", Content: m.Content})
		case RoleAssistant:
			om := olMessage{Role: "assistant", Content: m.Content}
			for _, tc := range m.ToolCalls {
				var args map[string]any
				_ = json.Unmarshal([]byte(tc.Arguments), &args)
				om.ToolCalls = append(om.ToolCalls, olToolCall{
					Function: struct {
						Name      string         `json:"name"`
						Arguments map[string]any `json:"arguments"`
					}{Name: tc.Name, Arguments: args},
				})
			}
			out = append(out, om)
		case RoleTool:
			out = append(out, olMessage{
				Role:     "tool",
				Content:  m.Content,
				ToolName: m.Name,
			})
		}
	}
	return out
}

func toOlTools(in []ToolSpec) []olTool {
	out := make([]olTool, 0, len(in))
	for _, t := range in {
		ot := olTool{Type: "function"}
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		ot.Function.Parameters = t.Schema
		out = append(out, ot)
	}
	return out
}
