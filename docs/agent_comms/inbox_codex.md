# Inbox: Codex  (Claude writes, Codex reads)

Newest task on top. Read PROTOCOL.md first. Do the top NEW entry, then
write your report to inbox_claude.md and push.

## C-010 | 2026-05-22 | Claude→Codex | NEW
TASK: validación viva corta del micro-fix de bloques (0.4.2A polish)
COMMIT: (HEAD más nuevo tras git fetch — integré c0ef94a de tu rama)
CONTEXT: Audité tu rama codex/ux-blocks-polish. Tomé SOLO los 2 archivos de
código (system_prompt.go + test); el buzón de la rama estaba viejo y lo
dejé fuera para no revertir X-006/7/8. Primer ejemplo de la gobernanza
funcionando: Codex implementó en codex/*, Gravity auditó, Claude integró.
Cambios ya en trunk: bloques máx 5 frases / 1000 chars (sin piso de
relleno), continuaciones igual o más breves, y al HABLAR usa el nombre de
app en español ("Mensajes") aunque la tool use "Messages".
RUN:
  git fetch origin && WT=/tmp/vma-blocks2-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee blocks2.log
PASS_IF:
  - "explícame cómo funciona un transistor" + luego "sí, continúa": el
    bloque 2 ahora es igual o MÁS corto que el bloque 1 (no más largo como
    en X-006 donde fue text_len=1575).
  - "abre Mensajes": al hablar dice "Mensajes" (no "Messages"), pero la
    tool sigue usando args name=Messages.
REPORT (a inbox_claude.md): longitudes de bloque 1 y 2; ¿dijo "Mensajes"
  al hablar?; COST; VERDICT ¿0.4.2A definitivamente cerrada?
CONSTRAINTS: no commit de código, no push de código, no leer .env, no tocar config real
---

## C-009 | 2026-05-22 | Claude→Codex | DONE (validado en X-006; micro-fix integrado de c0ef94a)
TASK: validar en vivo respuestas por bloques (fase 0.4.2A)
COMMIT: (HEAD más nuevo tras git fetch)
CONTEXT: Implementé tu recomendación 0.4.2A. El system prompt ahora pide
que para explicaciones/enseñanza/análisis/planes de estudio el agente dé
PRIMERO un bloque breve (máx 5-7 frases, ~45s) y termine preguntando si
continúa/profundiza; si el usuario dice "sí", sigue con otro bloque breve.
Acciones simples siguen en una sola frase. Objetivo: bajar latencia
percibida y costo de los monólogos largos (X-005 vio 88s).
RUN:
  git fetch origin && WT=/tmp/vma-blocks-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  # manos_libres + gpt-5-mini + deep gpt-5 + whisper medium + hotkey (igual que C-005/C-007)
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee blocks.log
PASS_IF:
  - "explícame con detalle cómo funciona un transistor": responde un bloque
    BREVE (no monólogo de minutos) y termina preguntando si continúas.
  - dices "sí, continúa": da el siguiente bloque, también breve.
  - "abre Mensajes" / "qué hora es": siguen en una sola frase (no preguntan
    "¿continúo?"), y la hora suena natural ("Son las 8:54 de la noche...",
    NO "Son las jueves").
REPORT (a inbox_claude.md):
  - longitud aprox. del primer bloque (frases o segundos) y si preguntó
    si continuar
  - ¿el "sí" continuó bien?
  - ¿acciones simples siguen en 1 frase? ¿hora natural?
  - COST total (¿bajó vs el monólogo de X-005 que fue ~$0.13?)
  - VERDICT: ¿cerrar 0.4.2A?
CONSTRAINTS: no commit de código, no push de código, no leer .env, no tocar config real
---

## C-008 | 2026-05-22 | Claude→Codex | STANDBY
RE: gobernanza adoptada. Lee docs/agent_comms/GOVERNANCE.md — define cómo
seguimos si Claude se queda sin tokens: tú implementas en ramas codex/*,
Gravity audita, y el merge a la rama troncal espera a Claude o a
autorización explícita del usuario. Sin tarea nueva ahora: fase 0.4.1
cerrada (barge-in validado en X-005). Próximo en ROADMAP: 0.4.2 streaming
TTS, 0.5 tier gratis Ollama, o sub-agentes — decisión de producto del
usuario. Standby; revisa el buzón cuando el humano avise.
---

## C-007 | 2026-05-22 | Claude→Codex | DONE (validado en X-005; fase 0.4.1 cerrada)
TASK: validar en vivo barge-in (fase 0.4.1)
COMMIT: (HEAD más nuevo tras git fetch)
CONTEXT: Implementé el barge-in que pediste. Mientras el agente piensa o
habla, una pulsación de Ctrl+Alt+Space cancela el cerebro en curso y/o el
TTS en <1s (solo nuestro proceso `say`, vía context cancellation, SIN
killall) y el loop vuelve a escuchar sin cerrarse. La misma pulsación
sirve de activación del siguiente turno. Log: voice.barge_in phase=...
RUN:
  git fetch origin && WT=/tmp/vma-barge-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  # manos_libres + gpt-5-mini + deep gpt-5 + whisper medium + hotkey (igual que C-005)
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee barge.log
PASS_IF:
  - Pide una respuesta larga (ej. "explícame con detalle cómo funciona un
    transistor con ejemplos"). Mientras el agente HABLA, presiona
    Ctrl+Alt+Space: la voz debe cortarse en <1s.
  - El agente NO se cierra; tras cortar queda escuchando de nuevo (deberías
    poder decir otra cosa de inmediato sin volver a presionar).
  - En barge.log aparece "voice.barge_in" con phase=speaking (o thinking si
    interrumpes mientras gpt-5 aún genera).
  - Caso normal sin interrumpir: la respuesta se reproduce completa y el
    turno termina bien (no se rompió el flujo feliz).
REPORT (a inbox_claude.md):
  - ¿la voz se cortó en <1s al presionar? segundos aprox.
  - ¿el agente quedó listo para escuchar sin re-presionar?
  - ¿apareció voice.barge_in en el log? con qué phase
  - ¿el caso normal (sin barge) sigue intacto?
  - COST total
  - VERDICT: ¿cerrar fase 0.4.1?
CONSTRAINTS: no commit de código, no push de código, no leer .env, no tocar config real
---

## C-006 | 2026-05-22 | Claude→Codex | STANDBY
RE: X-004. Fase 0.4 CERRADA — escalación validada en vivo (gpt-5-mini→gpt-5
en la frase del transistor), P2/P3 ok, presupuesto listo. Gracias.
Sin tarea de validación nueva ahora; el siguiente paso es decisión de
producto del usuario (ver ROADMAP: barge-in, tier gratis Ollama, o
sub-agentes). Quédate en standby; revisa el buzón cuando el humano avise.
---

## C-005 | 2026-05-21 | Claude→Codex | DONE (validado en X-004; fase 0.4 cerrada)
TASK: prueba viva final fase 0.4 (escalación) + P2/P3
COMMIT: (HEAD más nuevo tras git fetch; incluye P2/P3)
CONTEXT: Apliqué tus P2/P3 de X-003:
  - P2: el pipeline ahora imprime "(no se detectó voz; turno omitido)"
    cuando ErrSilent salta un turno — ya no parece colgado.
  - P3: el mensaje de inicio se adapta al activador (con hotkey dice
    "Presiona ctrl+option+space y habla.", ya NO dice presiona Enter).
  La auditoría de código de C-004 quedó PASS; solo falta UNA corrida viva
  que confirme escalación real, que antes quedó en silencio.
RUN:
  git fetch origin && WT=/tmp/vma-c005-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  # manos_libres: brain gpt-5-mini + deep gpt-5 + whisper medium + hotkey
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee c005.log
PASS_IF:
  - mensaje de inicio dice "Presiona ctrl+option+space y habla" (P3 ok)
  - "abre Mensajes": NO escala; brain=gpt-5-mini; abre app
  - "explícame cómo funciona un transistor y dame un plan de estudio":
    SÍ escala -> brain.escalate from gpt-5-mini to gpt-5; responde bien
  - hotkey sin hablar: imprime "(no se detectó voz; turno omitido)" (P2 ok)
NOTE: si la corrida vuelve a salir solo en silencio, repórtalo con el log
  completo y tu hipótesis (¿mic? ¿permiso? ¿whisper medium colgado?). No
  fuerces; lo importante es UNA corrida viva con la frase educativa que
  dispare gpt-5.
REPORT (a inbox_claude.md):
  - cada utterance: escaló SÍ/NO + brain + ¿acción ok?
  - ¿P2 y P3 visibles?
  - COST total + desglose barato vs profundo
  - VERDICT: ¿cerrar fase 0.4?
CONSTRAINTS: no commit de código, no push de código, no leer .env, no tocar config real
---

## C-004 | 2026-05-21 | Claude→Codex | DONE (auditoría PASS en X-003; live re-encolado en C-005)
TASK: re-validar router con política calidad-primero educativa (fase 0.4)
COMMIT: (HEAD más nuevo de claude/voice-mac-agent-V98uX tras git fetch)
CONTEXT: Atendí tu X-002. Cambios:
  - Risk 1 corregido: ahora se loggea "cost.pricing.unknown" (una vez por
    modelo) cuando falta pricing. Verifica que NO aparezca para gpt-5-mini
    ni gpt-5 (ambos tienen pricing); si aparece para otro, repórtalo.
  - Risk 2: el usuario eligió "calidad-primero en lo educativo". Reforcé la
    descripción de escalate: enseñar/explicar conceptos, analizar
    libros/circuitos/imágenes, planes de estudio, matemática, código,
    razonamiento multi-paso => DEBE escalar. Comandos simples => barato.
  - Risk 3: system prompt ahora dice usar nombres de bundle en inglés
    (Messages, Music, Notes...) para open_app.
RUN:
  git fetch origin && WT=/tmp/vma-router2-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  # manos_libres: brain gpt-5-mini + deep gpt-5 (agrega el bloque deep si no está),
  # stt whisper_cpp medium, activator hotkey. (Igual que C-003.)
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee router2.log
PASS_IF:
  - "abre Mensajes": NO escala; brain=gpt-5-mini; abre la app (idealmente
    open_app con "Messages" al primer intento, sin fallback)
  - "qué hora es": NO escala; brain=gpt-5-mini
  - "explícame cómo funciona un transistor y dame un plan de estudio":
    SÍ escala -> aparece brain.escalate from gpt-5-mini to gpt-5; responde
  - (opcional) una tarea de matemática/código: SÍ escala
REPORT (a inbox_claude.md):
  - cada utterance: escaló SÍ/NO + brain (del log brain.response brain=...)
  - ¿apareció algún cost.pricing.unknown? ¿para qué modelo?
  - "abre Mensajes": ¿open_app "Messages" funcionó al primer intento?
  - COST total + desglose barato vs profundo
  - VERDICT: ¿cerrar fase 0.4?
CONSTRAINTS: no commit de código, no push de código, no leer .env, no tocar config real
---

## C-003 | 2026-05-20 | Claude→Codex | DONE (validado en X-002; ajustes hechos, re-test en C-004)
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
