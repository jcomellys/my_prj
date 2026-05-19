package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
)

// Screenshot captures the user's screen and attaches the image to the
// tool result so the Brain can "see" it on the next round. This is what
// enables a blind user to ask "léeme lo que dice esta ventana" and get a
// useful answer — GPT-5 / Claude both accept images.
//
// On macOS the capture is done with the native `screencapture` command,
// which is silent (-x) and handles Retina scaling correctly. The Mac
// will prompt for Screen Recording permission the first time; the user
// must approve in System Settings → Privacy & Security → Screen Recording.
type Screenshot struct {
	// CmdName overrides the binary path (mainly for tests).
	CmdName string

	// TempDir is where the captured PNG is written before being read back.
	TempDir string

	// MaxBytes is a safety cap. Screenshots above this are rejected to
	// avoid sending huge images to the brain. Default 8 MiB.
	MaxBytes int64
}

func NewScreenshot() *Screenshot {
	return &Screenshot{MaxBytes: 8 * 1024 * 1024}
}

func (Screenshot) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name: "screenshot",
		Description: "Capture the user's current screen as an image and attach it for analysis. " +
			"Use this whenever the user asks about visible content: 'qué hay en pantalla', " +
			"'léeme esta ventana', 'describe la imagen', 'qué dice este botón', 'analiza el gráfico', " +
			"or when an action requires reading the screen first. After this tool runs, the image is " +
			"available to you in the same conversation.",
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
	}
}

func (s *Screenshot) Execute(ctx context.Context, _ string) (Result, error) {
	dir := s.TempDir
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "agent-shot-*.png")
	if err != nil {
		return Result{}, fmt.Errorf("temp png: %w", err)
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)

	bin := s.CmdName
	if bin == "" {
		bin = "screencapture"
	}

	// -x = silent (no shutter sound), -t png = PNG output, path is the
	// file. We let macOS capture the whole main display; per-region
	// capture is a future tool. On non-macOS this binary won't exist and
	// the error message will say so.
	cmd := exec.CommandContext(ctx, bin, "-x", "-t", "png", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return Result{}, fmt.Errorf("screencapture: %w (output=%s, hint: grant Screen Recording permission to Terminal in System Settings → Privacy & Security)", err, string(out))
	}

	info, err := os.Stat(path)
	if err != nil {
		return Result{}, fmt.Errorf("stat capture: %w", err)
	}
	max := s.MaxBytes
	if max == 0 {
		max = 8 * 1024 * 1024
	}
	if info.Size() > max {
		return Result{}, fmt.Errorf("screenshot too large (%d bytes, max %d)", info.Size(), max)
	}
	if info.Size() < 100 {
		return Result{}, fmt.Errorf("screencapture produced an empty file at %s — permission likely denied", filepath.Clean(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("read capture: %w", err)
	}

	return Result{
		Text: fmt.Sprintf("Captured screen (%d bytes). The image is attached for you to analyze.", len(data)),
		Images: []brain.ImageBlob{{
			MediaType: "image/png",
			Data:      data,
		}},
	}, nil
}
