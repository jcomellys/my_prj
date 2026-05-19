# Codex — Fase 0.3.2: activadores físicos (hotkey global + USB)

> Esta guía está escrita para que la sigas tú, Codex, sobre la Mac mini
> M4 del usuario. Yo (Claude) la dejé lista antes de quedarme sin
> tokens. Si todo va bien, el resultado es **dos nuevos activadores**
> que dejan al usuario manos-libres real (especialmente útil para
> personas sin manos o sin teclado).

## Autorización para esta fase

Para esta tarea SÍ tienes permiso para:
- Modificar archivos en `~/my_prj`.
- Crear ramas y `git commit` (mismo branch `claude/voice-mac-agent-V98uX`).
- Hacer `git push -u origin claude/voice-mac-agent-V98uX`.
- Instalar dependencias Go (`go get`, `go mod tidy`).

NO está permitido todavía:
- Cambiar a otra rama remota / hacer merge.
- Modificar `master`.
- Tocar `.env` o la API key.
- Cambiar arquitectura general (interfaces ya definidas en `internal/activator`, `internal/voice`).
- Borrar tests existentes (puedes agregar).

## Objetivo de la fase

Añadir dos `Activator` nuevos a `internal/activator/`:

1. **`Hotkey`**: activa el agente cuando el usuario presiona una
   combinación global del sistema (default ⌃⌥Espacio = Ctrl+Option+Space).
   Funciona desde cualquier app, no requiere estar en Terminal.
2. **`USBPresence`**: activa el agente cuando aparece un volumen USB con
   un nombre conocido (e.g. `/Volumes/AGENT`). Pensado para usuarios sin
   manos: enchufan el USB, hablan, sacan el USB para "apagar".

Cada uno implementa la interfaz `activator.Activator` que ya existe.

## Criterios de aceptación

- [ ] `go test -race -count=1 ./...` pasa en linux y darwin.
- [ ] `GOOS=darwin GOARCH=arm64 go build ./...` pasa.
- [ ] El usuario puede ejecutar el agente con `activator.kind: hotkey`
      en config y activarlo presionando ⌃⌥Espacio desde cualquier app.
- [ ] El usuario puede ejecutar con `activator.kind: usb_presence` y
      activarlo enchufando un USB llamado `AGENT`. Sacar el USB no
      mata el agente; reinsertarlo dispara la siguiente activación.
- [ ] El `config.example.yaml` tiene un nuevo perfil `manos_libres`
      que combina hotkey + voz + GPT-5.
- [ ] Documentación actualizada en `docs/ROADMAP.md` y `docs/HANDOFF.md`.
- [ ] Un único commit con mensaje del template al final.

## Dependencias nuevas

Solo una, para hotkey global en macOS:

```bash
cd ~/my_prj
go get golang.design/x/hotkey@latest
go mod tidy
```

Esto añade `golang.design/x/hotkey` y sus dependencias indirectas
(`golang.design/x/mainthread` para correr el hotkey loop en el main
thread, requisito de Cocoa). En Mac usa CGO (`-framework Cocoa`),
preinstalado con Xcode Command Line Tools.

## Archivos a crear

### 1. `internal/activator/usb_presence.go` (sin build tag — funciona en todos los SO)

```go
package activator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// USBPresence activates each time a named USB volume appears on the
// system. The first time the volume is detected the activator returns;
// subsequent calls require the USB to be removed and reinserted before
// activating again — edge-triggered, not level-triggered. This lets a
// user without hand mobility "talk" by inserting their USB, and "stop"
// the next turn by removing it.
//
// On macOS, mounted volumes appear under /Volumes/<name>. On Linux the
// path is typically /media/<user>/<name> or /mnt/<name>; we accept any
// of those — caller can override via Paths.
type USBPresence struct {
	// VolumeName is the case-sensitive name of the USB volume that
	// signals activation. Default "AGENT".
	VolumeName string

	// Paths to scan. Default: ["/Volumes", "/media", "/mnt"].
	Paths []string

	// PollInterval between filesystem checks. Default 500ms.
	PollInterval time.Duration

	// wasPresent tracks the prior state so we only fire on rising edge.
	wasPresent bool
}

func NewUSBPresence(volumeName string) *USBPresence {
	return &USBPresence{
		VolumeName:   volumeName,
		Paths:        []string{"/Volumes", "/media", "/mnt"},
		PollInterval: 500 * time.Millisecond,
	}
}

func (USBPresence) Name() string { return "usb_presence" }

func (u *USBPresence) WaitForActivation(ctx context.Context) error {
	if u.VolumeName == "" {
		return fmt.Errorf("usb_presence: VolumeName is empty")
	}
	interval := u.PollInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Wait for the volume to be ABSENT before listening for the rising
	// edge — if the USB is already plugged in when WaitForActivation
	// is called for the second turn, we shouldn't double-trigger.
	if u.wasPresent {
		for {
			if !u.present() {
				u.wasPresent = false
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
	}

	// Now wait for the rising edge: not present -> present.
	for {
		if u.present() {
			u.wasPresent = true
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (u *USBPresence) present() bool {
	for _, base := range u.Paths {
		// On Linux /media/<user>/<NAME>; on Mac /Volumes/<NAME>. Match
		// both shapes by checking direct child AND one-level-deep.
		direct := filepath.Join(base, u.VolumeName)
		if _, err := os.Stat(direct); err == nil {
			return true
		}
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidate := filepath.Join(base, e.Name(), u.VolumeName)
			if _, err := os.Stat(candidate); err == nil {
				return true
			}
		}
	}
	return false
}
```

### 2. `internal/activator/usb_presence_test.go`

```go
package activator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUSBPresence_RisingEdge(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "AGENT")

	u := &USBPresence{
		VolumeName:   "AGENT",
		Paths:        []string{tmp},
		PollInterval: 20 * time.Millisecond,
	}

	// Spawn a goroutine that creates the volume after ~50ms.
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.MkdirAll(target, 0o755)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := u.WaitForActivation(ctx); err != nil {
		t.Fatalf("WaitForActivation: %v", err)
	}

	// Second call must NOT fire while volume is still present.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()
	err := u.WaitForActivation(ctx2)
	if err == nil {
		t.Fatal("expected timeout: USB still inserted, no edge")
	}

	// Remove the volume, then re-create it. The second activation should fire.
	if err := os.RemoveAll(target); err != nil {
		t.Fatalf("remove: %v", err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.MkdirAll(target, 0o755)
	}()
	ctx3, cancel3 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel3()
	if err := u.WaitForActivation(ctx3); err != nil {
		t.Fatalf("second activation failed: %v", err)
	}
}

func TestUSBPresence_RequiresVolumeName(t *testing.T) {
	u := &USBPresence{}
	err := u.WaitForActivation(context.Background())
	if err == nil {
		t.Fatal("expected error when VolumeName empty")
	}
}
```

### 3. `internal/activator/hotkey_darwin.go` (con build tag darwin)

```go
//go:build darwin

package activator

import (
	"context"
	"fmt"
	"strings"

	"golang.design/x/hotkey"
	"golang.design/x/mainthread"
)

// Hotkey is a global system hotkey activator. On macOS this uses
// Cocoa via cgo (provided by golang.design/x/hotkey). Each press of
// the combination returns from WaitForActivation; the agent then
// records one voice turn.
//
// Important macOS quirk: the hotkey library MUST run on the main OS
// thread (Cocoa requirement). We use golang.design/x/mainthread for
// that. Because of this, main.go imports a helper to wrap main().
type Hotkey struct {
	// Combo is the textual representation, e.g. "ctrl+option+space".
	// Parsed in Register().
	Combo string

	hk     *hotkey.Hotkey
	keydown <-chan hotkey.Event
}

func NewHotkey(combo string) *Hotkey {
	if combo == "" {
		combo = "ctrl+option+space"
	}
	return &Hotkey{Combo: combo}
}

func (Hotkey) Name() string { return "hotkey" }

// Register parses the combo string and registers the hotkey with the
// OS. Call once at startup, BEFORE the conversation loop starts.
func (h *Hotkey) Register() error {
	mods, key, err := parseCombo(h.Combo)
	if err != nil {
		return err
	}
	hk := hotkey.New(mods, key)
	if err := hk.Register(); err != nil {
		return fmt.Errorf("hotkey register %q: %w", h.Combo, err)
	}
	h.hk = hk
	h.keydown = hk.Keydown()
	return nil
}

func (h *Hotkey) WaitForActivation(ctx context.Context) error {
	if h.hk == nil {
		if err := h.Register(); err != nil {
			return err
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-h.keydown:
		return nil
	}
}

// parseCombo turns "ctrl+option+space" into the hotkey library's
// modifier slice + Key. Supports: ctrl, option (alt), shift, cmd
// (super), and letter/space/enter keys.
func parseCombo(combo string) ([]hotkey.Modifier, hotkey.Key, error) {
	parts := strings.Split(strings.ToLower(combo), "+")
	if len(parts) < 2 {
		return nil, 0, fmt.Errorf("hotkey combo needs at least one modifier and one key: %q", combo)
	}
	var mods []hotkey.Modifier
	var key hotkey.Key
	for i, p := range parts {
		p = strings.TrimSpace(p)
		last := i == len(parts)-1
		if last {
			switch p {
			case "space":
				key = hotkey.KeySpace
			case "enter", "return":
				key = hotkey.KeyReturn
			default:
				if len(p) == 1 && p[0] >= 'a' && p[0] <= 'z' {
					key = hotkey.Key(hotkey.KeyA) + hotkey.Key(p[0]-'a')
				} else {
					return nil, 0, fmt.Errorf("unsupported key %q", p)
				}
			}
			continue
		}
		switch p {
		case "ctrl", "control":
			mods = append(mods, hotkey.ModCtrl)
		case "option", "alt":
			mods = append(mods, hotkey.ModOption)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "cmd", "command", "super":
			mods = append(mods, hotkey.ModCmd)
		default:
			return nil, 0, fmt.Errorf("unsupported modifier %q", p)
		}
	}
	return mods, key, nil
}

// RunWithMainThread wraps the agent's main loop so the macOS Cocoa
// event pump can run on the OS main thread. Pass the agent's main
// loop function; this blocks until it returns.
func RunWithMainThread(loop func()) {
	mainthread.Init(loop)
}
```

### 4. `internal/activator/hotkey_other.go` (build tag NOT darwin — stub)

```go
//go:build !darwin

package activator

import (
	"context"
	"fmt"
)

// Hotkey is a stub on non-darwin platforms so the rest of the code
// compiles in CI. The real implementation lives in hotkey_darwin.go.
type Hotkey struct {
	Combo string
}

func NewHotkey(combo string) *Hotkey { return &Hotkey{Combo: combo} }

func (Hotkey) Name() string { return "hotkey" }

func (Hotkey) Register() error {
	return fmt.Errorf("hotkey activator only supported on darwin in fase 0.3.2")
}

func (h *Hotkey) WaitForActivation(ctx context.Context) error {
	return fmt.Errorf("hotkey activator only supported on darwin")
}

// RunWithMainThread on non-darwin just calls loop directly — no main
// thread routing needed.
func RunWithMainThread(loop func()) { loop() }
```

## Archivos a MODIFICAR

### 5. `internal/agent/config.go`

Añade un struct para configuración del activator detallada:

```go
type ActivatorConfig struct {
	Kind     string            `yaml:"kind"` // stdin | enter | hotkey | usb_presence
	Hotkey   HotkeyConfig      `yaml:"hotkey"`
	USB      USBPresenceConfig `yaml:"usb"`
}

type HotkeyConfig struct {
	Combo string `yaml:"combo"` // default "ctrl+option+space"
}

type USBPresenceConfig struct {
	VolumeName string `yaml:"volume_name"` // default "AGENT"
}
```

Borra el `ActivatorConfig` viejo (que solo tenía `Kind`). Asegúrate
de no romper los YAML existentes — los campos `Hotkey` y `USB` son
opcionales.

### 6. `cmd/agent/main.go`

En `buildVoice`, dentro del switch sobre `ac.Kind`, añade dos casos:

```go
case "hotkey":
	hk := activator.NewHotkey(ac.Hotkey.Combo)
	if err := hk.Register(); err != nil {
		return nil, fmt.Errorf("hotkey: %w", err)
	}
	actImpl = hk
case "usb_presence":
	name := ac.USB.VolumeName
	if name == "" {
		name = "AGENT"
	}
	actImpl = activator.NewUSBPresence(name)
```

Y al final de `main()`, envuelve la llamada a `orch.Run(ctx)` para
correr en el main thread (necesario para hotkey en macOS):

```go
// Antes:
// if err := orch.Run(ctx); err != nil && err != context.Canceled {
//     fatal(log, err)
// }

// Después:
runErr := make(chan error, 1)
activator.RunWithMainThread(func() {
	runErr <- orch.Run(ctx)
})
if err := <-runErr; err != nil && err != context.Canceled {
	fatal(log, err)
}
```

### 7. `config.example.yaml`

Añade un nuevo perfil completo, justo después del perfil `voice`:

```yaml
  # Tier — manos libres: hotkey global ⌃⌥Espacio + voz + GPT-5.
  # El usuario presiona Ctrl+Option+Espacio en cualquier app,
  # habla, y el agente actúa. Ideal para personas sin manos
  # con un switch USB que mande esa combinación.
  manos_libres:
    voice:
      mode: pipeline
      stt:
        provider: whisper_cpp
        whisper:
          model_path: ~/.whisper-models/ggml-small.bin
          language: es
          silence_seconds: 1.5
          threshold: "3%"
      tts:
        provider: macos_say
    brain:
      provider: openai
      model: gpt-5
      temperature: 0
      max_tokens: 4096
    activator:
      kind: hotkey
      hotkey:
        combo: "ctrl+option+space"

  # Tier — USB como interruptor físico.
  usb_switch:
    voice:
      mode: pipeline
      stt:
        provider: whisper_cpp
        whisper:
          model_path: ~/.whisper-models/ggml-small.bin
          language: es
      tts:
        provider: macos_say
    brain:
      provider: openai
      model: gpt-5
      max_tokens: 4096
    activator:
      kind: usb_presence
      usb:
        volume_name: AGENT
```

### 8. `docs/ROADMAP.md`

Marca fase 0.3.2 como hecha (sin tachar 0.3.3 que falta):

```markdown
## Fase 0.3.2 — Activadores físicos

- [x] HotkeyActivator usando golang.design/x/hotkey con build tag darwin.
- [x] Stub hotkey_other.go para que CI en linux compile.
- [x] USBPresenceActivator (edge-triggered): activación al insertar
      un USB con nombre conocido (default AGENT). Funcionamiento
      universal (Mac/Linux), pure Go, sin CGO.
- [x] main.go envuelve la run loop con RunWithMainThread para que
      Cocoa pueda correr en el OS main thread (requisito de hotkey).
- [x] Dos perfiles nuevos en config.example.yaml: manos_libres
      (hotkey) y usb_switch (USB).
- [ ] Validación en la Mac mini (pendiente).

**Exit criterion:** El usuario presiona ⌃⌥Espacio desde Chrome (no
desde Terminal), habla "abre Mensajes", y el agente actúa. El
usuario inserta un USB llamado AGENT, habla, lo retira, lo reinserta,
otra activación dispara.
```

### 9. `docs/HANDOFF.md`

Añade una línea bajo "Current state" mencionando fase 0.3.2.

## Build, test, push — protocolo exacto

```bash
cd ~/my_prj
git pull origin claude/voice-mac-agent-V98uX

# 1. Añade dependencia.
go get golang.design/x/hotkey@latest
go mod tidy

# 2. Verifica compila en linux (lo que tu CI haría).
go build ./...

# 3. Verifica compila en darwin (esto es lo que correrá en la Mac).
GOOS=darwin GOARCH=arm64 go build ./...

# 4. Tests.
go test -race -count=1 ./...
```

Si todo pasa, commit con este mensaje EXACTO:

```
Fase 0.3.2: hotkey global + activador USB

Two new activators implementing the existing activator.Activator
interface, unlocking real hands-free use for users without mobility:

- Hotkey: registers a global system hotkey (default Ctrl+Option+Space)
  on macOS via golang.design/x/hotkey. Works from any app, not just
  Terminal. Wraps the orchestrator's Run loop in mainthread.Init so
  the Cocoa event pump can run on the OS main thread (Cocoa
  requirement). Stub on non-darwin so CI in linux compiles.
- USBPresence: edge-triggered detection of a named USB volume
  (default AGENT). Insert USB → next turn fires. Remove + reinsert
  → next turn fires. Pure Go, no cgo, works on Mac/Linux. Lets a
  user without hands "speak" by physically touching a USB key.

main.go now exposes RunWithMainThread; on non-darwin it's a passthrough.
config.example.yaml ships two new profiles: manos_libres (hotkey) and
usb_switch (USB).

Tests: USB rising-edge + empty-volume-name guard. Hotkey is exercised
manually on Mac (no automatable global key press in Go test harness).

https://claude.ai/code/session_015ac8EJSAqxuH5gH9E3tEcz
```

Después push:

```bash
git push -u origin claude/voice-mac-agent-V98uX
```

## Verificación en la Mac (haz tú, Codex)

1. Apaga cualquier instancia del agente que esté corriendo.
2. Cambia `config.yaml`:
   ```bash
   sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.yaml
   rm config.yaml.bak
   ```
3. Ejecuta `go run ./cmd/agent --config config.yaml -v 2>&1 | tee hotkey.log` y déjalo corriendo en una terminal.
4. **Abre Chrome u otra app cualquiera. Presiona Ctrl+Option+Espacio.**
   Debe aparecer "🎤 escuchando..." en la terminal. Habla "abre Mensajes". Verifica que la acción ocurra.
5. La primera vez macOS puede pedir Permiso de Accesibilidad para Terminal — concédelo en Ajustes → Privacidad y Seguridad → Accesibilidad. Reintenta.
6. Ctrl+C para salir.
7. Cambia `active_profile: usb_switch` y reinicia.
8. Crea un volumen falso para probar sin USB real:
   ```bash
   sudo mkdir -p /Volumes/AGENT
   ```
   (Si pide password de Mac, dáselo al usuario para que lo apruebe).
   Debe activar de inmediato.
9. `sudo rm -rf /Volumes/AGENT` para "sacar el USB".
10. `sudo mkdir -p /Volumes/AGENT` de nuevo — debe disparar otra activación.

Reporta al usuario:
- ✅/❌ cada criterio de aceptación.
- ✅/❌ hotkey funcionó desde otra app (no terminal).
- ✅/❌ USB activator detectó rising edges.
- Cualquier error tal cual.

## Si algo falla

| Síntoma | Causa probable | Acción |
|---|---|---|
| `go get` falla, "module not found" | Conectividad | Reintenta |
| Build linux falla por cgo / Cocoa | Build tag mal puesto | Revisa los `//go:build darwin` / `//go:build !darwin` |
| Build darwin falla "framework not found" | Xcode CLT faltan | `xcode-select --install` |
| Hotkey no responde | Sin permiso de Accesibilidad | Ajustes → Privacidad → Accesibilidad → marca Terminal |
| Hotkey responde pero el loop se cuelga | Olvidaste RunWithMainThread | Revisa el bloque en main.go |
| USB activator no detecta | Permiso de FS o ruta distinta | Verifica `ls /Volumes`, ajusta `usb.volume_name` |

## Si no estás seguro de algo

NO improvises arquitectura. Pega al usuario el bloque que te
confunde + tu mejor interpretación + pregunta antes de continuar.

Suerte. El usuario está esperando este avance para gente que lo necesita.
