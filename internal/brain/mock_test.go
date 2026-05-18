package brain

import (
	"context"
	"strings"
	"testing"
)

func TestMock_PlainEcho(t *testing.T) {
	m := NewMock()
	resp, err := m.Chat(context.Background(), []Message{
		{Role: RoleSystem, Content: "ignore"},
		{Role: RoleUser, Content: "hola"},
	}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
	if !strings.Contains(resp.Text, "hola") {
		t.Errorf("expected echo to contain user text, got %q", resp.Text)
	}
}

func TestMock_OpenApp(t *testing.T) {
	m := NewMock()
	resp, err := m.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "abre Google Chrome"},
	}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "open_app" {
		t.Errorf("expected open_app tool, got %q", tc.Name)
	}
	if !strings.Contains(tc.Arguments, "Google Chrome") {
		t.Errorf("expected args to contain app name, got %q", tc.Arguments)
	}
}

func TestMock_UsesLastUserMessage(t *testing.T) {
	m := NewMock()
	resp, err := m.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "first"},
		{Role: RoleAssistant, Content: "ok"},
		{Role: RoleUser, Content: "abre Terminal"},
	}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "open_app" {
		t.Fatalf("expected open_app on last user message, got %+v", resp)
	}
}
