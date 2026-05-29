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
		Description: "Extract text from a PDF at a KNOWN path (the user gave it, or it's on disk) so it can be read aloud. " +
			"For a PDF the user has OPEN in Preview, use read_open_pdf instead (don't script Preview for the path). " +
			"Pass 'section' to get only that section (recommended — faster, cheaper than the whole document). " +
			"If it returns empty, the PDF is scanned (image-only) — offer a screenshot instead.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "Absolute or ~-relative path to the .pdf file."},
				"section": map[string]any{"type": "string", "description": "Optional: a heading/title to return only that part of the document."},
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
		Path    string `json:"path"`
		Section string `json:"section"`
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
	return extractPDFResult(ctx, t.OS, p, t.MaxBytes, args.Section)
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
			"Pass 'section' to get only that section (recommended — faster, cheaper than the whole document). " +
			"If several PDFs are open it asks which; empty result = scanned/image PDF (offer a screenshot).",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"section": map[string]any{"type": "string", "description": "Optional: a heading/title to return only that part."},
			},
			"additionalProperties": false,
		},
	}
}

func (t *ReadOpenPDF) Execute(ctx context.Context, argsJSON string) (Result, error) {
	var args struct {
		Section string `json:"section"`
	}
	_ = UnmarshalArgs(argsJSON, &args) // section is optional; "{}" is fine

	paths, err := openPreviewPDFs(ctx, t.OS)
	if err != nil {
		return Result{}, err
	}
	switch len(paths) {
	case 0:
		return Result{}, fmt.Errorf("read_open_pdf: no encontré un PDF abierto en Vista Previa. Pídele al usuario que lo abra, o usa read_pdf con la ruta")
	case 1:
		return extractPDFResult(ctx, t.OS, paths[0], t.MaxBytes, args.Section)
	default:
		// Several PDFs open: don't guess which is frontmost. Ask, and give the
		// paths so the model can call read_pdf with the chosen one.
		names := make([]string, len(paths))
		for i, p := range paths {
			names[i] = baseName(p)
		}
		return Result{Text: fmt.Sprintf(
			"Hay varios PDFs abiertos: %s. Pregúntale al usuario cuál leer y usa read_pdf con su ruta. Rutas: %s",
			strings.Join(names, ", "), strings.Join(paths, " | "))}, nil
	}
}

// openPreviewPDFs returns the unique paths of PDFs open in Preview, found via
// lsof — fast, deterministic, and with no Automation consent prompt (it never
// scripts Preview, which is what blocked ~87s live).
func openPreviewPDFs(ctx context.Context, osa osadapter.Adapter) ([]string, error) {
	// lsof -c Preview lists files held by Preview; -Fn prints name lines as
	// "n/path". Keep .pdf paths and strip the leading "n".
	out, err := osa.RunShell(ctx, `lsof -c Preview -Fn 2>/dev/null | grep -i '\.pdf$' | sed 's/^n//'`)
	if err != nil {
		return nil, fmt.Errorf("read_open_pdf: no pude inspeccionar Vista Previa: %w", err)
	}
	seen := map[string]bool{}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		p := strings.TrimSpace(line)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths, nil
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

// sectionWindowBytes bounds how much text a section request returns, keeping
// the model's second round small (the predicted next latency neck).
const sectionWindowBytes = 2500

// extractPDFResult wraps extractPDFText with the scanned-PDF hint, optional
// section narrowing, and the output bound shared by both PDF tools.
func extractPDFResult(ctx context.Context, osa osadapter.Adapter, path string, maxBytes int, section string) (Result, error) {
	out, err := extractPDFText(ctx, osa, path)
	if err != nil {
		return Result{}, fmt.Errorf("read_pdf: %w", err)
	}
	if out == "" {
		return Result{Text: "(El PDF no tiene texto extraíble; probablemente está escaneado como imagen. Sugiere leerlo con una captura de pantalla.)"}, nil
	}
	if section = strings.TrimSpace(section); section != "" {
		narrowed, found := narrowToSection(out, section)
		if !found {
			return Result{Text: fmt.Sprintf("No encontré la sección %q en el PDF. Pídele al usuario el título exacto, o léelo desde el inicio.", section)}, nil
		}
		return Result{Text: narrowed}, nil
	}
	if maxBytes <= 0 {
		maxBytes = 200 * 1024
	}
	if len(out) > maxBytes {
		out = out[:maxBytes] + "\n…(truncated)"
	}
	return Result{Text: out}, nil
}

// narrowToSection returns a bounded window of text starting at the first
// case-insensitive occurrence of section, so a "léeme la sección X" request
// sends only that part to the model instead of the whole document.
func narrowToSection(text, section string) (string, bool) {
	idx := strings.Index(strings.ToLower(text), strings.ToLower(section))
	if idx < 0 {
		return "", false
	}
	end := idx + sectionWindowBytes
	if end > len(text) {
		end = len(text)
	}
	out := text[idx:end]
	if end < len(text) {
		out += "\n…(continúa)"
	}
	return out, true
}

// baseName returns the file name component of a path (no path import churn).
func baseName(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// shellQuote wraps s in single quotes for /bin/sh, escaping any embedded
// single quotes. Safe for arbitrary file paths.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
