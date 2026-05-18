// Package tools wires concrete actions the agent can perform.
//
// Each Tool is a small adapter around an OSAdapter call (or a Brain delegation,
// browser action, etc., in later phases). The Tool is responsible for:
//
//   - Declaring its JSON-Schema for the Brain.
//   - Validating arguments before execution.
//   - Enforcing safety (e.g., command allowlists).
//   - Returning a short, model-friendly result string.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
)

// Tool is the concrete operation interface.
type Tool interface {
	// Spec returns the schema the Brain uses to call this tool.
	Spec() brain.ToolSpec

	// Execute runs the tool with raw JSON arguments and returns a short
	// result string suitable for feeding back to the Brain.
	Execute(ctx context.Context, argsJSON string) (string, error)
}

// Registry holds the set of tools available to the agent.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Spec().Name] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Specs() []brain.ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]brain.ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Spec())
	}
	return out
}

// UnmarshalArgs is a helper for Tool.Execute implementations.
func UnmarshalArgs(argsJSON string, dst any) error {
	if argsJSON == "" {
		argsJSON = "{}"
	}
	if err := json.Unmarshal([]byte(argsJSON), dst); err != nil {
		return fmt.Errorf("invalid tool arguments: %w", err)
	}
	return nil
}
