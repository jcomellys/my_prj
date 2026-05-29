package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jcomellys/voice-mac-agent/internal/brain"
	"github.com/jcomellys/voice-mac-agent/internal/osadapter"
)

// ReadPDF extracts the text of a PDF using macOS-native PDFKit, invoked
// through JavaScript for Automation (JXA) via osascript. This needs nothing
// installed — PDFKit ships with macOS — so the portable binary stays portable.
//
// The core accessibility use case: a user who cannot see asks "léeme este PDF"
// or "léeme la sección X". The brain first gets the open document's path from
// Preview (AppleScript: `tell application "Preview" to get path of front
// document`), then calls read_pdf with that path; the returned text is read
// back (and translated to the user's language if needed).
//
// Runs through the OS adapter's RunShell — NOT the allowlist-gated Shell tool —
// so it is always available regardless of the shell allowlist. The command is
// fixed (osascript) and the path is shell-quoted, so the brain cannot smuggle
// arbitrary shell through it.
type ReadPDF struct {
	OS       osadapter.Adapter
	MaxBytes int
}

func NewReadPDF(os osadapter.Adapter) *ReadPDF {
	return &ReadPDF{OS: os, MaxBytes: 200 * 1024}
}

func (ReadPDF) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name: "read_pdf",
		Description: "Extract the text of a PDF file so it can be read aloud to the user. " +
			"To read a PDF the user has open, first get its path from Preview via run_applescript " +
			"(`tell application \"Preview\" to get path of front document`), then call this with that path. " +
			"Also works for any .pdf on disk. Returns the document text; locate the requested section in it. " +
			"If it returns empty, the PDF is scanned (image-only) — offer to read it via a screenshot instead.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Absolute or ~-relative path to the .pdf file."},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
	}
}

// pdfTextJXA is the JavaScript-for-Automation program that loads a PDF with
// PDFKit and returns its full text. argv[0] is the file path. A nil document
// (unreadable file) throws inside PDFKit; we catch it and return empty so the
// Go layer can give the user a clear message rather than a stack trace.
const pdfTextJXA = `function run(argv){ObjC.import("Quartz");try{var u=$.NSURL.fileURLWithPath(argv[0]);var d=$.PDFDocument.alloc.initWithURL(u);var s=ObjC.unwrap(d.string);return s?s:"";}catch(e){return "";}}`

func (t *ReadPDF) Execute(ctx context.Context, argsJSON string) (Result, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := UnmarshalArgs(argsJSON, &args); err != nil {
		return Result{}, err
	}
	p, err := safePath(args.Path)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return Result{}, fmt.Errorf("read_pdf: no encuentro el archivo: %w", err)
	}
	if info.IsDir() {
		return Result{}, fmt.Errorf("read_pdf: %s es una carpeta, no un PDF", p)
	}
	return extractPDFResult(ctx, t.OS, p, t.MaxBytes)
}

// ReadOpenPDF reads the PDF currently open in Preview WITHOUT scripting
// Preview. Asking Preview for the path via AppleScript was measured at ~87s
// live (it blocks on the first-run Automation consent prompt / AppleEvent
// timeout) — fatal for a voice assistant. Instead we find the open file with
// `lsof` (sub-second, no consent prompt) and extract text with PDFKit. One
// deterministic tool call replaces a slow AppleScript round entirely.
type ReadOpenPDF struct {
	OS       osadapter.Adapter
	MaxBytes int
}

func NewReadOpenPDF(os osadapter.Adapter) *ReadOpenPDF {
	return &ReadOpenPDF{OS: os, MaxBytes: 200 * 1024}
}

func (ReadOpenPDF) Spec() brain.ToolSpec {
	return brain.ToolSpec{
		Name: "read_open_pdf",
		Description: "Read the PDF the user currently has open in Preview, aloud-ready. Use this for " +
			"\"léeme este PDF\" / \"léeme la sección X\" when a PDF is on screen — it finds the open file and " +
			"extracts its text in one fast step. Do NOT script Preview for the path (that hangs). " +
			"Locate the requested section in the returned text. Empty result = scanned/image PDF: offer a screenshot.",
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
	}
}

func (t *ReadOpenPDF) Execute(ctx context.Context, _ string) (Result, error) {
	path, err := frontmostPreviewPDF(ctx, t.OS)
	if err != nil {
		return Result{}, err
	}
	return extractPDFResult(ctx, t.OS, path, t.MaxBytes)
}

// frontmostPreviewPDF returns the path of a PDF open in Preview, found via
// lsof — fast and with no Automation consent prompt. Returns the first match;
// if several PDFs are open it may not be the frontmost, which is acceptable
// for the common single-document case.
func frontmostPreviewPDF(ctx context.Context, osa osadapter.Adapter) (string, error) {
	// lsof -c Preview lists files held by the Preview process; -Fn prints
	// name lines as "n/path". Keep .pdf paths, strip the leading "n".
	out, err := osa.RunShell(ctx, `lsof -c Preview -Fn 2>/dev/null | grep -i '\.pdf$' | sed 's/^n//' | head -n1`)
	if err != nil {
		return "", fmt.Errorf("read_open_pdf: no pude inspeccionar Vista Previa: %w", err)
	}
	path := strings.TrimSpace(out)
	if i := strings.IndexByte(path, '\n'); i >= 0 {
		path = path[:i]
	}
	if path == "" {
		return "", fmt.Errorf("read_open_pdf: no encontré un PDF abierto en Vista Previa. Pídele al usuario que lo abra, o usa la ruta con read_pdf")
	}
	return path, nil
}

// extractPDFText runs PDFKit (via JXA) over a path and returns the trimmed
// text. Empty text means a scanned/image PDF.
func extractPDFText(ctx context.Context, osa osadapter.Adapter, path string) (string, error) {
	// osascript -l JavaScript -e '<program>' '<path>'  — path is argv[0] in
	// run(). Both pieces are single-quoted so paths with spaces are safe.
	cmd := "osascript -l JavaScript -e " + shellQuote(pdfTextJXA) + " " + shellQuote(path)
	out, err := osa.RunShell(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}

// extractPDFResult wraps extractPDFText with the scanned-PDF hint and the
// output bound shared by both PDF tools.
func extractPDFResult(ctx context.Context, osa osadapter.Adapter, path string, maxBytes int) (Result, error) {
	out, err := extractPDFText(ctx, osa, path)
	if err != nil {
		return Result{}, fmt.Errorf("read_pdf: %w", err)
	}
	if out == "" {
		return Result{Text: "(El PDF no tiene texto extraíble; probablemente está escaneado como imagen. Sugiere leerlo con una captura de pantalla.)"}, nil
	}
	if maxBytes <= 0 {
		maxBytes = 200 * 1024
	}
	if len(out) > maxBytes {
		out = out[:maxBytes] + "\n…(truncated)"
	}
	return Result{Text: out}, nil
}

// shellQuote wraps s in single quotes for /bin/sh, escaping any embedded
// single quotes. Safe for arbitrary file paths.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
