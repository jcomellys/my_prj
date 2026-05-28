package preflight

import (
	"context"
	"strings"
	"testing"

	"github.com/jcomellys/voice-mac-agent/internal/agent"
)

func TestMicConcern(t *testing.T) {
	cases := []struct {
		vol  int
		want Status
	}{
		{-1, Warn},
		{7, Fail},   // the real value that derailed a live session
		{19, Fail},
		{30, Warn},
		{45, OK},
		{85, OK},
	}
	for _, c := range cases {
		got, detail := MicConcern(c.vol)
		if got != c.want {
			t.Errorf("MicConcern(%d) = %v, want %v (detail %q)", c.vol, got, c.want, detail)
		}
	}
	// The failing case must actually tell the user to raise the volume.
	_, detail := MicConcern(7)
	if !strings.Contains(detail, "Súbelo") {
		t.Errorf("low-volume detail should tell the user to raise it, got %q", detail)
	}
}

func TestVoiceConcern(t *testing.T) {
	installed := []string{"Alex", "Mónica", "Paulina"}

	if st, _ := VoiceConcern("Mónica", installed); st != OK {
		t.Errorf("installed voice should be OK, got %v", st)
	}
	if st, _ := VoiceConcern("mónica", installed); st != OK {
		t.Errorf("voice match should be case-insensitive, got %v", st)
	}
	if st, d := VoiceConcern("NoSuchVoice", installed); st != Warn {
		t.Errorf("missing voice should Warn, got %v (%q)", st, d)
	}
	if st, _ := VoiceConcern("", installed); st != OK {
		t.Errorf("empty voice (system default) should be OK, got %v", st)
	}
}

func TestParseVoiceList(t *testing.T) {
	raw := "Alex                en_US    # Most people recognize me by my voice.\n" +
		"Mónica              es_ES    # Hola, me llamo Mónica.\n" +
		"Bad News            en_US    # The light you see...\n" +
		"\n"
	got := parseVoiceList(raw)
	want := map[string]bool{"Alex": true, "Mónica": true, "Bad News": true}
	if len(got) != 3 {
		t.Fatalf("parsed %d voices, want 3: %v", len(got), got)
	}
	for _, v := range got {
		if !want[v] {
			t.Errorf("unexpected voice parsed: %q", v)
		}
	}
}

func TestParseInputVolume(t *testing.T) {
	if v, err := parseInputVolume(" 7\n"); err != nil || v != 7 {
		t.Errorf("parseInputVolume = %d,%v, want 7,nil", v, err)
	}
	if _, err := parseInputVolume("missing value"); err == nil {
		t.Error("expected error on non-numeric volume")
	}
}

// fakeOS canned-answers the two shell/applescript probes Run makes.
type fakeOS struct {
	voices string
	volume string
	asErr  error
}

func (fakeOS) Name() string                                  { return "fake" }
func (fakeOS) OpenApp(context.Context, string) error         { return nil }
func (f fakeOS) RunShell(context.Context, string) (string, error) {
	return f.voices, nil
}
func (f fakeOS) RunAppleScript(context.Context, string) (string, error) {
	return f.volume, f.asErr
}

func TestRun_FlagsLowMicAndMissingVoice(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	cfg := &agent.Config{
		ActiveProfile: "p",
		Profiles: map[string]agent.Profile{
			"p": {
				Brain: agent.BrainConfig{Provider: "openai", Model: "gpt-5"},
				Voice: agent.VoiceConfig{
					TTS: agent.TTSConfig{Provider: "macos_say", Voice: "Mónica"},
					STT: agent.STTConfig{Provider: "stdin"},
				},
			},
		},
	}
	osa := fakeOS{voices: "Alex   en_US  # hi", volume: "7"}

	checks := Run(context.Background(), cfg, osa)

	var micFail, voiceWarn bool
	for _, c := range checks {
		if c.Name == "micrófono" && c.Status == Fail {
			micFail = true
		}
		if c.Name == "voz (TTS)" && c.Status == Warn {
			voiceWarn = true // Mónica not in the (Alex-only) installed list
		}
	}
	if !micFail {
		t.Error("expected a Fail for the low mic volume (7/100)")
	}
	if !voiceWarn {
		t.Error("expected a Warn for the configured voice not being installed")
	}

	_, fails := Report(checks)
	if fails == 0 {
		t.Error("Report should count the mic failure")
	}
}
