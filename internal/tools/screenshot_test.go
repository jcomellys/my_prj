package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestScreenshot_FakeCapture exercises the tool with a stub `screencapture`
// binary that writes a fixed PNG. We can't trigger the real macOS capture
// from CI/Linux, but the tool's plumbing (exec, read, attach) is OS-agnostic.
func TestScreenshot_FakeCapture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not used on windows")
	}
	// Fake binary: writes a tiny but valid-enough PNG to the path passed
	// as the last argument.
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "screencapture")
	pngBytes := []byte{
		0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
	}
	for i := 0; i < 200; i++ {
		pngBytes = append(pngBytes, byte(i))
	}
	payload := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(payload, pngBytes, 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	script := "#!/bin/sh\nfor a in \"$@\"; do last=\"$a\"; done\ncp " + payload + " \"$last\"\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake screencapture: %v", err)
	}

	s := NewScreenshot()
	s.CmdName = fakeBin
	s.TempDir = dir

	res, err := s.Execute(context.Background(), "{}")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Images) != 1 {
		t.Fatalf("expected exactly 1 image attached, got %d", len(res.Images))
	}
	if res.Images[0].MediaType != "image/png" {
		t.Errorf("expected image/png, got %q", res.Images[0].MediaType)
	}
	if len(res.Images[0].Data) != len(pngBytes) {
		t.Errorf("expected %d bytes, got %d", len(pngBytes), len(res.Images[0].Data))
	}
	if !strings.Contains(res.Text, "attached") {
		t.Errorf("expected text to mention attachment, got %q", res.Text)
	}
}

func TestScreenshot_EmptyFileRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not used on windows")
	}
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "screencapture")
	// Writes a 10-byte file — below our 100-byte sanity floor.
	script := "#!/bin/sh\nfor a in \"$@\"; do last=\"$a\"; done\nprintf '0123456789' > \"$last\"\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake screencapture: %v", err)
	}

	s := NewScreenshot()
	s.CmdName = fakeBin
	s.TempDir = dir

	_, err := s.Execute(context.Background(), "{}")
	if err == nil {
		t.Fatal("expected error on tiny capture (permission denial signal)")
	}
	if !strings.Contains(err.Error(), "empty") && !strings.Contains(err.Error(), "permission") {
		t.Errorf("expected permission/empty hint, got %v", err)
	}
}

func TestScreenshot_OversizeRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not used on windows")
	}
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "screencapture")
	// Writes a 1 MiB file.
	script := "#!/bin/sh\nfor a in \"$@\"; do last=\"$a\"; done\nhead -c 1048576 /dev/urandom > \"$last\"\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake screencapture: %v", err)
	}

	s := NewScreenshot()
	s.CmdName = fakeBin
	s.TempDir = dir
	s.MaxBytes = 512 * 1024 // 512 KiB cap

	_, err := s.Execute(context.Background(), "{}")
	if err == nil {
		t.Fatal("expected error when capture exceeds MaxBytes")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' in error, got %v", err)
	}
}
