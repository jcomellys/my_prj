package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
)

// systemRoots are paths a write must never touch, even with "amplio" power.
// Reading is unrestricted; writing is blocked here to avoid the sub-agent
// corrupting the OS. The user's own files/projects remain writable.
var systemRoots = []string{"/System", "/usr", "/bin", "/sbin", "/Library", "/private/etc", "/etc"}

// ReadFile reads a UTF-8 text file and returns its content (bounded).
type ReadFile struct {
	MaxBytes int64 // default 200 KiB
}

func NewReadFile() *ReadFile { return &ReadFile{MaxBytes: 200 * 1024} }

func (ReadFile) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name:        "read_file",
		Description: "Read a text file from disk and return its contents. Use absolute paths or paths under the user's home. For very large files only the first part is returned.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Absolute or ~-relative file path."},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
	}
}

func (t *ReadFile) Execute(_ context.Context, argsJSON string) (Result, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := UnmarshalArgs(argsJSON, &args); err != nil {
		return Result{}, err
	}
	p, err := safePath(args.Path)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return Result{}, fmt.Errorf("read_file: %w", err)
	}
	if info.IsDir() {
		return Result{}, fmt.Errorf("read_file: %s is a directory", p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Result{}, fmt.Errorf("read_file: %w", err)
	}
	max := t.MaxBytes
	if max <= 0 {
		max = 200 * 1024
	}
	if int64(len(data)) > max {
		return Result{Text: string(data[:max]) + "\n…(truncated)"}, nil
	}
	return Result{Text: string(data)}, nil
}

// WriteFile creates or overwrites a text file. Refuses system paths.
type WriteFile struct{}

func NewWriteFile() *WriteFile { return &WriteFile{} }

func (WriteFile) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name:        "write_file",
		Description: "Create or overwrite a text file with the given content. Use for generating documents, notes, or code. Cannot write to macOS system directories. Creates parent folders as needed.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "Absolute or ~-relative destination path."},
				"content": map[string]any{"type": "string", "description": "Full file content to write."},
			},
			"required":             []string{"path", "content"},
			"additionalProperties": false,
		},
	}
}

func (t *WriteFile) Execute(_ context.Context, argsJSON string) (Result, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := UnmarshalArgs(argsJSON, &args); err != nil {
		return Result{}, err
	}
	p, err := safePath(args.Path)
	if err != nil {
		return Result{}, err
	}
	if isSystemPath(p) {
		return Result{}, fmt.Errorf("write_file: refusing to write to a system path: %s", p)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return Result{}, fmt.Errorf("write_file: %w", err)
	}
	if err := os.WriteFile(p, []byte(args.Content), 0o644); err != nil {
		return Result{}, fmt.Errorf("write_file: %w", err)
	}
	return Textf("Wrote %d bytes to %s.", len(args.Content), p), nil
}

// safePath expands ~ and rejects path traversal. Returns a cleaned absolute
// path. Reading/writing relative to traversal (..) is rejected outright.
func safePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("path traversal (..) not allowed: %s", p)
	}
	return filepath.Clean(p), nil
}

func isSystemPath(p string) bool {
	for _, root := range systemRoots {
		if p == root || strings.HasPrefix(p, root+"/") {
			return true
		}
	}
	return false
}
