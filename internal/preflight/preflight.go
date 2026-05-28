// Package preflight runs environment readiness checks so a user — especially
// one who cannot see the screen — knows the agent is fully set up BEFORE a
// voice session, instead of discovering a dead mic or a missing voice mid-use.
//
// It exists because a live session was derailed by two silent setup problems:
// the microphone input volume was at 7/100 (almost inaudible) and the default
// voice sounded robotic. `--doctor` surfaces exactly these, with the fix.
package preflight

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/jcomellys/voice-mac-agent/internal/agent"
	"github.com/jcomellys/voice-mac-agent/internal/osadapter"
)

// Status is the outcome of a single check.
type Status int

const (
	OK Status = iota
	Warn
	Fail
)

func (s Status) icon() string {
	switch s {
	case OK:
		return "✅"
	case Warn:
		return "⚠️ "
	default:
		return "❌"
	}
}

// Check is one readiness finding.
type Check struct {
	Name   string
	Status Status
	Detail string
}

// --- pure decision logic (unit-tested, no I/O) ------------------------------

// MicConcern classifies the system input (microphone) volume, 0-100.
func MicConcern(vol int) (Status, string) {
	switch {
	case vol < 0:
		return Warn, "no pude leer el volumen de entrada del micrófono"
	case vol < 20:
		return Fail, fmt.Sprintf("volumen de entrada %d/100: demasiado bajo, casi no se te oirá. Súbelo a ~70-85 en Ajustes del Sistema → Sonido → Entrada", vol)
	case vol < 45:
		return Warn, fmt.Sprintf("volumen de entrada %d/100: algo bajo. Considera subirlo a ~70-85", vol)
	default:
		return OK, fmt.Sprintf("volumen de entrada %d/100", vol)
	}
}

// VoiceConcern classifies the configured TTS voice against the installed set.
func VoiceConcern(want string, installed []string) (Status, string) {
	want = strings.TrimSpace(want)
	if want == "" {
		return OK, "voz por defecto del sistema (define 'voice' para una voz consistente)"
	}
	for _, v := range installed {
		if strings.EqualFold(strings.TrimSpace(v), want) {
			return OK, fmt.Sprintf("voz %q instalada (si suena robótica, instala su versión Enhanced en Ajustes → Accesibilidad → Contenido hablado → Gestionar voces)", want)
		}
	}
	return Warn, fmt.Sprintf("voz %q no está instalada; se usará la voz por defecto. Instálala en Ajustes → Accesibilidad → Contenido hablado → Gestionar voces", want)
}

// parseVoiceList extracts voice names from `say -v ?` output. Lines look like:
//
//	Mónica              es_ES    # Hola, me llamo Mónica.
//
// Names may contain spaces, so we cut at the locale token (xx_XX).
func parseVoiceList(raw string) []string {
	locale := regexp.MustCompile(`[a-z]{2}_[A-Z]{2}`)
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		loc := locale.FindStringIndex(line)
		if loc == nil {
			continue
		}
		name := strings.TrimSpace(line[:loc[0]])
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// parseInputVolume reads the integer from `input volume of (get volume settings)`.
func parseInputVolume(raw string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(raw))
}

// --- orchestration (shells out via the adapter) -----------------------------

// Run gathers all checks for the active profile. It never panics; a gather
// failure becomes a Warn so the report is always complete.
func Run(ctx context.Context, cfg *agent.Config, osa osadapter.Adapter) []Check {
	prof := cfg.Active()
	var checks []Check

	// Brain credentials.
	checks = append(checks, brainKeyCheck(prof.Brain))

	// Audio playback backend (used for TTS file playback + earcons).
	if _, err := exec.LookPath("afplay"); err != nil {
		checks = append(checks, Check{"afplay", Fail, "no se encontró afplay; el audio no sonará (debería venir con macOS)"})
	} else {
		checks = append(checks, Check{"afplay", OK, "disponible"})
	}

	// TTS voice.
	if prof.Voice.TTS.Provider == "macos_say" || prof.Voice.TTS.Provider == "" {
		raw, err := osa.RunShell(ctx, "say -v '?'")
		if err != nil {
			checks = append(checks, Check{"voz (TTS)", Warn, "no pude listar las voces instaladas: " + err.Error()})
		} else {
			st, detail := VoiceConcern(prof.Voice.TTS.Voice, parseVoiceList(raw))
			checks = append(checks, Check{"voz (TTS)", st, detail})
		}
	}

	// Microphone input volume.
	if raw, err := osa.RunAppleScript(ctx, "input volume of (get volume settings)"); err != nil {
		checks = append(checks, Check{"micrófono", Warn, "no pude leer el volumen de entrada: " + err.Error()})
	} else if vol, perr := parseInputVolume(raw); perr != nil {
		checks = append(checks, Check{"micrófono", Warn, "volumen de entrada ilegible: " + strings.TrimSpace(raw)})
	} else {
		st, detail := MicConcern(vol)
		checks = append(checks, Check{"micrófono", st, detail})
	}

	// STT model + binaries (only for whisper profiles).
	if prof.Voice.STT.Provider == "whisper_cpp" {
		checks = append(checks, whisperChecks(prof.Voice.STT.Whisper)...)
	}

	return checks
}

func brainKeyCheck(b agent.BrainConfig) Check {
	switch b.Provider {
	case "openai":
		if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
			return Check{"clave del cerebro", Fail, "OPENAI_API_KEY no está definida"}
		}
		return Check{"clave del cerebro", OK, "OPENAI_API_KEY presente"}
	case "anthropic":
		if strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) == "" {
			return Check{"clave del cerebro", Fail, "ANTHROPIC_API_KEY no está definida"}
		}
		return Check{"clave del cerebro", OK, "ANTHROPIC_API_KEY presente"}
	case "ollama":
		return Check{"clave del cerebro", OK, "Ollama local (sin clave)"}
	default:
		return Check{"clave del cerebro", OK, fmt.Sprintf("proveedor %q (sin chequeo de clave)", b.Provider)}
	}
}

func whisperChecks(w agent.WhisperSTTConfig) []Check {
	var out []Check
	model := expandHome(w.ModelPath)
	if model == "" {
		out = append(out, Check{"modelo STT", Fail, "model_path vacío"})
	} else if _, err := os.Stat(model); err != nil {
		out = append(out, Check{"modelo STT", Fail, fmt.Sprintf("no encuentro el modelo whisper en %s", model)})
	} else {
		out = append(out, Check{"modelo STT", OK, model})
	}

	sox := orDefault(w.SOXBin, "sox")
	if _, err := exec.LookPath(sox); err != nil {
		out = append(out, Check{"sox (micrófono)", Fail, "no se encontró sox; instálalo con: brew install sox"})
	} else {
		out = append(out, Check{"sox (micrófono)", OK, "disponible"})
	}

	wc := orDefault(w.WhisperBin, "whisper-cli")
	if _, err := exec.LookPath(wc); err != nil {
		out = append(out, Check{"whisper-cli", Fail, "no se encontró whisper-cli; instálalo con: brew install whisper-cpp"})
	} else {
		out = append(out, Check{"whisper-cli", OK, "disponible"})
	}
	return out
}

// Report renders the checks as a human-readable block and returns it along
// with the number of hard failures.
func Report(checks []Check) (text string, fails int) {
	var b strings.Builder
	b.WriteString("Chequeo de preparación (doctor):\n")
	for _, c := range checks {
		if c.Status == Fail {
			fails++
		}
		b.WriteString(fmt.Sprintf("  %s %-18s %s\n", c.Status.icon(), c.Name, c.Detail))
	}
	if fails == 0 {
		b.WriteString("\nTodo listo. Puedes iniciar el agente.\n")
	} else {
		b.WriteString(fmt.Sprintf("\n%d problema(s) que impiden funcionar bien. Corrígelos y vuelve a correr --doctor.\n", fails))
	}
	return b.String(), fails
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + strings.TrimPrefix(p, "~")
		}
	}
	return p
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
