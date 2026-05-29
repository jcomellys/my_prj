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

// scriptedOS answers RunShell based on the command, so we can simulate the
// two-step read_open_pdf flow (lsof to find the path, then osascript/PDFKit).
type scriptedOS struct {
	lsofOut string
	lsofErr error
	pdfText string
	gotLsof string
	gotPDF  string
}

func (scriptedOS) Name() string                          { return "scripted" }
func (scriptedOS) OpenApp(context.Context, string) error { return nil }
func (s *scriptedOS) RunShell(_ context.Context, cmd string) (string, error) {
	if strings.Contains(cmd, "lsof") {
		s.gotLsof = cmd
		return s.lsofOut, s.lsofErr
	}
	s.gotPDF = cmd
	return s.pdfText, nil
}
func (s *scriptedOS) RunAppleScript(context.Context, string) (string, error) { return "", nil }

func TestReadOpenPDF_FindsViaLsofAndExtracts(t *testing.T) {
	osa := &scriptedOS{
		lsofOut: "/Users/j/Desktop/paper.pdf\n",
		pdfText: "Introduction\nThis paper explains transistors.",
	}
	out, err := NewReadOpenPDF(osa).Execute(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Text, "transistors") {
		t.Errorf("expected extracted text, got %q", out.Text)
	}
	// Must locate the PDF with lsof, NOT by scripting Preview (the 87s path).
	if !strings.Contains(osa.gotLsof, "lsof") {
		t.Errorf("expected lsof lookup, got %q", osa.gotLsof)
	}
	// The found path must be passed (quoted) to the PDFKit extractor.
	if !strings.Contains(osa.gotPDF, "/Users/j/Desktop/paper.pdf") {
		t.Errorf("expected the lsof path forwarded to PDFKit, got %q", osa.gotPDF)
	}
	if strings.Contains(osa.gotLsof, "Preview\" to get path") || strings.Contains(osa.gotPDF, "to get path of front document") {
		t.Error("must never script Preview for the path")
	}
}

func TestReadOpenPDF_SectionReturnsOnlyThatPart(t *testing.T) {
	full := "Resumen\nbla bla bla.\n" + "Introducción\nLos transistores amplifican señales. " + strings.Repeat("X", 5000)
	osa := &scriptedOS{lsofOut: "/tmp/p.pdf", pdfText: full}
	out, err := NewReadOpenPDF(osa).Execute(context.Background(), `{"section":"Introducción"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasPrefix(out.Text, "Introducción") {
		t.Errorf("section result must start at the heading, got %q", out.Text[:30])
	}
	if strings.Contains(out.Text, "Resumen") {
		t.Errorf("section result must NOT include earlier sections, got %q", out.Text[:30])
	}
	if len(out.Text) > sectionWindowBytes+40 {
		t.Errorf("section must be bounded (%d), got %d bytes", sectionWindowBytes, len(out.Text))
	}
}

// TestReadOpenPDF_BilingualSectionMatch covers the live failure from X-014:
// the user said "introducción" (Spanish) but the PDF's heading was
// "Introduction" (English). The narrow must still find it.
func TestReadOpenPDF_BilingualSectionMatch(t *testing.T) {
	full := "Abstract\nShort summary.\n\nIntroduction\nTransistors amplify signals. " + strings.Repeat("X", 4000)
	osa := &scriptedOS{lsofOut: "/tmp/p.pdf", pdfText: full}
	out, err := NewReadOpenPDF(osa).Execute(context.Background(), `{"section":"introducción"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasPrefix(out.Text, "Introduction") {
		t.Errorf("expected the section to start at the English heading, got %q", out.Text[:30])
	}
	if strings.Contains(out.Text, "Abstract") {
		t.Errorf("must not include the earlier Abstract section")
	}
}

// TestReadOpenPDF_AccentInsensitiveSectionMatch: STT may drop the accent, so
// "introduccion" must still find an "Introducción" heading.
func TestReadOpenPDF_AccentInsensitiveSectionMatch(t *testing.T) {
	full := "Resumen\ncorto.\n\nIntroducción\nLos transistores amplifican. " + strings.Repeat("X", 4000)
	osa := &scriptedOS{lsofOut: "/tmp/p.pdf", pdfText: full}
	out, err := NewReadOpenPDF(osa).Execute(context.Background(), `{"section":"introduccion"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasPrefix(out.Text, "Introducción") {
		t.Errorf("expected to find the accented heading from a non-accented request, got %q", out.Text[:30])
	}
}

func TestFoldedIndex_PreservesOriginalOffset(t *testing.T) {
	// "Introducción" in UTF-8: ó = 2 bytes, so the heading starts at byte 16
	// (after "Resumen\ncorto.\n\n"). The folded version is shorter, but the
	// returned offset must point into the ORIGINAL string so slicing works.
	text := "Resumen\ncorto.\n\nIntroducción\nbody"
	idx := foldedIndex(text, "introduccion")
	want := strings.Index(text, "Introducción")
	if idx != want {
		t.Errorf("foldedIndex = %d, want %d (original byte offset of accented heading)", idx, want)
	}
}

func TestReadOpenPDF_SectionNotFound(t *testing.T) {
	osa := &scriptedOS{lsofOut: "/tmp/p.pdf", pdfText: "Solo hay un resumen aquí."}
	out, err := NewReadOpenPDF(osa).Execute(context.Background(), `{"section":"Conclusiones"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(strings.ToLower(out.Text), "no encontré la sección") {
		t.Errorf("expected a 'section not found' message, got %q", out.Text)
	}
}

func TestReadOpenPDF_MultipleAsksWhich(t *testing.T) {
	osa := &scriptedOS{lsofOut: "/Users/j/a.pdf\n/Users/j/b.pdf\n/Users/j/a.pdf\n"} // dup on purpose
	out, err := NewReadOpenPDF(osa).Execute(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.Text, "varios PDFs") {
		t.Errorf("expected an ask-which message, got %q", out.Text)
	}
	if !strings.Contains(out.Text, "a.pdf") || !strings.Contains(out.Text, "b.pdf") {
		t.Errorf("expected both file names listed, got %q", out.Text)
	}
	if strings.Contains(osa.gotPDF, "PDFDocument") {
		t.Error("must not extract any PDF until the user picks one")
	}
}

func TestReadOpenPDF_NoOpenPDF(t *testing.T) {
	osa := &scriptedOS{lsofOut: "  \n"} // lsof found nothing
	_, err := NewReadOpenPDF(osa).Execute(context.Background(), `{}`)
	if err == nil {
		t.Fatal("expected an error when no PDF is open")
	}
	if !strings.Contains(err.Error(), "no encontré un PDF abierto") {
		t.Errorf("expected a clear 'no open PDF' message, got %v", err)
	}
}

func TestReadOpenPDF_ScannedPDF(t *testing.T) {
	osa := &scriptedOS{lsofOut: "/tmp/scan.pdf", pdfText: "   "} // PDFKit returns no text
	out, err := NewReadOpenPDF(osa).Execute(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(strings.ToLower(out.Text), "escaneado") {
		t.Errorf("expected scanned-PDF hint, got %q", out.Text)
	}
}

func TestShellQuote_EscapesQuotes(t *testing.T) {
	got := shellQuote("/Users/me/it's a.pdf")
	want := `'/Users/me/it'\''s a.pdf'`
	if got != want {
		t.Errorf("shellQuote = %q, want %q", got, want)
	}
}
