package stt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// WhisperCPP captures audio from the system microphone and transcribes it
// locally using whisper.cpp. Free of API cost, runs at faster-than-realtime
// on Apple Silicon (Mac mini M4 with `small` model: ~5x realtime).
//
// Dependencies (Mac): brew install sox whisper-cpp
// Model:              ~/.whisper-models/ggml-<size>.bin
//
//	tiny:   fastest, weakest (recommend only for english quick demos)
//	base:   small balance, ok for clear speech
//	small:  *recommended* for production; multilingual, decent accuracy
//	medium: better accuracy; slower; ~1.5 GB
//
// Recording pipeline uses `sox -d` with the silence filter:
//
//	sox -d out.wav silence 1 0.1 3% 1 1.5 3%
//
// meaning: start recording as soon as audio above 3% is heard; stop after
// 1.5 s of silence at the same threshold. Adjust SilenceSeconds and
// Threshold for noisy environments. After recording, the audio can be padded
// with silence before transcription; this gives Whisper context at the start
// of very short command clips without sending fake words to the brain.
type WhisperCPP struct {
	// Binary paths. If empty, defaults are looked up on PATH.
	SOXBin     string
	WhisperBin string

	// Required: path to a downloaded whisper.cpp model (.bin).
	ModelPath string

	// Whisper language hint. "es", "en", "auto", etc. Default "auto".
	Language string

	// Silence detection (sox `silence` filter).
	SilenceSeconds float64 // default 1.5
	Threshold      string  // default "3%"

	// MaxListenSeconds caps how long we wait for the user to speak. Without
	// it, sox's silence filter blocks indefinitely when the mic is activated
	// but no speech follows (accidental hotkey / physical switch). On
	// timeout, Listen returns ErrSilent so the turn is skipped and the user
	// hears the "mic closed" cue. Default 10.
	MaxListenSeconds float64

	// Short-clip defense and transcription conditioning.
	MinDurationSeconds float64 // default 0.30; clips below this are silence/noise
	LeadingPadSeconds  float64 // default 0.50; prepended before whisper
	TrailingPadSeconds float64 // default 0.20; appended before whisper
	InitialPrompt      string  // optional whisper prompt for command vocabulary
	NoSpeechThreshold  float64 // optional whisper -nth override

	// Working directory for the temporary recording WAV. Defaults to OS temp.
	TempDir string

	// VerboseEcho writes a one-line "🎤 listening..." banner when recording
	// starts so the user knows the agent is hearing them. Default true.
	VerboseEcho bool
}

func NewWhisperCPP(modelPath string) *WhisperCPP {
	return &WhisperCPP{
		ModelPath:          modelPath,
		Language:           "auto",
		SilenceSeconds:     1.5,
		Threshold:          "3%",
		MinDurationSeconds: 0.30,
		LeadingPadSeconds:  0.50,
		TrailingPadSeconds: 0.20,
		MaxListenSeconds:   10,
		VerboseEcho:        true,
	}
}

func (w *WhisperCPP) Name() string { return "whisper_cpp" }

// PreflightCheck verifies binaries and model are present. Run at agent
// startup so the user gets a clear error before the first conversation
// attempt rather than mid-call.
func (w *WhisperCPP) PreflightCheck() error {
	sox := w.soxBin()
	if _, err := exec.LookPath(sox); err != nil {
		return fmt.Errorf("sox binary not found (%s). Install with: brew install sox", sox)
	}
	wh := w.whisperBin()
	if _, err := exec.LookPath(wh); err != nil {
		return fmt.Errorf("whisper binary not found (%s). Install with: brew install whisper-cpp", wh)
	}
	if w.ModelPath == "" {
		return errors.New("whisper model path is empty (set stt.whisper.model_path in config)")
	}
	if _, err := os.Stat(w.ModelPath); err != nil {
		return fmt.Errorf("whisper model not found at %s: %w. Download from https://huggingface.co/ggerganov/whisper.cpp", w.ModelPath, err)
	}
	return nil
}

func (w *WhisperCPP) Listen(ctx context.Context) (string, error) {
	wav, err := w.record(ctx)
	if err != nil {
		return "", err
	}
	defer os.Remove(wav)

	if w.tooShort(wav) {
		return "", ErrSilent
	}

	transcriptionWav := wav
	if w.LeadingPadSeconds > 0 || w.TrailingPadSeconds > 0 {
		padded, err := w.padForTranscription(ctx, wav)
		if err != nil {
			return "", err
		}
		transcriptionWav = padded
		defer os.Remove(padded)
	}

	text, err := w.transcribe(ctx, transcriptionWav)
	if err != nil {
		return "", err
	}
	cleaned := cleanTranscript(text)
	if isHallucinatedSilence(cleaned) {
		// whisper-small commonly hallucinates [MÚSICA] / [BLANK_AUDIO]
		// on borderline-quiet or sub-second audio. Treat as a silent
		// turn so the orchestrator skips it instead of bothering the
		// brain with an empty / bracketed string.
		return "", ErrSilent
	}
	return cleaned, nil
}

// --- recording with sox ----------------------------------------------------

func (w *WhisperCPP) record(ctx context.Context) (string, error) {
	dir := w.TempDir
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "agent-rec-*.wav")
	if err != nil {
		return "", fmt.Errorf("temp wav: %w", err)
	}
	wavPath := f.Name()
	_ = f.Close()

	if w.VerboseEcho {
		fmt.Println("🎤 escuchando...")
	}

	silenceSecs := w.SilenceSeconds
	if silenceSecs <= 0 {
		silenceSecs = 1.5
	}
	threshold := w.Threshold
	if threshold == "" {
		threshold = "3%"
	}

	// sox -d <out.wav> rate 16000 channels 1 silence <start> <end>
	// Whisper.cpp expects 16kHz mono. Recording at that rate avoids a
	// post-conversion step.
	args := []string{
		"-q",
		"-d",
		"-r", "16000",
		"-c", "1",
		"-b", "16",
		wavPath,
		"silence",
		"1", "0.1", threshold, // start trigger
		"1", strconv.FormatFloat(silenceSecs, 'f', 2, 64), threshold, // stop trigger
	}

	// Cap the wait so an activation with no speech can't block forever.
	// sox's silence filter blocks until audio crosses the threshold; with
	// no timeout, an accidental hotkey press leaves the user stuck.
	recordCtx := ctx
	if w.MaxListenSeconds > 0 {
		var cancel context.CancelFunc
		recordCtx, cancel = context.WithTimeout(ctx, time.Duration(w.MaxListenSeconds*float64(time.Second)))
		defer cancel()
	}

	cmd := exec.CommandContext(recordCtx, w.soxBin(), args...)
	cmd.Stderr = os.Stderr // surface sox errors directly
	if err := cmd.Run(); err != nil {
		_ = os.Remove(wavPath)
		// A real parent cancellation (Ctrl-C) propagates as-is.
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		// Our listen timeout firing means the user never spoke in time.
		if recordCtx.Err() == context.DeadlineExceeded {
			return "", ErrSilent
		}
		return "", fmt.Errorf("sox record: %w", err)
	}

	// If the resulting file is suspiciously small, the user likely activated
	// without speaking. Treat as empty utterance.
	if info, err := os.Stat(wavPath); err == nil && info.Size() < 4096 {
		_ = os.Remove(wavPath)
		return "", ErrSilent
	}
	return wavPath, nil
}

func (w *WhisperCPP) tooShort(wav string) bool {
	minDuration := w.MinDurationSeconds
	if minDuration <= 0 {
		return false
	}
	duration, ok := wavPCM16MonoDurationSeconds(wav)
	return ok && duration < minDuration
}

func (w *WhisperCPP) padForTranscription(ctx context.Context, wav string) (string, error) {
	dir := w.TempDir
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "agent-rec-padded-*.wav")
	if err != nil {
		return "", fmt.Errorf("temp padded wav: %w", err)
	}
	paddedPath := f.Name()
	_ = f.Close()

	args := []string{
		"-q",
		wav,
		paddedPath,
		"pad",
		formatSeconds(w.LeadingPadSeconds),
		formatSeconds(w.TrailingPadSeconds),
	}
	cmd := exec.CommandContext(ctx, w.soxBin(), args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(paddedPath)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("sox pad: %w", err)
	}
	return paddedPath, nil
}

// --- transcription with whisper.cpp ----------------------------------------

func (w *WhisperCPP) transcribe(ctx context.Context, wav string) (string, error) {
	args := []string{
		"-m", w.ModelPath,
		"-f", wav,
		"-l", w.langOrAuto(),
		"-nt", // no timestamps in output
		"-np", // no progress prints
	}
	if w.InitialPrompt != "" {
		args = append(args, "--prompt", w.InitialPrompt)
	}
	if w.NoSpeechThreshold > 0 {
		args = append(args, "-nth", formatSeconds(w.NoSpeechThreshold))
	}
	cmd := exec.CommandContext(ctx, w.whisperBin(), args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("whisper exited %d: %s", ee.ExitCode(), string(ee.Stderr))
		}
		return "", fmt.Errorf("whisper run: %w", err)
	}
	return string(out), nil
}

// --- helpers ----------------------------------------------------------------

// ErrSilent signals the user activated the agent but no speech was captured
// (silence, noise, a too-short clip, or the listen timeout firing). The
// voice loop treats it as a non-fatal "skip this turn". Exported so other
// STT providers and the voice package can produce/recognize it.
var ErrSilent = errors.New("no speech detected")

// IsSilent reports whether the error indicates the user activated but did
// not speak. The voice loop treats this as a non-fatal "skip this turn".
func IsSilent(err error) bool { return errors.Is(err, ErrSilent) }

func (w *WhisperCPP) soxBin() string {
	if w.SOXBin != "" {
		return w.SOXBin
	}
	return "sox"
}

func (w *WhisperCPP) whisperBin() string {
	if w.WhisperBin != "" {
		return w.WhisperBin
	}
	// `whisper-cli` is what Homebrew's whisper-cpp formula installs.
	// Older docs still mention `main`; prefer the modern name.
	return "whisper-cli"
}

func (w *WhisperCPP) langOrAuto() string {
	if w.Language == "" {
		return "auto"
	}
	return w.Language
}

func formatSeconds(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func wavPCM16MonoDurationSeconds(path string) (float64, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	const (
		wavHeaderBytes = 44
		bytesPerSecond = 16000 * 2 // 16kHz, mono, 16-bit PCM from record().
	)
	if info.Size() <= wavHeaderBytes {
		return 0, true
	}
	return float64(info.Size()-wavHeaderBytes) / bytesPerSecond, true
}

// nonSpeechMarker matches bracketed/parenthesized non-speech labels that
// whisper.cpp emits when audio is silent, very short, or ambiguous —
// across languages and capitalizations: [MÚSICA], [Música], [música],
// [BLANK_AUDIO], [Music], [silence], [aplausos], (música), etc.
// Anything inside [] or () that doesn't contain a letter from a normal
// word context gets pruned. The match is greedy enough to catch the
// hallucinations without eating real bracketed user content (rare in
// natural speech).
var nonSpeechMarker = regexp.MustCompile(`(?i)[\[(][^)\]]*?(música|music|silence|blank_audio|applause|aplausos|risas|laughter|ruido|noise|sonido|sound|silencio)[^)\]]*?[\])]`)

// cleanTranscript removes whitespace framing and non-speech markers from
// whisper.cpp output. If after cleaning nothing remains, the caller treats
// the utterance as silent (ErrSilent) instead of forwarding empty text to
// the brain.
func cleanTranscript(s string) string {
	s = strings.TrimSpace(s)
	s = nonSpeechMarker.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// isHallucinatedSilence returns true if the cleaned transcript is empty
// OR consists only of stray punctuation / brackets — both signal that
// whisper heard nothing meaningful.
func isHallucinatedSilence(cleaned string) bool {
	if cleaned == "" {
		return true
	}
	for _, r := range cleaned {
		if !strings.ContainsRune(" \t\n.,;:!?¡¿-—()[]{}\"'`", r) {
			return false
		}
	}
	return true
}

// ExpandHome expands a leading "~" in p to the user's home directory.
// Used so config files can write "~/.whisper-models/ggml-small.bin".
func ExpandHome(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}
