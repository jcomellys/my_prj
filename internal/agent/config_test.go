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
