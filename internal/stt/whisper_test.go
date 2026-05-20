package stt

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestWhisperCPP_ListenTimesOutWhenNoSpeech reproduces the accessibility bug
// Codex found live: activating the mic without speaking left sox blocked
// indefinitely. With MaxListenSeconds the listen must return ErrSilent
// promptly and never reach whisper.
func TestWhisperCPP_ListenTimesOutWhenNoSpeech(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake not implemented for windows")
	}
	dir := t.TempDir()
	sox := filepath.Join(dir, "sox")
	whisper := filepath.Join(dir, "whisper")
	whisperCalled := filepath.Join(dir, "whisper-called")

	// sox that blocks well past the timeout, simulating "waiting for speech
	// that never comes".
	writeScript(t, sox, "#!/bin/sh\nsleep 5\n")
	writeScript(t, whisper, fmt.Sprintf("#!/bin/sh\ntouch %q\necho hola\n", whisperCalled))

	w := NewWhisperCPP("/tmp/model.bin")
	w.SOXBin = sox
	w.WhisperBin = whisper
	w.VerboseEcho = false
	w.MinDurationSeconds = 0 // isolate timeout behavior
	w.LeadingPadSeconds = 0
	w.TrailingPadSeconds = 0
	w.MaxListenSeconds = 0.3

	start := time.Now()
	got, err := w.Listen(context.Background())
	elapsed := time.Since(start)

	if !IsSilent(err) {
		t.Fatalf("Listen() err = %v, want ErrSilent", err)
	}
	if got != "" {
		t.Fatalf("Listen() text = %q, want empty", got)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("Listen() took %v; timeout did not fire", elapsed)
	}
	if _, err := os.Stat(whisperCalled); !os.IsNotExist(err) {
		t.Fatal("whisper must not run when the listen window times out")
	}
}

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
	if !IsSilent(ErrSilent) {
		t.Error("expected IsSilent(ErrSilent) to be true")
	}
	if IsSilent(nil) {
		t.Error("IsSilent(nil) should be false")
	}
}

func TestWhisperCPPListenSkipsTooShortClipBeforeWhisper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake not implemented for windows")
	}
	dir := t.TempDir()
	sox := filepath.Join(dir, "sox")
	whisper := filepath.Join(dir, "whisper")
	whisperCalled := filepath.Join(dir, "whisper-called")

	// 0.25s at 16kHz mono 16-bit PCM plus a minimal WAV header. This is
	// large enough to pass the legacy byte-size guard but too short to trust.
	writeScript(t, sox, "#!/bin/sh\nout=\"$9\"\ndd if=/dev/zero of=\"$out\" bs=1 count=8044 >/dev/null 2>&1\n")
	writeScript(t, whisper, fmt.Sprintf("#!/bin/sh\ntouch %q\nexit 42\n", whisperCalled))

	w := NewWhisperCPP("/tmp/model.bin")
	w.SOXBin = sox
	w.WhisperBin = whisper
	w.VerboseEcho = false
	w.MinDurationSeconds = 0.30

	got, err := w.Listen(context.Background())
	if !IsSilent(err) {
		t.Fatalf("Listen() err = %v, want silent", err)
	}
	if got != "" {
		t.Fatalf("Listen() text = %q, want empty", got)
	}
	if _, err := os.Stat(whisperCalled); !os.IsNotExist(err) {
		t.Fatalf("whisper was called for too-short audio")
	}
}

func TestWhisperCPPTranscribePassesPromptAndNoSpeechThreshold(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake not implemented for windows")
	}
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	whisper := filepath.Join(dir, "whisper")
	writeScript(t, whisper, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\necho ok\n", argsFile))

	wav := filepath.Join(dir, "clip.wav")
	if err := os.WriteFile(wav, make([]byte, 44+32000), 0o644); err != nil {
		t.Fatal(err)
	}

	w := NewWhisperCPP("/tmp/model.bin")
	w.WhisperBin = whisper
	w.Language = "es"
	w.InitialPrompt = "comandos Mac en español"
	w.NoSpeechThreshold = 0.30

	got, err := w.transcribe(context.Background(), wav)
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	if strings.TrimSpace(got) != "ok" {
		t.Fatalf("transcribe output = %q, want ok", got)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := "\n" + string(data)
	for _, want := range []string{
		"\n-l\nes\n",
		"\n--prompt\ncomandos Mac en español\n",
		"\n-nth\n0.30\n",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("args missing %q in:\n%s", want, args)
		}
	}
}

func TestWavPCM16MonoDurationSeconds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.wav")
	if err := os.WriteFile(path, make([]byte, 44+16000), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := wavPCM16MonoDurationSeconds(path)
	if !ok {
		t.Fatal("duration should be available")
	}
	if math.Abs(got-0.5) > 0.001 {
		t.Fatalf("duration = %.4f, want 0.5", got)
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

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
}
