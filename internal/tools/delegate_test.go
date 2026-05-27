package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeDelegate struct {
	gotTask string
	reply   string
	err     error
}

func (f *fakeDelegate) Run(_ context.Context, task string) (string, error) {
	f.gotTask = task
	return f.reply, f.err
}

func TestDelegateTask_PassesTaskAndReturnsSummary(t *testing.T) {
	d := &fakeDelegate{reply: "Listo: app creada en ~/proyecto."}
	tool := NewDelegateTask(d)

	out, err := tool.Execute(context.Background(), `{"task":"crea una app de notas"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if d.gotTask != "crea una app de notas" {
		t.Errorf("delegate got task %q", d.gotTask)
	}
	if !strings.Contains(out.Text, "app creada") {
		t.Errorf("expected sub-agent summary, got %q", out.Text)
	}
}

func TestDelegateTask_EmptyTask(t *testing.T) {
	tool := NewDelegateTask(&fakeDelegate{})
	_, err := tool.Execute(context.Background(), `{"task":"  "}`)
	if err == nil {
		t.Fatal("expected error on empty task")
	}
}

func TestDelegateTask_NoDelegate(t *testing.T) {
	tool := NewDelegateTask(nil)
	_, err := tool.Execute(context.Background(), `{"task":"algo"}`)
	if err == nil {
		t.Fatal("expected error when no sub-agent configured")
	}
	if !strings.Contains(err.Error(), "no configurado") {
		t.Fatalf("expected no configurado error, got %v", err)
	}
}

func TestDelegateTask_PropagatesError(t *testing.T) {
	d := &fakeDelegate{err: errors.New("boom")}
	tool := NewDelegateTask(d)
	_, err := tool.Execute(context.Background(), `{"task":"x"}`)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected propagated error, got %v", err)
	}
}
