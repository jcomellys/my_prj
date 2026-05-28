package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempPDF creates a throwaway file with a .pdf name so the tool's
// existence check passes. Content is irrelevant — extraction is mocked.
func writeTempPDF(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(p, []byte("%PDF-1.4 fake"), 0o644); err != nil {
		t.Fatalf("write temp pdf: %v", err)
	}
	return p
}

func TestReadPDF_ReturnsExtractedText(t *testing.T) {
	p := writeTempPDF(t)
	osa := &fakeOS{shellOut: "Section 1\nHello world."}
	rp := NewReadPDF(osa)

	out, err := rp.Execute(context.Background(), `{"path":"`+p+`"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Text, "Hello world") {
		t.Errorf("expected extracted text, got %q", out.Text)
	}
	// The command must invoke osascript in JXA mode and pass the (quoted) path.
	if !strings.Contains(osa.gotCmd, "osascript -l JavaScript") {
		t.Errorf("expected JXA osascript invocation, got %q", osa.gotCmd)
	}
	if !strings.Contains(osa.gotCmd, "PDFDocument") {
		t.Errorf("expected PDFKit program in command, got %q", osa.gotCmd)
	}
	if !strings.Contains(osa.gotCmd, p) {
		t.Errorf("expected the pdf path forwarded to the shell, got %q", osa.gotCmd)
	}
}

func TestReadPDF_EmptyMeansScanned(t *testing.T) {
	p := writeTempPDF(t)
	osa := &fakeOS{shellOut: "   "} // PDFKit returns no text for image-only PDFs
	rp := NewReadPDF(osa)

	out, err := rp.Execute(context.Background(), `{"path":"`+p+`"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(strings.ToLower(out.Text), "escaneado") {
		t.Errorf("expected scanned-PDF hint, got %q", out.Text)
	}
	if !strings.Contains(strings.ToLower(out.Text), "captura") {
		t.Errorf("expected screenshot-fallback suggestion, got %q", out.Text)
	}
}

func TestReadPDF_MissingFile(t *testing.T) {
	osa := &fakeOS{shellOut: "should not run"}
	rp := NewReadPDF(osa)

	_, err := rp.Execute(context.Background(), `{"path":"`+filepath.Join(t.TempDir(), "nope.pdf")+`"}`)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if osa.gotCmd != "" {
		t.Errorf("osascript should not run when the file is missing, ran %q", osa.gotCmd)
	}
}

func TestReadPDF_RejectsTraversal(t *testing.T) {
	osa := &fakeOS{}
	rp := NewReadPDF(osa)

	_, err := rp.Execute(context.Background(), `{"path":"../../etc/passwd.pdf"}`)
	if err == nil {
		t.Fatal("expected traversal to be rejected")
	}
	if osa.gotCmd != "" {
		t.Errorf("osascript should not run on a rejected path, ran %q", osa.gotCmd)
	}
}

func TestReadPDF_Truncates(t *testing.T) {
	p := writeTempPDF(t)
	osa := &fakeOS{shellOut: strings.Repeat("x", 5000)}
	rp := NewReadPDF(osa)
	rp.MaxBytes = 1000

	out, err := rp.Execute(context.Background(), `{"path":"`+p+`"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Text, "truncated") {
		t.Errorf("expected truncation marker, got tail %q", out.Text[len(out.Text)-40:])
	}
}

func TestShellQuote_EscapesQuotes(t *testing.T) {
	got := shellQuote("/Users/me/it's a.pdf")
	want := `'/Users/me/it'\''s a.pdf'`
	if got != want {
		t.Errorf("shellQuote = %q, want %q", got, want)
	}
}
