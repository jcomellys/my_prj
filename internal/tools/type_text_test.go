package tools

import (
	"context"
	"strings"
	"testing"
)

func TestTypeText_TypesWithoutSending(t *testing.T) {
	osa := &fakeOS{}
	tt := NewTypeText(osa)

	out, err := tt.Execute(context.Background(), `{"text":"Hola Claude","submit":false}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(osa.gotCmd, "keystroke \"Hola Claude\"") {
		t.Errorf("expected keystroke of the text, got %q", osa.gotCmd)
	}
	if strings.Contains(osa.gotCmd, "key code 36") {
		t.Errorf("must NOT press Return when submit=false, got %q", osa.gotCmd)
	}
	if !strings.Contains(strings.ToLower(out.Text), "sin enviar") {
		t.Errorf("expected 'sin enviar' confirmation, got %q", out.Text)
	}
}

func TestTypeText_SubmitPressesReturn(t *testing.T) {
	osa := &fakeOS{}
	tt := NewTypeText(osa)

	// Confirmation turn: empty text, submit=true => just press Return.
	if _, err := tt.Execute(context.Background(), `{"text":"","submit":true}`); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(osa.gotCmd, "key code 36") {
		t.Errorf("expected Return key code when submit=true, got %q", osa.gotCmd)
	}
	if strings.Contains(osa.gotCmd, "keystroke") {
		t.Errorf("should not keystroke anything for empty text, got %q", osa.gotCmd)
	}
}

func TestTypeText_NothingToDo(t *testing.T) {
	osa := &fakeOS{}
	tt := NewTypeText(osa)
	if _, err := tt.Execute(context.Background(), `{"text":"","submit":false}`); err == nil {
		t.Fatal("expected error when there is nothing to type or send")
	}
	if osa.gotCmd != "" {
		t.Errorf("must not run any AppleScript when there's nothing to do, ran %q", osa.gotCmd)
	}
}

func TestEscapeAppleScriptString(t *testing.T) {
	got := escapeAppleScriptString(`Él dijo "hola"` + "\n" + `y se fue\`)
	// Quotes and backslashes must be escaped, newline turned into \n.
	if !strings.Contains(got, `\"hola\"`) {
		t.Errorf("quotes not escaped: %q", got)
	}
	if !strings.Contains(got, `\n`) {
		t.Errorf("newline not escaped: %q", got)
	}
	if !strings.Contains(got, `se fue\\`) {
		t.Errorf("backslash not escaped: %q", got)
	}
}
