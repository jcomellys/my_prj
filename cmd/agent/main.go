// Command agent is the voice-controlled Mac agent entry point.
//
// Run with: go run ./cmd/agent --config config.yaml
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jcomellys/voice-mac-agent/internal/activator"
	"github.com/jcomellys/voice-mac-agent/internal/agent"
	"github.com/jcomellys/voice-mac-agent/internal/audiocue"
	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/cost"
	"github.com/jcomellys/voice-mac-agent/internal/osadapter"
	"github.com/jcomellys/voice-mac-agent/internal/stt"
	"github.com/jcomellys/voice-mac-agent/internal/tools"
	"github.com/jcomellys/voice-mac-agent/internal/tts"
	"github.com/jcomellys/voice-mac-agent/internal/voice"
)

func main() {
	var (
		configPath = flag.String("config", "config.yaml", "path to YAML config")
		envFile    = flag.String("env", ".env", "path to .env file (optional)")
		verbose    = flag.Bool("v", false, "verbose logging")
	)
	flag.Parse()

	loadDotEnv(*envFile)

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		fatal(log, err)
	}
	prof := cfg.Active()
	log.Info("config.loaded", "profile", cfg.ActiveProfile)

	// --- Brain ---------------------------------------------------------------
	b, err := buildBrain(prof.Brain)
	if err != nil {
		fatal(log, err)
	}
	log.Info("brain.ready", "name", b.Name())

	// --- Cost tracker -------------------------------------------------------
	var tracker *cost.Tracker
	if cfg.Cost.Enabled {
		t, err := cost.NewTracker("cost.log")
		if err != nil {
			fatal(log, err)
		}
		tracker = t
		log.Info("cost.tracker.ready", "path", "cost.log")
	}

	// --- OS Adapter + Tools -------------------------------------------------
	osa := osadapter.NewMacOS()
	registry := tools.NewRegistry()
	if cfg.Tools.OpenApp.Enabled {
		registry.Register(tools.NewOpenApp(osa))
	}
	if cfg.Tools.AppleScript.Enabled {
		registry.Register(tools.NewAppleScript(osa))
	}
	if cfg.Tools.Shell.Enabled {
		registry.Register(tools.NewShell(osa, cfg.Tools.Shell.AllowUnrestricted, cfg.Tools.Shell.Allowlist))
	}
	if cfg.Tools.Screenshot.Enabled {
		registry.Register(tools.NewScreenshot())
	}
	if tracker != nil {
		registry.Register(tools.NewShowCost(tracker))
	}
	log.Info("tools.registered", "count", len(registry.Specs()))

	// --- Voice (STT + TTS + Activator) --------------------------------------
	v, err := buildVoice(prof.Voice, prof.Activator)
	if err != nil {
		fatal(log, err)
	}
	log.Info("voice.ready", "name", v.Name())

	// --- Orchestrator -------------------------------------------------------
	orch := agent.New(v, b, registry, agent.DefaultSystemPrompt, log)
	if tracker != nil {
		orch.WithCost(tracker)
	}

	// --- Run ----------------------------------------------------------------
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Println("Agente listo. Escribe lo que quieras decir y presiona Enter. Ctrl-C para salir.")
	runErr := make(chan error, 1)
	activator.RunWithMainThread(func() {
		runErr <- orch.Run(ctx)
	})
	if err := <-runErr; err != nil && err != context.Canceled {
		fatal(log, err)
	}
}

func buildBrain(c agent.BrainConfig) (brain.Brain, error) {
	switch c.Provider {
	case "openai":
		key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		if key == "" {
			return nil, fmt.Errorf("brain.provider=openai but OPENAI_API_KEY is unset")
		}
		b := brain.NewOpenAI(key, c.Model)
		b.Temperature = c.Temperature
		b.MaxTokens = c.MaxTokens
		return b, nil
	case "anthropic":
		key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
		if key == "" {
			return nil, fmt.Errorf("brain.provider=anthropic but ANTHROPIC_API_KEY is unset")
		}
		b := brain.NewAnthropic(key, c.Model)
		b.Temperature = c.Temperature
		if c.MaxTokens > 0 {
			b.MaxTokens = c.MaxTokens
		}
		return b, nil
	case "ollama":
		host := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
		b := brain.NewOllama(host, c.Model)
		b.Temperature = c.Temperature
		b.NumPredict = c.MaxTokens
		return b, nil
	case "mock", "":
		return brain.NewMock(), nil
	default:
		return nil, fmt.Errorf("brain.provider=%q not yet wired (gemini comes later)", c.Provider)
	}
}

func buildVoice(vc agent.VoiceConfig, ac agent.ActivatorConfig) (voice.Provider, error) {
	if vc.Mode != "" && vc.Mode != "pipeline" {
		return nil, fmt.Errorf("voice.mode=%q not yet supported (only pipeline in fase 0.1)", vc.Mode)
	}

	var sttImpl stt.STT
	switch vc.STT.Provider {
	case "stdin", "":
		sttImpl = stt.NewStdin()
	case "whisper_cpp":
		w := stt.NewWhisperCPP(stt.ExpandHome(vc.STT.Whisper.ModelPath))
		if vc.STT.Whisper.Language != "" {
			w.Language = vc.STT.Whisper.Language
		}
		if vc.STT.Whisper.SilenceSeconds > 0 {
			w.SilenceSeconds = vc.STT.Whisper.SilenceSeconds
		}
		if vc.STT.Whisper.Threshold != "" {
			w.Threshold = vc.STT.Whisper.Threshold
		}
		if vc.STT.Whisper.MaxListenSeconds > 0 {
			w.MaxListenSeconds = vc.STT.Whisper.MaxListenSeconds
		}
		if vc.STT.Whisper.MinDurationSeconds != nil {
			w.MinDurationSeconds = *vc.STT.Whisper.MinDurationSeconds
		}
		if vc.STT.Whisper.LeadingPadSeconds != nil {
			w.LeadingPadSeconds = *vc.STT.Whisper.LeadingPadSeconds
		}
		if vc.STT.Whisper.TrailingPadSeconds != nil {
			w.TrailingPadSeconds = *vc.STT.Whisper.TrailingPadSeconds
		}
		if vc.STT.Whisper.InitialPrompt != "" {
			w.InitialPrompt = vc.STT.Whisper.InitialPrompt
		}
		if vc.STT.Whisper.NoSpeechThreshold != nil {
			w.NoSpeechThreshold = *vc.STT.Whisper.NoSpeechThreshold
		}
		if vc.STT.Whisper.SOXBin != "" {
			w.SOXBin = vc.STT.Whisper.SOXBin
		}
		if vc.STT.Whisper.WhisperBin != "" {
			w.WhisperBin = vc.STT.Whisper.WhisperBin
		}
		if err := w.PreflightCheck(); err != nil {
			return nil, fmt.Errorf("whisper preflight: %w", err)
		}
		sttImpl = w
	default:
		return nil, fmt.Errorf("stt.provider=%q not yet wired", vc.STT.Provider)
	}

	var ttsImpl tts.TTS
	switch vc.TTS.Provider {
	case "macos_say", "":
		ttsImpl = tts.NewMacOSSay(vc.TTS.Voice, vc.TTS.Rate)
	default:
		return nil, fmt.Errorf("tts.provider=%q not yet wired", vc.TTS.Provider)
	}

	var actImpl activator.Activator
	switch ac.Kind {
	case "stdin", "":
		actImpl = activator.NewAlwaysOn()
	case "enter":
		actImpl = activator.NewEnter()
	case "hotkey":
		// Hotkey registration touches Cocoa on macOS, so it must happen
		// inside RunWithMainThread. Hotkey.WaitForActivation registers
		// lazily on the first turn, after the orchestrator is on that thread.
		actImpl = activator.NewHotkey(ac.Hotkey.Combo)
	case "usb_presence":
		name := ac.USB.VolumeName
		if name == "" {
			name = "AGENT"
		}
		actImpl = activator.NewUSBPresence(name)
	default:
		return nil, fmt.Errorf("activator.kind=%q not yet wired (hotkey global comes in fase 0.3)", ac.Kind)
	}

	// Audible earcons (accessibility). Default ON unless explicitly disabled.
	cuesOn := true
	if vc.Cues.Enabled != nil {
		cuesOn = *vc.Cues.Enabled
	}
	cues := audiocue.New(cuesOn, vc.Cues.ListeningSound, vc.Cues.CapturedSound)

	return voice.NewPipeline(sttImpl, ttsImpl, actImpl).WithCues(cues), nil
}

// loadDotEnv parses a minimal KEY=VALUE .env file (no quotes/escapes) and
// sets entries into the process env if they are not already set.
// Silent on missing file — the user may set env vars another way.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		// Strip optional surrounding quotes.
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		if _, present := os.LookupEnv(key); !present {
			_ = os.Setenv(key, val)
		}
	}
}

func fatal(log *slog.Logger, err error) {
	log.Error("fatal", "err", err)
	os.Exit(1)
}
