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

	// osascript -l JavaScript -e '<program>' '<path>'  — path is the argv[0]
	// the JXA run() receives. Both pieces are single-quoted so the path can
	// contain spaces or punctuation without breaking the shell.
	cmd := "osascript -l JavaScript -e " + shellQuote(pdfTextJXA) + " " + shellQuote(p)
	out, err := t.OS.RunShell(ctx, cmd)
	if err != nil {
		return Result{}, fmt.Errorf("read_pdf: %w: %s", err, strings.TrimSpace(out))
	}
	out = strings.TrimSpace(out)
	if out == "" {
		// No extractable text: almost always a scanned/image PDF. Tell the
		// brain plainly so it can offer the screenshot fallback.
		return Result{Text: "(El PDF no tiene texto extraíble; probablemente está escaneado como imagen. Sugiere leerlo con una captura de pantalla.)"}, nil
	}

	max := t.MaxBytes
	if max <= 0 {
		max = 200 * 1024
	}
	if len(out) > max {
		out = out[:max] + "\n…(truncated)"
	}
	return Result{Text: out}, nil
}

// shellQuote wraps s in single quotes for /bin/sh, escaping any embedded
// single quotes. Safe for arbitrary file paths.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
