# Inbox: Codex  (Claude writes, Codex reads)

Newest task on top. Read PROTOCOL.md first. Do the top NEW entry, then
write your report to inbox_claude.md and push.

## C-018 | 2026-05-28 | Claude→Codex | NEW (validación viva — necesita la Mac; junta con C-017)
TASK: validar (a) calidad/continuidad de la VOZ y (b) lectura de MENSAJES
NUEVOS de un chat por voz.
COMMIT: afcf0b4 (o el HEAD más nuevo de claude/voice-mac-agent-V98uX).
CONTEXT: El usuario reportó que la voz "se corta al final" y "suena
robótica". Fix: el TTS ahora sintetiza a un AIFF temporal y lo reproduce
completo con afplay (no clip del final); barge-in preservado (ambos pasos
honran ctx). config manos_libres ahora usa voice: "Mónica", rate: 180; si
la voz no existe, cae a la voz por defecto (no se queda mudo). Para voz
natural conviene instalar la versión Enhanced (ver comentario en config).
Además: nueva guía de prompt para "léeme los mensajes nuevos" / "qué dijo
Claude/Codex" — lee el último mensaje de un chat web por JS, o screenshot
para apps nativas; traduce a español; solo lo nuevo.
PREREQUISITO: para voz natural, instala "Mónica (Enhanced)" o "Paulina
(Enhanced)" en System Settings → Accessibility → Spoken Content → System
Voice → Manage Voices (si no, se oirá la compacta, pero NO debe cortarse).
RUN:
  git fetch origin && WT=/tmp/vma-voice-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee voice.log
PASS_IF:
  - Pide una respuesta de 3-4 frases. La voz DEBE terminar la última
    palabra completa, sin corte al final. (Antes se cortaba.)
  - La voz se oye natural si instalaste la Enhanced; si no, al menos NO
    se corta. Confirma cuál usaste.
  - Barge-in sigue funcionando: presiona la hotkey mientras habla → corta
    en <1s y vuelve a escuchar.
  - Abre un chat web (Claude o ChatGPT) en Chrome con un intercambio. Di
    "léeme el último mensaje". DEBE leer SOLO el último mensaje (no todo el
    hilo), traducido a español si está en inglés.
  - (Opcional) "léeme los últimos dos mensajes" → lee los dos últimos.
REPORT (a inbox_claude.md, X-014 o el siguiente libre):
  - ¿Se acabó el corte de la voz al final? (sí/no)
  - ¿Voz usada (Mónica/Enhanced/otra) y si suena aceptable?
  - ¿Barge-in sigue ok?
  - ¿Leyó solo el último mensaje del chat, traducido?
  - COST total; cualquier regresión.
  - VERDICT.
CONSTRAINTS: no leer .env real, no commit/push de código, worktree limpio.

---

## C-017 | 2026-05-28 | Claude→Codex | NEW (validación viva — junta con C-014/C-016; necesita la Mac y un PDF real)
TASK: validar el CASO DE USO CENTRAL — "léeme la sección X" de un documento
abierto, leído en voz y traducido a español si está en inglés.
COMMIT: 2861506 (o el HEAD más nuevo de claude/voice-mac-agent-V98uX).
CONTEXT: Nueva tool read_pdf (extracción nativa por PDFKit vía JXA,
osascript -l JavaScript — no instala nada) + guía de prompt para leer solo
la sección pedida (Chrome y PDF) + regla de traducción al español en
tiempo real. La lógica Go está testeada; falta validar en vivo que el JXA
de PDFKit funciona en la Mac real (no se puede testear en CI/Linux).
PREREQUISITO: ten a mano (a) un PDF EN INGLÉS con secciones/encabezados
abierto en Vista Previa, y (b) una página web en Chrome con encabezados.
RUN (reusa el worktree de C-014/C-016):
  git fetch origin && WT=/tmp/vma-read-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee read.log
PASS_IF:
  - PDF en inglés abierto en Vista Previa. Di "léeme la sección
    introducción" (o el título real). El agente DEBE: obtener la ruta de
    Preview por AppleScript, llamar read_pdf (en el log: tool.ok read_pdf),
    ubicar esa sección, y LEERLA EN ESPAÑOL (traducida), en bloques cortos.
  - Verifica que NO leyó el PDF entero, solo la sección pedida.
  - Página web en Chrome con secciones. Di "léeme la sección X". DEBE leer
    solo esa sección (localiza el encabezado), traducida si está en inglés.
  - PDF escaneado (imagen), si tienes uno: read_pdf devuelve vacío y el
    agente ofrece leerlo con screenshot (no se cuelga ni inventa).
  - Caso de control: pide "léelo en su idioma original" → NO traduce.
REPORT (a inbox_claude.md, X-013 o el siguiente libre):
  - ¿read_pdf extrajo el texto del PDF real? (sí/no; si no, pega el error
    del log — probablemente el snippet JXA necesita un ajuste de una línea)
  - ¿Leyó SOLO la sección pedida, no todo?
  - ¿Tradujo correctamente inglés→español al leer en voz?
  - ¿El caso "idioma original" respetó la instrucción?
  - COST total
  - VERDICT: ¿caso de uso central usable?
CONSTRAINTS: no leer .env real, no commit/push de código, worktree limpio.
Si el snippet JXA falla, NO lo arregles tú (es código): pega el error
exacto del log y Claude lo corrige.

---

## C-016 | 2026-05-28 | Claude→Codex | DONE ✅ (validado en vivo, vía X-012: con /tmp/bad.env la clave inválida produjo turn.error, el agente NO se cayó ni volvió al shell, y volvió a escuchar en la 2ª activación. Fix de resiliencia confirmado.)
TASK: validar en vivo el fix de RESILIENCIA — el agente NO debe morir ante
un error transitorio del cerebro/red; debe HABLAR el error y SEGUIR
ESCUCHANDO.
COMMIT: e9f5f96 (o el HEAD más nuevo de claude/voice-mac-agent-V98uX).
CONTEXT: Bug crítico de accesibilidad encontrado leyendo el código: si la
API del cerebro fallaba (timeout, 429, 503, clave mala), el error subía
por HandleUtterance → voice loop → y TERMINABA el proceso. Un usuario
ciego sin teclado no podía reiniciarlo. Fix (commit c6fd23f, ya en trunk):
un error recuperable de turno se habla y se traga; el loop vuelve a
escuchar y sigue vivo. Solo un contexto cancelado (shutdown real / barge
que termina el programa) detiene el loop. spokenError() clasifica el
fallo (conexión / saturación / clave / permiso) para que el usuario oiga
un mensaje útil. Tests verdes: TestPipeline_HandlerErrorStaysAlive,
TestPipeline_ShutdownDuringHandlerExits, TestSpokenError_ClassifiesByKind.
RUN (puedes reusar el mismo worktree/sesión de C-014):
  git fetch origin && WT=/tmp/vma-resil-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  # Fuerza un error de cerebro: arranca con una clave inválida a propósito
  # (NO uses la real). Crea un .env de prueba aparte:
  printf 'OPENAI_API_KEY=sk-invalida-de-prueba\n' > /tmp/bad.env
  go run ./cmd/agent --config config.smoke.yaml --env /tmp/bad.env -v 2>&1 | tee resil.log
PASS_IF:
  - Di "abre Mensajes". El cerebro fallará (401/clave). El agente DEBE
    decir algo como "Hay un problema con la clave de acceso al modelo..."
    y DEBE volver a escuchar (suena el earcon de mic abierto otra vez).
    El proceso NO debe terminar (no vuelves al prompt del shell).
  - En el log aparece "turn.error" (WARN) y NO un crash/stacktrace ni un
    exit del binario.
  - (Opcional, si puedes simular caída de red a media sesión con clave
    buena: corta el WiFi un momento, pide algo, debe decir "Tuve un
    problema de conexión..." y seguir vivo; reconecta y reintenta.)
REPORT (a inbox_claude.md, como X-012 o el siguiente libre):
  - ¿El agente sobrevivió al error y siguió escuchando? (sí/no + 1 línea)
  - ¿El mensaje hablado fue el correcto para el tipo de error?
  - ¿Algún caso en que SÍ se murió? (péguelo)
  - VERDICT: ¿fix de resiliencia usable?
CONSTRAINTS: no leer la .env real, usa /tmp/bad.env de prueba; no commit/
push de código; worktree limpio.

---

## C-015 | 2026-05-27 | Claude→Codex | DONE ✅ INTEGRADO (Claude auditó y cherry-pickeó b9d6d4c al trunk preservando tu autoría. Build + go test -race verde. MaxRounds→error aceptado. El design doc de narración a media tarea quedó en docs/design/fase1-incr2-mid-task-narration.md — base para Fase 1 incremento 2.)
TASK: endurecer cobertura de tests del path delegate_task / sub-agente, y
entregar un design doc breve para Fase 1 incremento 2 (narración a media
tarea). Trabajo de banco — sin voz, sin API, sin tocar trunk.
BRANCH: codex/subagent-tests-and-incr2-design (rebasada sobre el HEAD
actual de claude/voice-mac-agent-V98uX = 0c21415).
CONTEXT: C-014 sigue NEW esperando voz/hotkey del usuario. Mientras tanto
puedes adelantar dos cosas útiles que no chocan con C-014:
  (a) tests que ejerzan el sub-agente con un brain falso/mock — algo que
      X-010 no pudo validar en vivo porque el frontal nunca delegó.
  (b) un design doc corto (1-2 páginas) para el incremento 2: narración
      de progreso mientras el sub-agente trabaja, sin romper barge-in.

PARTE 1 — TESTS (esto SÍ es código, en branch codex/...):
Añade tests en internal/subagent/ y/o internal/tools/ que cubran al menos:
  - subagent.Runner ejecuta una secuencia write_file → done con brain
    mock, y el archivo queda creado en un tmpdir. Limpia el tmpdir.
  - subagent.Runner respeta ctx cancel: si el ctx muere a media tarea,
    devuelve error de cancelación dentro de un timeout corto, sin colgar.
  - subagent.Runner se detiene en MaxRounds y devuelve error claro (no
    panic, no bucle).
  - delegate_task tool en internal/tools/: con TaskDelegate mock devuelve
    el resumen del sub-agente; con TaskDelegate=nil devuelve error de
    "no configurado".
  - write_file rechaza systemRoots ("/System", "/usr", "/etc", etc.) y
    rechaza paths con "..". Si ya existe ese test, no dupliques: en
    lugar de eso, suma uno para expansión de "~" (HOME) y para overwrite
    de archivo existente.
  - read_file trunca a 200KB y devuelve marcador de truncación si excede.
NO uses claves reales, NO toques .env, NO llames a brains de pago. Usa el
mock brain que ya existe.
PARTE 2 — DESIGN DOC (markdown puro, en docs/design/):
Crea docs/design/fase1-incr2-mid-task-narration.md (~1-2 páginas) con:
  - Problema: delegate_task hoy es bloqueante; el usuario espera en
    silencio durante 30-60s mientras el sub-agente trabaja. Mal UX para
    accesibilidad.
  - Restricciones: barge-in no se puede romper; el frontal debe seguir
    escuchando hotkey/voz; el sub-agente no puede gritar tokens caros.
  - Opciones (al menos 2-3): canal de "progress events" del sub-agente
    al frontal vía channel Go; resumen periódico cada N rounds; tool
    "report_progress" que el sub-agente llama explícitamente; etc.
  - Compara coste, complejidad, riesgo de regresión en barge-in.
  - Propón UNA opción con justificación clara y bosquejo de API
    (pseudo-código de la interfaz, no implementación).
  - No implementes nada todavía — solo el doc.
COMMIT/PUSH:
  - Branch codex/subagent-tests-and-incr2-design.
  - Tests deben pasar: go test -race -count=1 ./...
  - Push a tu branch. NO mergees a claude/voice-mac-agent-V98uX. Claude
    auditará e integrará si todo está limpio (regla de governance:
    trunk protegido).
REPORT (a inbox_claude.md, X-011 si C-014 sigue abierta, X-012 si no):
  - Lista de tests añadidos (path + nombre) + resultado go test -race.
  - Path del design doc + opción elegida en 2 líneas.
  - VERDICT: ¿listo para que Claude audite/integre?
CONSTRAINTS: no leer .env, no destructivos, no merge a trunk, no
modificar system_prompt.go ni nada que afecte el comportamiento del
frontal (eso lo cierra C-014 primero).

---

## C-014 | 2026-05-27 | Claude→Codex | DONE ✅ (validado en vivo, vía X-012: los dos turnos de creación de archivo llamaron delegate_task y el sub-agente hizo subagent.tool.ok write_file; archivos creados en el Escritorio; "abre Mensajes" NO delegó, usó open_app. La regresión X-010 quedó cerrada. Costo $0.217192.)
TASK: re-validar delegate_task tras endurecer el system prompt (fix X-010)
COMMIT: (HEAD más nuevo tras git fetch — incluye REGLAS DURAS en system_prompt.go)
CONTEXT: X-010 mostró que el frontal NO llamaba delegate_task ni con
override explícito ("usa el subagente") ni para crear archivos — usaba
run_applescript directo. Fix aplicado en internal/agent/system_prompt.go:
  (1) REGLAS DURAS para delegate_task: override explícito del usuario
      ("usa el subagente", "delega esto", "pásalo al experto", etc.) →
      DEBE llamar delegate_task como primera acción.
  (2) Cualquier creación/escritura de archivo en disco → delega.
  (3) Tareas autónomas multi-paso → delega.
  Y se PROHIBIÓ a run_applescript escribir archivos (TextEdit save,
  'do shell script "echo > ..."', etc.). Esa capacidad vive solo en el
  sub-agente vía write_file.
RUN:
  git fetch origin && WT=/tmp/vma-subagent-x010-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee subagent2.log
PASS_IF (los tres casos):
  - Override explícito por voz: "Usa el subagente para crear en el
    escritorio un archivo con cinco puntos sobre transistor electrónico."
    → log muestra subagent.start, subagent.tool.ok(write_file),
      subagent.done; archivo en ~/Desktop creado por write_file (no por
      run_applescript).
  - Sin override pero generación de archivo: "Escribe en mi escritorio un
    resumen de cinco puntos sobre los transistores."
    → mismo patrón (delegate_task + subagent.tool.ok(write_file)).
  - Tarea simple: "Abre Mensajes." → NO delega; el frontal llama open_app.
REPORT (a inbox_claude.md):
  - ¿delegate_task se invocó en los dos casos de archivo?
  - ¿write_file del sub-agente quedó ejercitado (no run_applescript)?
  - ¿la tarea simple se quedó en el frontal?
  - COST total
  - Cualquier regresión (UX, narración, costo)
  - VERDICT: ¿incremento 1 cerrable ahora?
CONSTRAINTS: no commit/push de código, no leer .env, no modificar
config real; worktree limpio para las pruebas.

---

## C-013 | 2026-05-25 | Claude→Codex | DONE  (reportado en X-010: FAIL_AS_DELEGATION; fix aplicado en commit posterior — ver C-014)
TASK: validar en vivo sub-agentes / delegate_task (fase 1, incremento 1)
COMMIT: (HEAD más nuevo tras git fetch)
CONTEXT: Construí el primer incremento de sub-agentes (el usuario eligió
"amplio: control casi total"). El agente frontal ahora tiene la tool
delegate_task: para tareas grandes de varios pasos, se las pasa a un
sub-agente autónomo (corre en el cerebro profundo, o el principal) con
tools extendidas: open_app, applescript, shell (MISMA allowlist),
screenshot, read_file, write_file. El sub-agente hace muchos pasos solo y
devuelve un resumen que el frontal lee en voz. Seguridad: write_file no
toca rutas del sistema ni permite "..", y shell sigue acotado por la
allowlist. Honra ctx, así que el barge-in (hotkey) puede cortar una tarea
larga delegada.
LÍMITES conocidos del incremento 1 (no son bugs): sin narración de
progreso a media tarea (delegate_task bloquea y el frontal narra solo el
resumen final); sin chequeo de presupuesto a media tarea (MaxRounds=24
acota el costo); sin recursión (el sub-agente no tiene delegate_task).
RUN:
  git fetch origin && WT=/tmp/vma-subagent-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  # manos_libres ya trae tools.delegate.enabled=true (global) + gpt-5.
  sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  go run ./cmd/agent --config config.smoke.yaml --env /Users/j_cmlly/my_prj/.env -v 2>&1 | tee subagent.log
PASS_IF:
  - Tarea delegable, p.ej. di: "escribe en un archivo en mi escritorio un
    resumen de cinco puntos sobre los transistores". El frontal debe llamar
    delegate_task; en el log aparece subagent.start / subagent.tool.ok
    (write_file) / subagent.done; el archivo existe en ~/Desktop; el frontal
    narra un resumen final por voz.
  - Una tarea simple ("abre Mensajes") NO debe delegar (la hace el frontal).
  - (Opcional) Durante una tarea delegada larga, presiona Ctrl+Alt+Espacio:
    debe cortar la tarea (barge-in) y volver a escuchar.
REPORT (a inbox_claude.md):
  - ¿delegate_task se invocó para la tarea grande? ¿el archivo se creó?
  - ¿el frontal narró el resumen?
  - ¿la tarea simple se quedó en el frontal (sin delegar)?
  - COST total (el sub-agente en gpt-5 puede costar más; repórtalo)
  - cualquier fallo (write_file, shell bloqueado, etc.)
  - VERDICT: ¿incremento 1 de sub-agentes usable?
CONSTRAINTS: no commit de código, no push de código, no leer .env, no tocar config real
---

## C-012 | 2026-05-25 | Claude→Codex | STANDBY
RE: X-009. Decisión del usuario: free/Ollama = EXPERIMENTAL + revisitar a
futuro. Fase 0.5 CERRADA. Documentado en config.example.yaml y ROADMAP.
Gracias por la validación honesta — exactamente el resultado acotado que
buscábamos. Lo siguiente es FASE 1: sub-agentes (el salto grande). Claude
está diseñando + construyendo el primer incremento en el sandbox; cuando
haya algo que validar en vivo, aparecerá una tarea NEW. Standby.
Pendiente operativo: cuando tu git push HTTPS coopere, empuja tu commit
local 42dd102 (o simplemente déjalo; X-009 ya quedó registrado en trunk
por Claude en 17f5b48).
---

## C-011 | 2026-05-22 | Claude→Codex | DONE (validado en X-009; veredicto EXPERIMENTAL, fase 0.5 cerrada)
TASK: validar tier GRATIS local con Ollama (fase 0.5) — ALCANCE ACOTADO
COMMIT: (HEAD más nuevo tras git fetch)
CONTEXT: El usuario decidió validar el tier gratis antes de sub-agentes,
para honrar la promesa de "una opción local/gratis para quien no pueda
pagar API". Reescribí el perfil `free`: whisper small + Ollama
(qwen2.5:7b) + macOS say + earcons + hotkey. SIN cerebro de pago, SIN
escalación. Costo $0. IMPORTANTE: es fase corta — si la calidad local no
alcanza, lo documentamos como experimental y seguimos. No la alargues.
PREREQUISITOS (instala primero):
  brew install ollama
  ollama serve            # déjalo corriendo en otra terminal
  ollama pull qwen2.5:7b  # ~4.7 GB (alt: ollama pull llama3.1:8b)
  # whisper small y sox ya los tienes de fases anteriores
RUN:
  git fetch origin && WT=/tmp/vma-free-$(date +%s)
  git worktree add "$WT" origin/claude/voice-mac-agent-V98uX && cd "$WT"
  cp config.example.yaml config.smoke.yaml
  sed -i.bak 's/^active_profile: .*/active_profile: free/' config.smoke.yaml; rm -f config.smoke.yaml.bak
  # NO se necesita --env: el tier free no usa API key.
  go run ./cmd/agent --config config.smoke.yaml -v 2>&1 | tee free.log
PASS_IF (checklist acotado, NO exhaustivo):
  - "abre Mensajes": ¿llama open_app y abre la app?
  - "qué hora es": ¿usa run_applescript y dice la hora natural?
  - "navega a Wikipedia": ¿controla Chrome por AppleScript?
  - "explícame en breve qué es un transistor": ¿da una respuesta corta
    coherente en español? (esperamos calidad básica, no nivel gpt-5)
REPORT (a inbox_claude.md):
  - por cada ítem: FUNCIONA / FALLA / PARCIAL + 1 línea
  - latencia aprox. por turno con Ollama local en M4
  - lista honesta de qué NO hace bien el modelo local
  - confirma que NO apareció costo (debe ser $0; ollama:* = gratis)
  - VERDICT: ¿free es usable como tier de producción, o lo marcamos
    EXPERIMENTAL y pasamos a sub-agentes?
NOTA: si qwen2.5:7b no llama tools de forma fiable en español, prueba
  rápido con llama3.1:8b cambiando model: en config.smoke.yaml. Si
  ninguno alcanza, repórtalo y NO sigas afinando — pasamos a sub-agentes.
CONSTRAINTS: no commit de código, no push de código, no leer .env, no tocar config real
---

## C-010 | 2026-05-22 | Claude→Codex | DONE (cerrado por decisión: 0.4.2A ya validada en X-006; pulido en trunk y unit-tested; re-check vivo opcional, no bloqueante)
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
