# Fase 0.2 — Voz real con whisper.cpp

> Instrucciones para una IA externa (Codex u otra) operando en la Mac mini
> M4 del usuario. Misma autorización y reglas que `CODEX_HANDOFF.md`:
> no commits, no push, no leer la API key, respetar permisos de macOS.

## Qué cambia respecto a fase 0.1

Hasta ahora el agente leía lo que escribías por teclado. En fase 0.2 el
agente escucha por el micrófono y transcribe localmente con whisper.cpp.
La salida (voz) sigue siendo `say` de macOS. Costo extra: $0 — todo el
procesamiento de audio es local.

Componentes nuevos:
- `sox`: graba del micrófono con detección de silencio.
- `whisper.cpp` (binario `whisper-cli`): transcribe el WAV a texto.
- `internal/stt/whisper.go`: pega los dos comandos.
- `internal/activator/enter.go`: dispara la grabación por cada Enter.

## Paso 1 — Pull del código de fase 0.2

```bash
cd ~/my_prj
git pull origin claude/voice-mac-agent-V98uX
git log --oneline -1
```

Debe mostrar el commit más reciente con "fase 0.2" o "whisper" en el
mensaje.

## Paso 2 — Instalar dependencias

```bash
brew install sox whisper-cpp
sox --version | head -1
whisper-cli --help | head -5
```

Ambos deben responder sin error. Si `whisper-cli` no se encuentra,
verifica con `brew --prefix whisper-cpp` que la fórmula esté instalada,
y mira `ls "$(brew --prefix)/bin" | grep whisper` por si el binario
tiene otro nombre.

## Paso 3 — Descargar un modelo whisper

Recomendado: `small` (multilingüe, ~480 MB, excelente balance en M4).

```bash
mkdir -p ~/.whisper-models
curl -L --progress-bar \
  -o ~/.whisper-models/ggml-small.bin \
  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
ls -lh ~/.whisper-models/ggml-small.bin
```

Debe pesar ~480 MB. Si quedó muy chico, la descarga se interrumpió;
borra y reintenta.

## Paso 4 — Cambiar al perfil "voice" del config

```bash
sed -i.bak 's/^active_profile: .*/active_profile: voice/' config.yaml
head -3 config.yaml
rm -f config.yaml.bak
```

Debe mostrar `active_profile: voice`. Si tu `config.yaml` aún no tiene
el perfil `voice`, regenéralo:

```bash
cp config.example.yaml config.yaml
sed -i.bak 's/^active_profile: .*/active_profile: voice/' config.yaml
rm -f config.yaml.bak
```

## Paso 5 — Permitir micrófono a Terminal

Antes de correr, abre **Ajustes del Sistema → Privacidad y Seguridad →
Micrófono** y verifica que **Terminal** esté en la lista con el
interruptor activo. Si no aparece, la primera ejecución de `sox -d`
disparará el diálogo de permiso — concédelo y reintenta.

## Paso 6 — Smoke test de voz

```bash
go run ./cmd/agent --config config.yaml -v 2>&1 | tee voice.log
```

Salida esperada al arrancar:
```
config.loaded profile=voice
brain.ready name=openai:gpt-5
cost.tracker.ready path=cost.log
tools.registered count=4
voice.ready name="pipeline(stt=whisper_cpp,tts=macos_say,act=enter)"
Agente listo. ...
[Enter para hablar]
```

Cuando veas `[Enter para hablar]`, presiona Enter y di en voz clara:

> **"Abre Google Chrome"**

Debe pasar lo siguiente:
1. Aparece `🎤 escuchando...`.
2. Cuando termines de hablar y haya ~1.5 s de silencio, `sox` corta y
   pasa el WAV a whisper-cli.
3. Whisper transcribe (toma 1-3 s en M4).
4. Aparece una línea `brain.response` en el log.
5. Chrome se abre y oyes la voz confirmando.
6. Vuelve `[Enter para hablar]`.

Si nada se graba o whisper devuelve vacío:
- Verifica que estés cerca del micrófono.
- Sube `threshold` a `"5%"` o `"10%"` en `config.yaml → voice → voice →
  stt → whisper → threshold` (entornos ruidosos).
- Ajusta `silence_seconds` si el corte de fin es muy abrupto (sube a 2.0)
  o muy lento (baja a 1.0).

Prueba al menos 2 utterances distintas, p. ej.:
- "Abre Google Chrome"
- "Qué hora es"
- "Cuánto llevo gastado hoy"

Ctrl+C para salir.

## Paso 7 — Reporte

Cuéntale al usuario:
- ✅/❌ instalación de `sox` y `whisper-cli`.
- ✅/❌ descarga del modelo (tamaño obtenido).
- ✅/❌ permiso de micrófono concedido.
- Para cada utterance probada: lo que dijiste, lo que whisper transcribió
  (sale en `voice.log` como un mensaje `user` antes de `brain.response`),
  si la acción se ejecutó.
- Costo total (suma de `usd=` en `voice.log`).
- Cualquier error tal cual aparece en pantalla.

## Diagnóstico rápido

| Síntoma | Causa probable | Acción |
|---|---|---|
| `sox: command not found` | brew falló | `brew install sox` y revisar PATH |
| `whisper-cli: command not found` | brew formula con otro nombre | `brew install whisper-cpp` y `ls $(brew --prefix)/bin/whisper*` |
| `whisper preflight: ...model not found` | ruta incorrecta o descarga rota | revisa `ls -lh ~/.whisper-models/` |
| `no speech detected` repetido | mic muy lejos o ruido | aumenta `threshold`; o di algo más fuerte |
| Diálogo "Terminal quiere acceder al micrófono" | macOS pide permiso | clic Permitir, reintentar |
| Transcripción en idioma equivocado | lenguaje auto se confundió | fija `language: es` (o `en`) en config |
| Transcripción incluye `[Música]` u otros corchetes | whisper detectó silencio | el código ya los limpia; verifica que el binario sea reciente |

## No hacer

- ❌ Borrar `~/.whisper-models/` sin avisar.
- ❌ Cambiar el binario `whisper-cli` por `main` o por una versión vieja
  sin verificar compatibilidad de flags (`-nt`, `-np`, `-l`).
- ❌ Commits ni push.
- ❌ Tocar el `.env`.

## Si todo verde

Reporta al usuario. La siguiente fase (0.3) reemplazará el Enter por
un hotkey global tipo ⌃⌥Espacio para que ni siquiera tengas que estar
en la terminal — sea ciega o sin manos, la persona simplemente toca
una tecla (o usa un switch externo conectado por USB) y habla.
