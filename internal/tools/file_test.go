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

func TestWriteFile_RejectsSystemPath(t *testing.T) {
	w := NewWriteFile()
	_, err := w.Execute(context.Background(), `{"path":"/usr/bin/evil","content":"x"}`)
	if err == nil {
		t.Fatal("expected refusal to write to a system path")
	}
	if !strings.Contains(err.Error(), "system path") {
		t.Errorf("expected system-path error, got %v", err)
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
