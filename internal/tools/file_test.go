package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteThenReadFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "note.txt")

	w := NewWriteFile()
	out, err := w.Execute(context.Background(), `{"path":"`+p+`","content":"hola mundo"}`)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(out.Text, "Wrote") {
		t.Errorf("unexpected write result: %q", out.Text)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	r := NewReadFile()
	got, err := r.Execute(context.Background(), `{"path":"`+p+`"}`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Text != "hola mundo" {
		t.Errorf("read = %q, want %q", got.Text, "hola mundo")
	}
}

func TestWriteFile_ExpandsHomeAndOverwrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := filepath.Join(home, "voice-agent", "note.txt")

	w := NewWriteFile()
	if _, err := w.Execute(context.Background(), `{"path":"~/voice-agent/note.txt","content":"primero"}`); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if _, err := w.Execute(context.Background(), `{"path":"~/voice-agent/note.txt","content":"segundo"}`); err != nil {
		t.Fatalf("write overwrite: %v", err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read overwritten file: %v", err)
	}
	if string(got) != "segundo" {
		t.Fatalf("content after overwrite = %q", string(got))
	}
}

func TestWriteFile_RejectsSystemPath(t *testing.T) {
	w := NewWriteFile()
	for _, root := range systemRoots {
		p := filepath.Join(root, "voice-agent-test.txt")
		_, err := w.Execute(context.Background(), `{"path":"`+p+`","content":"x"}`)
		if err == nil {
			t.Fatalf("expected refusal to write to system root %s", root)
		}
		if !strings.Contains(err.Error(), "system path") {
			t.Errorf("expected system-path error for %s, got %v", root, err)
		}
	}
}

func TestFile_RejectsTraversal(t *testing.T) {
	w := NewWriteFile()
	_, err := w.Execute(context.Background(), `{"path":"../../etc/passwd","content":"x"}`)
	if err == nil {
		t.Fatal("expected traversal rejection")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Errorf("expected traversal error, got %v", err)
	}
}

func TestReadFile_Truncates(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.txt")
	big := strings.Repeat("x", 5000)
	if err := os.WriteFile(p, []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewReadFile()
	r.MaxBytes = 1000
	got, err := r.Execute(context.Background(), `{"path":"`+p+`"}`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(got.Text, "truncated") {
		t.Errorf("expected truncation marker")
	}
	if len(got.Text) > 1100 {
		t.Errorf("expected bounded output, got %d", len(got.Text))
	}
}

func TestReadFile_DefaultLimitIs200KiB(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big-default.txt")
	big := strings.Repeat("a", 200*1024+17)
	if err := os.WriteFile(p, []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := NewReadFile().Execute(context.Background(), `{"path":"`+p+`"}`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(got.Text, "truncated") {
		t.Fatalf("expected truncation marker, got suffix %q", got.Text[len(got.Text)-20:])
	}
	if !strings.HasPrefix(got.Text, strings.Repeat("a", 200*1024)) {
		t.Fatalf("expected 200 KiB prefix before truncation marker")
	}
	if strings.Contains(got.Text, strings.Repeat("a", 200*1024+1)) {
		t.Fatalf("read_file returned more than the default 200 KiB payload")
	}
}
