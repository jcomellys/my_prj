package tools

import (
	"context"
	"strings"
	"testing"
)

// fakeOS implements osadapter.Adapter for tests without invoking a real shell.
type fakeOS struct {
	gotCmd     string
	shellOut   string
	shellErr   error
	openAppErr error
}

func (f *fakeOS) Name() string { return "fake" }

func (f *fakeOS) OpenApp(_ context.Context, name string) error {
	f.gotCmd = name
	return f.openAppErr
}

func (f *fakeOS) RunShell(_ context.Context, command string) (string, error) {
	f.gotCmd = command
	return f.shellOut, f.shellErr
}

func (f *fakeOS) RunAppleScript(_ context.Context, script string) (string, error) {
	f.gotCmd = script
	return f.shellOut, f.shellErr
}

func TestShell_AllowlistBlocks(t *testing.T) {
	os := &fakeOS{shellOut: "ok"}
	s := NewShell(os, false, []string{"open "})

	_, err := s.Execute(context.Background(), `{"command":"rm -rf /"}`)
	if err == nil {
		t.Fatal("expected blocked command to error")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("expected allowlist mention in error, got %v", err)
	}
	if os.gotCmd != "" {
		t.Errorf("expected shell never invoked, but it ran %q", os.gotCmd)
	}
}

func TestShell_AllowlistPermits(t *testing.T) {
	os := &fakeOS{shellOut: "ok"}
	s := NewShell(os, false, []string{"open "})

	out, err := s.Execute(context.Background(), `{"command":"open https://example.com"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "ok" {
		t.Errorf("expected ok, got %q", out)
	}
	if os.gotCmd != "open https://example.com" {
		t.Errorf("unexpected command passed to OS: %q", os.gotCmd)
	}
}

func TestShell_UnrestrictedRuns(t *testing.T) {
	os := &fakeOS{shellOut: "ok"}
	s := NewShell(os, true, nil)

	_, err := s.Execute(context.Background(), `{"command":"echo hi"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestShell_OutputTruncated(t *testing.T) {
	long := strings.Repeat("x", 5000)
	os := &fakeOS{shellOut: long}
	s := NewShell(os, true, nil)

	out, err := s.Execute(context.Background(), `{"command":"big"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out) > 2100 {
		t.Errorf("expected truncated output (~2000 chars), got %d", len(out))
	}
	if !strings.Contains(out, "truncated") {
		t.Errorf("expected truncation marker, got %q", out[len(out)-50:])
	}
}

func TestShell_EmptyCommand(t *testing.T) {
	os := &fakeOS{}
	s := NewShell(os, true, nil)

	_, err := s.Execute(context.Background(), `{"command":"   "}`)
	if err == nil {
		t.Fatal("expected error on empty command")
	}
}

func TestOpenApp_ArgsRoundTrip(t *testing.T) {
	os := &fakeOS{}
	t1 := NewOpenApp(os)
	out, err := t1.Execute(context.Background(), `{"name":"Google Chrome"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if os.gotCmd != "Google Chrome" {
		t.Errorf("expected OpenApp invoked with Google Chrome, got %q", os.gotCmd)
	}
	if !strings.Contains(out, "Google Chrome") {
		t.Errorf("expected app name in success message, got %q", out)
	}
}

func TestOpenApp_BadArgs(t *testing.T) {
	os := &fakeOS{}
	t1 := NewOpenApp(os)
	_, err := t1.Execute(context.Background(), `{"name":}`)
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestAppleScript_PassesThrough(t *testing.T) {
	os := &fakeOS{shellOut: "Hello"}
	t1 := NewAppleScript(os)
	out, err := t1.Execute(context.Background(), `{"script":"return \"Hello\""}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "Hello" {
		t.Errorf("expected Hello, got %q", out)
	}
	if !strings.Contains(os.gotCmd, "return") {
		t.Errorf("expected script forwarded to OS, got %q", os.gotCmd)
	}
}

func TestAppleScript_EmptyOutputBecomesOK(t *testing.T) {
	os := &fakeOS{shellOut: ""}
	t1 := NewAppleScript(os)
	out, err := t1.Execute(context.Background(), `{"script":"display dialog \"hi\""}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "OK" {
		t.Errorf("expected OK fallback, got %q", out)
	}
}
