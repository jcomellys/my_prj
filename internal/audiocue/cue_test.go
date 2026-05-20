package audiocue

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNew_DisabledWhenBackendMissing(t *testing.T) {
	// afplay does not exist on the Linux CI sandbox, so requesting enabled
	// cues must come back disabled rather than failing at play time.
	p := New(true, "", "")
	if runtime.GOOS != "darwin" && p.Enabled() {
		t.Error("cues should auto-disable when afplay backend is absent")
	}
}

func TestNew_DefaultsApplied(t *testing.T) {
	p := New(false, "", "")
	if p.listening != DefaultListeningSound {
		t.Errorf("listening default = %q, want %q", p.listening, DefaultListeningSound)
	}
	if p.captured != DefaultCapturedSound {
		t.Errorf("captured default = %q, want %q", p.captured, DefaultCapturedSound)
	}
}

func TestNew_CustomSoundsApplied(t *testing.T) {
	p := New(false, "/a.aiff", "/b.aiff")
	if p.listening != "/a.aiff" || p.captured != "/b.aiff" {
		t.Errorf("custom sounds not applied: %+v", p)
	}
}

func TestPlay_NoopWhenDisabled(t *testing.T) {
	// Disabled player must not attempt to exec anything. We point bin at a
	// script that would create a file if invoked, then assert it wasn't.
	if runtime.GOOS == "windows" {
		t.Skip("shell sentinel not used on windows")
	}
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "ran")
	fakeBin := filepath.Join(dir, "fakeplayer")
	script := "#!/bin/sh\ntouch " + sentinel + "\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	p := &Player{enabled: false, bin: fakeBin, listening: "/x", captured: "/y"}
	p.Listening()
	p.Captured()

	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("disabled player should never exec the backend")
	}
}

func TestPlay_InvokesBackendWhenEnabled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell sentinel not used on windows")
	}
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "ran")
	fakeBin := filepath.Join(dir, "fakeplayer")
	script := "#!/bin/sh\necho \"$1\" >> " + sentinel + "\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	p := &Player{enabled: true, bin: fakeBin, listening: "/snd/open.aiff", captured: "/snd/close.aiff"}
	p.Listening()
	p.Captured()

	data, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("backend was not invoked: %v", err)
	}
	got := string(data)
	if want := "/snd/open.aiff\n/snd/close.aiff\n"; got != want {
		t.Errorf("backend invoked with %q, want %q", got, want)
	}
}
