package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write tmp config: %v", err)
	}
	return p
}

func TestLoadConfig_Minimal(t *testing.T) {
	path := writeTemp(t, `
active_profile: free
profiles:
  free:
    voice:
      mode: pipeline
      stt: { provider: stdin }
      tts: { provider: macos_say }
    brain:
      provider: mock
    activator: { kind: stdin }
tools:
  open_app: { enabled: true }
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ActiveProfile != "free" {
		t.Errorf("expected active=free, got %q", cfg.ActiveProfile)
	}
	prof := cfg.Active()
	if prof.Brain.Provider != "mock" {
		t.Errorf("expected brain.provider=mock, got %q", prof.Brain.Provider)
	}
	if !cfg.Tools.OpenApp.Enabled {
		t.Errorf("expected open_app enabled")
	}
}

func TestLoadConfig_MissingActiveProfileErrors(t *testing.T) {
	path := writeTemp(t, `
active_profile: ghost
profiles:
  free:
    brain: { provider: mock }
`)
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error when active_profile has no matching entry")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error to mention missing profile, got %v", err)
	}
}

func TestLoadConfig_WhisperTuningPreservesExplicitZero(t *testing.T) {
	path := writeTemp(t, `
active_profile: voice
profiles:
  voice:
    voice:
      mode: pipeline
      stt:
        provider: whisper_cpp
        whisper:
          model_path: ~/.whisper-models/ggml-small.bin
          min_duration_seconds: 0
          leading_pad_seconds: 0
          trailing_pad_seconds: 0
          no_speech_threshold: 0
      tts: { provider: macos_say }
    brain: { provider: mock }
    activator: { kind: stdin }
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	w := cfg.Active().Voice.STT.Whisper
	cases := map[string]*float64{
		"min_duration_seconds": w.MinDurationSeconds,
		"leading_pad_seconds":  w.LeadingPadSeconds,
		"trailing_pad_seconds": w.TrailingPadSeconds,
		"no_speech_threshold":  w.NoSpeechThreshold,
	}
	for name, got := range cases {
		if got == nil {
			t.Fatalf("%s should preserve explicit zero as non-nil", name)
		}
		if *got != 0 {
			t.Fatalf("%s = %v, want 0", name, *got)
		}
	}
}

func TestLoadConfig_WhisperTuningOmittedIsNil(t *testing.T) {
	path := writeTemp(t, `
active_profile: voice
profiles:
  voice:
    voice:
      mode: pipeline
      stt:
        provider: whisper_cpp
        whisper:
          model_path: ~/.whisper-models/ggml-small.bin
      tts: { provider: macos_say }
    brain: { provider: mock }
    activator: { kind: stdin }
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	w := cfg.Active().Voice.STT.Whisper
	if w.MinDurationSeconds != nil || w.LeadingPadSeconds != nil ||
		w.TrailingPadSeconds != nil || w.NoSpeechThreshold != nil {
		t.Fatalf("omitted whisper tuning fields should remain nil: %+v", w)
	}
}

func TestLoadConfig_EmptyActiveProfileErrors(t *testing.T) {
	path := writeTemp(t, `
profiles:
  free:
    brain: { provider: mock }
`)
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error when active_profile is missing")
	}
}

func TestLoadConfig_NonexistentFile(t *testing.T) {
	_, err := LoadConfig("/no/such/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
