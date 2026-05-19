package stt

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCleanTranscript(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"  hola mundo  \n", "hola mundo"},
		{"[BLANK_AUDIO]", ""},
		{"[Música]  hola", "hola"},
		{"hello [Music] world", "hello  world"},
		{"", ""},
		// Regression: whisper-small returned this on a fase 0.2 voice test
		// where the user said "Abre Mensajes" — was passed to the brain
		// as-is and the brain ignored it.
		{"[MÚSICA]", ""},
		{"[música]", ""},
		{"  [MÚSICA] ", ""},
		{"abre Chrome [música]", "abre Chrome"},
		{"[Aplausos] gracias", "gracias"},
		{"(música)", ""},
	}
	for _, c := range cases {
		if got := cleanTranscript(c.in); got != c.want {
			t.Errorf("cleanTranscript(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestIsHallucinatedSilence(t *testing.T) {
	cases := []struct {
		in       string
		wantTrue bool
	}{
		{"", true},
		{"   ", true},
		{".", true},
		{"...", true},
		{"[ ]", true},
		{"abre Chrome", false},
		{"hola", false},
		{"?", true},
		{"¿", true},
	}
	for _, c := range cases {
		got := isHallucinatedSilence(c.in)
		if got != c.wantTrue {
			t.Errorf("isHallucinatedSilence(%q) = %v want %v", c.in, got, c.wantTrue)
		}
	}
}

func TestExpandHome(t *testing.T) {
	if got := ExpandHome("/abs/path"); got != "/abs/path" {
		t.Errorf("absolute path unchanged: %q", got)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir available")
	}
	got := ExpandHome("~/foo/bar")
	want := filepath.Join(home, "foo/bar")
	if got != want {
		t.Errorf("ExpandHome(~/foo/bar) = %q want %q", got, want)
	}
}

func TestIsSilent(t *testing.T) {
	if !IsSilent(errSilent) {
		t.Error("expected IsSilent(errSilent) to be true")
	}
	if IsSilent(nil) {
		t.Error("IsSilent(nil) should be false")
	}
}

// TestWhisperCPP_PreflightChecksModel verifies the preflight surfaces a
// helpful error when the model file is missing.
func TestWhisperCPP_PreflightChecksModel(t *testing.T) {
	w := NewWhisperCPP(filepath.Join(t.TempDir(), "ghost.bin"))
	// Fake the binaries so the error is specifically about the model.
	w.SOXBin = fakeOK(t)
	w.WhisperBin = fakeOK(t)

	err := w.PreflightCheck()
	if err == nil {
		t.Fatal("expected error for missing model")
	}
	if !strings.Contains(err.Error(), "ghost.bin") {
		t.Errorf("expected error to mention model path, got %v", err)
	}
}

func TestWhisperCPP_PreflightChecksSox(t *testing.T) {
	w := NewWhisperCPP("/tmp/whatever")
	w.SOXBin = "/nonexistent/sox-ghost"
	w.WhisperBin = fakeOK(t)

	err := w.PreflightCheck()
	if err == nil {
		t.Fatal("expected error for missing sox")
	}
	if !strings.Contains(err.Error(), "sox") {
		t.Errorf("expected error to mention sox, got %v", err)
	}
}

// fakeOK writes a tiny executable script that exits 0, so PreflightCheck's
// LookPath succeeds on it. Returns the absolute path.
func fakeOK(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("not implemented for windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake")
	body := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake: %v", err)
	}
	return path
}
