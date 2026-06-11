package tts

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeTTS records fallback usage.
type fakeTTS struct {
	mu     sync.Mutex
	spoken []string
}

func (f *fakeTTS) Name() string { return "fake" }
func (f *fakeTTS) Speak(_ context.Context, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.spoken = append(f.spoken, text)
	return nil
}

func newTestOpenAITTS(t *testing.T, srvURL string, fallback TTS) (*OpenAITTS, *[]string) {
	t.Helper()
	var played []string
	o := NewOpenAITTS("sk-test", "", "", 0, fallback)
	o.BaseURL = srvURL
	o.run = func(_ context.Context, name string, args ...string) error {
		played = append(played, name+" "+strings.Join(args, " "))
		return nil
	}
	return o, &played
}

func TestOpenAITTS_RequestShapeAndPlayback(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte("fake-mp3-bytes"))
	}))
	defer srv.Close()

	o, played := newTestOpenAITTS(t, srv.URL, nil)
	if err := o.Speak(context.Background(), "Hola mundo."); err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotBody["model"] != "gpt-4o-mini-tts" || gotBody["voice"] != "marin" {
		t.Errorf("defaults not applied: %+v", gotBody)
	}
	if gotBody["input"] != "Hola mundo." {
		t.Errorf("input = %v", gotBody["input"])
	}
	if len(*played) != 1 || !strings.HasPrefix((*played)[0], "afplay ") {
		t.Errorf("expected one afplay invocation, got %v", *played)
	}
}

func TestOpenAITTS_SpeaksSentenceBySentence(t *testing.T) {
	var inputs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		inputs = append(inputs, body["input"].(string))
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	o, played := newTestOpenAITTS(t, srv.URL, nil)
	if err := o.Speak(context.Background(), "Primera frase. Segunda frase. Tercera."); err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if len(inputs) < 2 {
		t.Errorf("expected sentence-by-sentence requests, got %d: %v", len(inputs), inputs)
	}
	if len(*played) != len(inputs) {
		t.Errorf("each chunk must be played: %d requests vs %d plays", len(inputs), len(*played))
	}
}

func TestOpenAITTS_FallsBackOnAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	fb := &fakeTTS{}
	o, played := newTestOpenAITTS(t, srv.URL, fb)
	if err := o.Speak(context.Background(), "Hola. Adiós."); err != nil {
		t.Fatalf("Speak must not fail when fallback succeeds: %v", err)
	}
	if len(*played) != 0 {
		t.Errorf("nothing should play via afplay on API error, got %v", *played)
	}
	if len(fb.spoken) != 1 || !strings.Contains(fb.spoken[0], "Hola") || !strings.Contains(fb.spoken[0], "Adiós") {
		t.Errorf("fallback must speak the remaining text once, got %v", fb.spoken)
	}
}

func TestOpenAITTS_BargeInWinsOverFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	fb := &fakeTTS{}
	o, _ := newTestOpenAITTS(t, srv.URL, fb)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // barge-in before speaking
	if err := o.Speak(ctx, "Hola."); err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if len(fb.spoken) != 0 {
		t.Errorf("a barge-in must not trigger the fallback voice, got %v", fb.spoken)
	}
}
