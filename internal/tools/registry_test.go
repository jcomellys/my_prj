package tools

import (
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	os := &fakeOS{}
	r.Register(NewOpenApp(os))
	r.Register(NewShell(os, true, nil))

	if _, ok := r.Get("open_app"); !ok {
		t.Error("expected open_app to be registered")
	}
	if _, ok := r.Get("run_shell"); !ok {
		t.Error("expected run_shell to be registered")
	}
	if _, ok := r.Get("nope"); ok {
		t.Error("expected unknown tool to be missing")
	}
	if got := len(r.Specs()); got != 2 {
		t.Errorf("expected 2 specs, got %d", got)
	}
}

func TestRegistry_LastRegistrationWins(t *testing.T) {
	r := NewRegistry()
	os := &fakeOS{}
	r.Register(NewShell(os, false, []string{"a"}))
	r.Register(NewShell(os, true, nil)) // replace
	if got := len(r.Specs()); got != 1 {
		t.Errorf("expected 1 (replaced) spec, got %d", got)
	}
}
