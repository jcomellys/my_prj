# Instrucciones para una IA externa (Codex u otra) — fase 0.1 setup

> Si eres una IA leyendo este archivo, este documento es para ti. Si eres
> un humano, este documento describe paso a paso lo que la IA debe hacer
> en tu Mac, y puedes revisarlo antes o después.

## Tu rol y autorización

Eres un asistente operando en la Mac mini M4 (macOS Tahoe / macOS 26) del
usuario **jcomellys**. Tu trabajo en esta sesión es **dejar el repositorio
`voice-mac-agent` corriendo en modo de prueba** y reportar el resultado.

**Está autorizado:**
- Instalar Homebrew y dependencias (`go`, `git`).
- Clonar el repositorio, hacer `git pull` y `git checkout` de ramas
  existentes.
- Crear archivos locales `.env`, `config.yaml`, `config.test.yaml`
  (todos están en `.gitignore`).
- Ejecutar `make`, `go build`, `go test`, `go run`.
- Abrir aplicaciones (Chrome, TextEdit) como parte de la prueba.
- Aprobar diálogos de macOS sobre permisos de Automatización/Accesibilidad
  cuando aparezcan (preguntar al usuario antes si no estás seguro).

**NO está autorizado sin confirmación explícita del usuario:**
- `git commit`, `git push`, ni crear ramas nuevas.
- Modificar código fuente del repo (solo archivos locales `.env`/`config.yaml`).
- Cambiar `master` o cualquier rama remota.
- Eliminar archivos del usuario fuera del repo `~/my_prj`.
- Llamar a APIs de pago en bucle (la prueba con GPT-5 hace ~3 llamadas, no
  20).
- Hacer `sudo` para cualquier cosa que no sea la instalación oficial de
  Homebrew.

## Contexto rápido del proyecto

`voice-mac-agent` es un agente de voz que controla la Mac para ayudar a
personas con discapacidad (ceguera, movilidad reducida) a usar la
computadora hablando. Diseño por capas con cerebro intercambiable
(OpenAI/Anthropic/Ollama). Lee `docs/HANDOFF.md` y `docs/ARCHITECTURE.md`
para profundizar, pero **para esta tarea no es necesario**.

El estado actual es **fase 0.1**: skeleton funcional, sin captura de
audio aún. La "STT" temporal es stdin: el usuario teclea y el agente
responde con `say` (TTS nativo de macOS).

## Manejo de la API key — REGLA INVIOLABLE

El usuario tiene una `OPENAI_API_KEY`. Tu trabajo con esa key:

1. **Pídele al usuario que la pegue en `.env` él mismo** (con `open -e
   .env` en TextEdit). NO le pidas que te la diga.
2. Si por alguna razón ya la tienes en contexto, **nunca la escribas en
   logs, en commits, en archivos versionados, en mensajes al usuario,
   ni en mensajes a otros sistemas.**
3. Si vas a ejecutar comandos que la lean, hazlo solo desde el binario
   compilado o `go run`. No la hagas `echo` ni `cat .env`.
4. Si crees que se filtró, dile al usuario inmediatamente para que la
   rote en https://platform.openai.com/api-keys.

## Protocolo de reporte

Después de cada paso, dile al usuario en una línea:
`✅ paso N OK` o `❌ paso N falló: <error breve>`.

Si algo falla:
1. NO intentes "arreglar" cambios de código.
2. Pega los últimos ~20 renglones de salida.
3. Pregunta al usuario si seguir o detenerse.

Al final, reporta:
- Cuántos pasos completaste.
- Si la prueba mock funcionó (Chrome abrió + voz).
- Si la prueba real con GPT-5 funcionó.
- Si hubo costo, cuánto (sale en logs como `usd=...`).

## Pasos

### Paso 1 — Verificar punto de partida

```bash
sw_vers          # debe mostrar macOS 26.x (Tahoe)
uname -m         # debe mostrar arm64
whoami           # debe ser el usuario, no root
pwd              # debes estar en algún sitio sensato; iremos a ~
```

Si `uname -m` muestra `x86_64`, esto NO es la Mac mini M4 esperada. Detente
y pregunta al usuario.

### Paso 2 — Instalar Homebrew (si falta)

```bash
which brew
```

Si responde una ruta (típicamente `/opt/homebrew/bin/brew`), salta al paso
3.

Si no:

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

Esto le pedirá al usuario su password de Mac (no la tuya, no la de OpenAI;
la del macOS local). El usuario debe escribirla; tú no.

Después agrega Homebrew al PATH (necesario en Apple Silicon):

```bash
echo >> ~/.zprofile
echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
eval "$(/opt/homebrew/bin/brew shellenv)"
brew --version
```

Si `brew --version` muestra `Homebrew 4.x.x`, ✅ paso 2.

### Paso 3 — Instalar Go y Git

```bash
brew install go git
go version
git --version
```

Go debe ser 1.22 o superior. Si es más viejo, dile al usuario; **no hagas
downgrade ni upgrade forzado**.

### Paso 4 — Clonar el repo

```bash
cd ~
if [ -d my_prj ]; then
  echo "Carpeta my_prj ya existe; entrando."
  cd my_prj
  git fetch origin
else
  git clone https://github.com/jcomellys/my_prj.git
  cd my_prj
fi
git checkout claude/voice-mac-agent-V98uX
git pull origin claude/voice-mac-agent-V98uX
git status
```

`git status` debe decir `nothing to commit, working tree clean`. Si hay
modificaciones locales que el usuario no esperaba, **pregúntale antes de
tocarlas**.

### Paso 5 — Verificar que compila

```bash
go build ./...
go test -race -count=1 ./...
```

Esperas: `ok` en cada paquete con tests, y `go build` silencioso.

Si falla, pega el error al usuario y detente.

### Paso 6 — Configurar .env (la API key)

```bash
[ -f .env ] || cp .env.example .env
open -e .env
```

Dile al usuario:
> "Abrí `.env` en TextEdit. Por favor cambia
> `OPENAI_API_KEY=sk-...` por tu key real, guarda con Cmd+S y cierra. **No
> me digas la key.** Cuando termines, dime 'listo'."

Espera su confirmación. Después verifica que la key se ve plausible sin
exponerla:

```bash
grep -c '^OPENAI_API_KEY=sk-' .env
```

Debe devolver `1`. Si devuelve `0`, el formato está mal — pídele al
usuario que verifique.

**NUNCA hagas `cat .env`.**

### Paso 7 — Smoke test con cerebro mock (sin gastar tokens)

```bash
cat > config.test.yaml <<'EOF'
active_profile: test
profiles:
  test:
    voice:
      mode: pipeline
      stt: { provider: stdin }
      tts: { provider: macos_say }
    brain:
      provider: mock
    activator: { kind: stdin }
tools:
  open_app: { enabled: true }
  applescript: { enabled: true }
  shell:
    enabled: true
    allow_unrestricted: false
    allowlist: [ "open " ]
cost: { enabled: true }
EOF

go run ./cmd/agent --config config.test.yaml
```

Aparece `you>`. Tu trabajo aquí:

1. Tipea `abre Google Chrome` y Enter.
2. Confirma que Chrome se abrió.
3. Confirma que oíste la voz de la Mac diciendo "Opened Google Chrome." o
   similar.
4. **Importante**: la primera vez macOS pedirá permiso de Automatización
   ("Terminal quiere controlar Google Chrome"). Pídele al usuario que dé
   click en "Permitir". Vuelve a tipear `abre Google Chrome` y revisa.
5. Ctrl+C para salir.

Reporta: ✅ si Chrome abrió Y se escuchó la voz. ❌ con el log si no.

### Paso 8 — Configurar perfil premium (GPT-5)

```bash
[ -f config.yaml ] || cp config.example.yaml config.yaml
```

Modifica solo la línea `active_profile`. Una forma segura sin abrir
editor:

```bash
sed -i.bak 's/^active_profile: .*/active_profile: premium/' config.yaml
diff config.yaml.bak config.yaml || true
rm config.yaml.bak
```

Verifica:

```bash
head -3 config.yaml
```

Debe mostrar `active_profile: premium`.

### Paso 9 — Prueba real con GPT-5 (gasta tokens — ~$0.01 esperado)

Advierte al usuario:
> "Voy a hacer 2-3 llamadas reales a OpenAI GPT-5. El costo esperado es
> menos de $0.05. ¿OK?"

Espera "sí".

```bash
go run ./cmd/agent --config config.yaml -v 2>&1 | tee run.log
```

En el prompt `you>`:
1. Tipea: `abre Terminal` y Enter. Espera la voz.
2. Tipea: `qué hora es` y Enter. Espera la voz (el modelo puede o no
   saberlo; con tal de que responda).
3. Tipea: `cuánto llevo gastado hoy` y Enter. El agente debe llamar a la
   tool `show_cost` y responder con tokens + USD.
4. Ctrl+C.

Examina `run.log` y busca líneas `brain.response`. Cada una tiene
`in_tokens=`, `out_tokens=`, `usd=`. Suma los `usd=` para el total.

### Paso 10 — Reporte final

Dile al usuario, máximo 4 líneas:

```
✅ Setup completo en macOS <versión>.
✅ Mock test: Chrome abrió + voz OK.
✅ GPT-5 test: 3 llamadas, total $X.XXXX.
Log completo en ~/my_prj/run.log.
```

Si algo falló, sustituye los ✅ por ❌ con el error.

## Si algo se rompe — guía de diagnóstico

| Síntoma | Causa probable | Acción |
|---|---|---|
| `OPENAI_API_KEY is unset` | `.env` no se cargó o key vacía | Verifica con `grep -c '^OPENAI_API_KEY=sk-' .env`; recuerda que no debes leer la key |
| `osascript` pide permiso | macOS bloquea automatización | Usuario aprueba en Ajustes → Privacidad y Seguridad → Automatización |
| `open -a "Google Chrome"` falla | Chrome no instalado o nombre distinto | `ls /Applications | grep -i chrome` |
| `say: command not found` | macOS roto (muy raro) | `xcode-select --install` |
| Errores de red al instalar `go` | Conectividad | Reintenta `brew install go` |
| `go: cannot find main module` | No estás en `~/my_prj` | `cd ~/my_prj` |
| Tests fallan en master pero no en la rama | Te quedaste en master | `git checkout claude/voice-mac-agent-V98uX` |

## No hacer nunca

- ❌ Hacer `git commit` o `git push` sin que el usuario te lo pida
  explícitamente para esta sesión.
- ❌ Tocar `~/.ssh`, `~/.aws`, `~/.config/gh`, ni ningún otro directorio
  con credenciales.
- ❌ Ejecutar `rm -rf` con rutas que no estén estrictamente dentro de
  `~/my_prj`.
- ❌ Pegar la API key en chat, logs, commits, mensajes a Slack, o
  cualquier sitio.
- ❌ Cambiar la rama activa a `master` y "limpiar" cosas.
- ❌ Hacer `brew uninstall` de paquetes que ya tenía el usuario.

## Si terminas con éxito

El usuario puede entonces pedir que avancemos a la **fase 0.2** (captura
real de audio con whisper.cpp). Eso es un trabajo separado; léelo en
`docs/ROADMAP.md` antes de empezar.

¡Buena suerte!
