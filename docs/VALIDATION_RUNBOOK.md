# Runbook de validación viva (manos_libres)

Una sola pasada en la Mac cuando GitHub responda. Objetivo: confirmar en vivo
lo que ya está en código y verde en tests, sobre todo la **latencia** (el
problema real) con números (`dur_ms`), no a ojo.

Commit objetivo: `2f44c21` o más nuevo de `claude/voice-mac-agent-V98uX`.

## 0. Traer el código (reintentar si GitHub corta)

```bash
cd ~/my_prj
git fetch origin claude/voice-mac-agent-V98uX   # reintenta si da "port 443"
git checkout claude/voice-mac-agent-V98uX
git pull origin claude/voice-mac-agent-V98uX
go test -race -count=1 ./...                     # debe quedar todo OK
```

## 1. Preparar el terreno

```bash
go run ./cmd/agent --config config.yaml --env .env --doctor
```
- Sube el micrófono a ~70–85 (Ajustes → Sonido → Entrada); confirma que la
  barra se mueve al hablar. (El doctor no lo puede leer en tu hardware.)
- Sin ❌, continúa.

Ten abiertos: un **PDF en inglés con secciones** en Vista Previa, y un **chat
web** (Claude/ChatGPT) en Chrome.

## 2. Arrancar

```bash
go run ./cmd/agent --config config.yaml --env .env -v
```

## 3. Pruebas (di cada frase, captura el log)

| # | Frase | Qué confirmar | Qué capturar del log |
|---|-------|---------------|----------------------|
| 1 | "Explícame en breve qué es un transistor" | Voz termina la última palabra; respuesta en segundos | `brain.response dur_ms=` |
| 2 | (mientras habla) hotkey | Corta en <1s | `voice.barge_in` |
| 3 | "Léeme la sección introducción" (PDF abierto) | **Lee solo esa sección, en español, rápido** | `tool.ok tool=read_open_pdf dur_ms=` ← **el número clave** |
| 4 | "Léeme el último mensaje" (Claude/ChatGPT en Chrome) | Lee solo el último, traducido | `tool.ok` + `brain.response dur_ms=` |
| 5 | "Escribe en el chat de Claude: hola, esto es una prueba" | Escribe y **pregunta "¿lo envío?"**; solo envía si dices "sí" | `tool.ok tool=type_text` |

## 4. Veredicto

- **Latencia**: ¿`read_open_pdf dur_ms` es de ~1–2 s (no 87 000)? Ese es el
  fix real. Pega esa línea.
- Si `read_open_pdf` devuelve "no encontré un PDF abierto", pégalo: hay una
  segunda ruta lista (vía `System Events` por título de ventana).
- Anota cualquier `turn.error`, corte de voz, o lectura de sección incorrecta.

## Notas
- No leer `.env` real; usar `/tmp/bad.env` solo para la prueba de resiliencia.
- Permiso de Accesibilidad ya concedido (hotkey) cubre `type_text`.
- `read_open_pdf` usa `lsof`, NO scriptea Vista Previa: no debe aparecer el
  diálogo de Automatización ni el cuelgue de ~87 s.

## Actualización tras X-017 (2026-06-11): cerebro rápido para fast-paths

Dato vivo X-017: el fast-path estructural funciona (read_open_pdf 145 ms,
una sola `brain.response`, "Introduction" leído en español), pero el E2E fue
5731 ms porque la vuelta de respuesta corrió en gpt-5 (5584 ms). El cuello
ya no es la tool: es el modelo que traduce/narra.

**Cambio:** `manos_libres` ahora trae `brain.fast` = gpt-5-mini con
`reasoning_effort: minimal`. Cuando el fast-path dispara, SOLO esa vuelta de
respuesta corre en el modelo rápido. Logs esperados, en orden:

```
fastpath.ok tool=read_open_pdf dur_ms≈100-200
brain.fastpath brain=openai:gpt-5-mini
brain.response brain=openai:gpt-5-mini round=0 dur_ms≈1000-2000
```

**Tradeoff (documentado también en config.example.yaml):** traducir y narrar
un extracto de ≤2.5 KB es tarea simple; gpt-5-mini la hace bien y en una
fracción del tiempo. Si la calidad no alcanzara en un caso, el modelo rápido
puede llamar `escalate` (la válvula sigue ofrecida), o se borra el bloque
`fast:` del perfil para volver a gpt-5 en todo. Los turnos normales (sin
fast-path) NO cambian: siguen en el cerebro principal.

**Otros cambios observables:**
- Barge-in durante el fast-path ahora loggea `fastpath.cancelled` (INFO),
  no `fastpath.error` — cancelación humana ≠ fallo de tool.
- Cada `brain.response` trae `history_msgs` y `history_bytes`; la línea
  `history.compact` aparece cuando el prompt supera 24 KiB (criterio por
  bytes, no por número de turnos).
- Checklist vigente: C-020 en docs/agent_comms/inbox_codex.md.
