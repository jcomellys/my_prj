package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OpenAITTS speaks with OpenAI's neural speech API (gpt-4o-mini-tts family) —
// the "natural app voice" tier, far closer to the Claude/ChatGPT app feel
// than macOS compact voices. Cost is per input character (fractions of a
// cent per sentence); the free tier keeps macos_say.
//
// Design mirrors MacOSSay deliberately:
//   - sentence-by-sentence so the first words play while the rest renders;
//   - each chunk lands as a COMPLETE mp3 played with afplay, so the tail
//     never clips ("se corta al final" stays fixed);
//   - every step honors ctx — a barge-in kills the HTTP call or afplay <1s.
//
// Resilience (accessibility rule: never go mute): any API failure flips this
// Speak call to the Fallback voice (macos_say) for the REST of the text, so
// a network blip degrades quality, not availability.
type OpenAITTS struct {
	APIKey string
	Model  string  // default gpt-4o-mini-tts
	Voice  string  // default marin
	Speed  float64 // 0 = server default (1.0)

	// Fallback speaks when the API fails. Usually MacOSSay. Nil = errors out.
	Fallback TTS

	// BaseURL and HTTP overridable in tests.
	BaseURL string
	HTTP    *http.Client

	// run plays a finished audio file (afplay); overridable in tests.
	run func(ctx context.Context, name string, args ...string) error
}

func NewOpenAITTS(apiKey, model, voice string, speed float64, fallback TTS) *OpenAITTS {
	if model == "" {
		model = "gpt-4o-mini-tts"
	}
	if voice == "" {
		voice = "marin"
	}
	return &OpenAITTS{
		APIKey:   apiKey,
		Model:    model,
		Voice:    voice,
		Speed:    speed,
		Fallback: fallback,
		BaseURL:  "https://api.openai.com/v1",
		HTTP:     &http.Client{Timeout: 60 * time.Second},
		run:      execRun,
	}
}

func (o *OpenAITTS) Name() string { return "openai_tts" }

func (o *OpenAITTS) Speak(ctx context.Context, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	chunks := splitSentences(text)
	for i, chunk := range chunks {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := o.speakChunk(ctx, chunk); err != nil {
			if ctx.Err() != nil {
				return ctx.Err() // barge-in, not an API problem
			}
			// API down / key bad / rate limited: finish the reply on the
			// local voice instead of going silent mid-sentence.
			if o.Fallback != nil {
				return o.Fallback.Speak(ctx, strings.Join(chunks[i:], " "))
			}
			return err
		}
	}
	return nil
}

func (o *OpenAITTS) speakChunk(ctx context.Context, text string) error {
	body, err := json.Marshal(map[string]any{
		"model":           o.Model,
		"voice":           o.Voice,
		"input":           text,
		"response_format": "mp3",
		"speed":           speedOrDefault(o.Speed),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.BaseURL+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("openai tts: %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	f, err := os.CreateTemp("", "vma-tts-*.mp3")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(audio); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	runner := o.run
	if runner == nil {
		runner = execRun
	}
	return runner(ctx, "afplay", tmp)
}

func speedOrDefault(s float64) float64 {
	if s <= 0 {
		return 1.0
	}
	return s
}
