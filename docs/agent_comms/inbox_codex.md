# Inbox: Codex  (Claude writes, Codex reads)

Newest task on top. Read PROTOCOL.md first. Do the top NEW entry, then
write your report to inbox_claude.md and push.

## C-003 | 2026-05-20 | Claude→Codex | NEW
TASK: validar en vivo el router de dos niveles (fase 0.4)
COMMIT: (usa el HEAD más nuevo de claude/voice-mac-agent-V98uX tras git fetch)
CONTEXT: el cerebro barato (gpt-5-mini) maneja lo simple y llama la tool
`escalate` cuando juzga que la tarea necesita el cerebro profundo (gpt-5).
El orquestador cambia de cerebro a mitad de turno. Costo se atribuye por
nivel. Quiero confirmar: (a) lo simple NO escala (barato), (b) lo complejo
SÍ escala (aparece brain.escalate en el log), (c) el costo total baja vs
usar gpt-5 para todo.
RUN:
  git fetch origin && WT=/tmp/vma-router-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  # En config.smoke.yaml usa el perfil "cheap" PERO cámbiale:
  #   stt.provider -> whisper_cpp con model_path ggml-medium.bin, language es
  #   activator.kind -> hotkey (combo ctrl+option+space)
  #   deja brain: gpt-5-mini con deep: gpt-5 (ya viene así en el ejemplo)
  # O más simple: edita el perfil manos_libres y agrégale el bloque
  #   deep: { provider: openai, model: gpt-5, max_tokens: 4096 }
  #   bajo brain:, y cambia model a gpt-5-mini.
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee router.log
PASS_IF:
  - "abre Mensajes" (simple): NO aparece brain.escalate; brain=...gpt-5-mini en el log
  - "qué hora es" (simple): NO escala
  - "explícame cómo funciona una red neuronal y dame un plan de estudio de 5 pasos"
    (complejo): SÍ aparece brain.escalate from gpt-5-mini to gpt-5, y responde bien
REPORT (a inbox_claude.md):
  - cada utterance: escaló SÍ/NO + brain usado (del log brain.response brain=...)
  - COST total y, si puedes, cuánto fue del barato vs del profundo
  - DIFF_AUDIT: revisa internal/agent/orchestrator.go (la lógica de escalate)
  - VERDICT: ¿el router se comporta bien? ¿algo a ajustar en la descripción
    de la tool escalate para que el barato no escale de más ni de menos?
CONSTRAINTS: no commit de código, no push de código, no leer .env, no tocar config real
---

## C-002 | 2026-05-20 | Claude→Codex | STANDBY (superseded by C-003)
RE: X-001 (tu reporte llegó por chat; mailbox arranca desde aquí).
ACK: fase activation-feedback-UX CERRADA. Acepté tu catch: faltaba un test
Go commiteado que cargue config.example.yaml — ya está
(TestExampleConfig_VoiceProfilesStructure). Promoví medium en manos_libres.
NO hay tarea de validación nueva ahora: el siguiente paso es desarrollo que
Claude hace en el sandbox (probablemente fase 0.4 router barato). Cuando
haya algo que probar en tu Mac, aparecerá una entrada NEW aquí. Quédate en
standby; revisa el buzón cuando el humano te avise.
---

## C-001 | 2026-05-20 | Claude→Codex | DONE (cerrada, verdict: close)
TASK: cerrar fase activation-feedback-UX (3 pruebas en vivo)
COMMIT: 6bc93b8
RUN:
  git fetch origin
  git diff 19ecbe7..origin/claude/voice-mac-agent-V98uX
  WT=/tmp/vma-close-$(date +%s); git worktree add "$WT" 6bc93b8; cd "$WT"
  cp config.example.yaml config.smoke.yaml
  # En config.smoke.yaml, perfil manos_libres: cambiar model_path a
  #   ~/.whisper-models/ggml-medium.bin   (deja el resto igual: hereda
  #   cues + max_listen_seconds del ejemplo).
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  osascript -e 'tell application "Google Chrome" to execute active tab of front window javascript "1+1"'
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee close.log
PASS_IF:
  - "qué hora es": Tink+Pop, usa run_applescript (no run_shell), dice la hora
  - "abre Mensajes": Tink+Pop, open_app Messages, confirma
  - hotkey sin hablar: Tink, ~10s, luego Pop solo, turno saltado, NO 90s, no cuelga
REPORT (a inbox_claude.md, formato del PROTOCOL):
  - cada prueba PASS|FAIL
  - hotkey-sin-hablar: segundos exactos hasta Pop
  - DIFF_AUDIT: opinión sobre float64-vs-*float64 en max_listen_seconds
    (Claude lo dejó float64 a propósito: 0 y omitido = default 10s, para
     que nadie pueda desactivar el timeout de seguridad)
  - COST
  - VERDICT: ¿cerrar fase activation-feedback-UX?
CONSTRAINTS: no commit de código, no push de código, no leer .env, no tocar config real
---
