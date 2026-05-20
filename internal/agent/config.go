package agent

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ActiveProfile string             `yaml:"active_profile"`
	Profiles      map[string]Profile `yaml:"profiles"`
	Tools         ToolsConfig        `yaml:"tools"`
	Cost          CostConfig         `yaml:"cost"`
}

type Profile struct {
	Voice     VoiceConfig     `yaml:"voice"`
	Brain     BrainConfig     `yaml:"brain"`
	Activator ActivatorConfig `yaml:"activator"`
}

type VoiceConfig struct {
	Mode string     `yaml:"mode"` // pipeline | realtime
	STT  STTConfig  `yaml:"stt"`
	TTS  TTSConfig  `yaml:"tts"`
	Cues CuesConfig `yaml:"cues"`
}

// CuesConfig controls the audible earcons that tell the user when the mic
// opens and closes. Accessibility-critical, so cues default to ON when
// Enabled is omitted (nil). Set enabled: false to silence them.
type CuesConfig struct {
	Enabled        *bool  `yaml:"enabled"`         // nil => default on
	ListeningSound string `yaml:"listening_sound"` // path to "mic open" sound
	CapturedSound  string `yaml:"captured_sound"`  // path to "mic closed" sound
}

type STTConfig struct {
	Provider string           `yaml:"provider"` // stdin | whisper_cpp | macos_speech | openai_whisper
	Whisper  WhisperSTTConfig `yaml:"whisper"`
}

type WhisperSTTConfig struct {
	ModelPath          string   `yaml:"model_path"`           // e.g. "~/.whisper-models/ggml-small.bin"
	Language           string   `yaml:"language"`             // "es" | "en" | "auto"
	SilenceSeconds     float64  `yaml:"silence_seconds"`      // end-of-utterance threshold; default 1.5
	Threshold          string   `yaml:"threshold"`            // sox amplitude threshold, e.g. "3%"
	MaxListenSeconds   float64  `yaml:"max_listen_seconds"`   // cap wait for speech; default 10, 0 keeps default
	MinDurationSeconds *float64 `yaml:"min_duration_seconds"` // reject shorter clips before whisper
	LeadingPadSeconds  *float64 `yaml:"leading_pad_seconds"`  // silence prepended before whisper
	TrailingPadSeconds *float64 `yaml:"trailing_pad_seconds"` // silence appended before whisper
	InitialPrompt      string   `yaml:"initial_prompt"`       // whisper vocabulary/context hint
	NoSpeechThreshold  *float64 `yaml:"no_speech_threshold"`  // optional whisper -nth override
	SOXBin             string   `yaml:"sox_bin"`              // override "sox" path
	WhisperBin         string   `yaml:"whisper_bin"`          // override "whisper-cli" path
}

type TTSConfig struct {
	Provider string `yaml:"provider"` // macos_say | piper | elevenlabs | openai_tts
	Voice    string `yaml:"voice"`
	Rate     int    `yaml:"rate"`
}

type BrainConfig struct {
	Provider    string  `yaml:"provider"` // openai | anthropic | gemini | ollama | mock
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

type ActivatorConfig struct {
	Kind   string            `yaml:"kind"` // stdin | enter | hotkey | usb_presence
	Hotkey HotkeyConfig      `yaml:"hotkey"`
	USB    USBPresenceConfig `yaml:"usb"`
}

type HotkeyConfig struct {
	Combo string `yaml:"combo"` // default "ctrl+option+space"
}

type USBPresenceConfig struct {
	VolumeName string `yaml:"volume_name"` // default "AGENT"
}

type ToolsConfig struct {
	Shell       ShellToolConfig `yaml:"shell"`
	OpenApp     ToolEnable      `yaml:"open_app"`
	AppleScript ToolEnable      `yaml:"applescript"`
	Screenshot  ToolEnable      `yaml:"screenshot"`
}

type ToolEnable struct {
	Enabled bool `yaml:"enabled"`
}

type ShellToolConfig struct {
	Enabled           bool     `yaml:"enabled"`
	AllowUnrestricted bool     `yaml:"allow_unrestricted"`
	Allowlist         []string `yaml:"allowlist"`
}

type CostConfig struct {
	Enabled          bool    `yaml:"enabled"`
	MonthlyBudgetUSD float64 `yaml:"monthly_budget_usd"`
	WarnAtPct        int     `yaml:"warn_at_pct"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if c.ActiveProfile == "" {
		return nil, fmt.Errorf("config: active_profile is required")
	}
	if _, ok := c.Profiles[c.ActiveProfile]; !ok {
		return nil, fmt.Errorf("config: active_profile %q has no matching profile entry", c.ActiveProfile)
	}
	return &c, nil
}

func (c *Config) Active() Profile {
	return c.Profiles[c.ActiveProfile]
}
