# Roadmap

Phases are ordered so each one produces something working end-to-end and
testable on the user's Mac mini before moving on. No phase is "skeleton
code only" — every phase ends with a real user-visible behaviour.

## Fase activation-feedback-UX  ✅ DONE (validated live, 2026-05-20)

- [x] Earcons: Tink when the mic opens, Pop when it closes. Default ON
      (accessibility). Validated live: both tones audible, open-cue does
      not bleed into the recording.
- [x] Listen timeout (max_listen_seconds, default 10): an activation with
      no speech self-closes (~10s) instead of hanging indefinitely. This
      was a real accessibility hole found by Codex's live test.
- [x] "qué hora es" resolves via run_applescript current date, not the
      blocked run_shell date.
- [x] medium promoted as the manos_libres STT default (A/B 10/10 vs 7/10
      on proper nouns; ~1.5s latency on M4).
- [x] Committed regression test loads config.example.yaml and asserts
      cues live under voice: and every voice profile caps the listen
      window (guards the mis-nesting bug from recurring).
- Open non-blocker: medium still occasionally degrades "qué hora es" to
  "y hora."; the brain handles it without run_shell. Revisit if STT
  accuracy on short phrases becomes a complaint (cloud STT for premium).

## Fase 0.1 — Skeleton with one real path  ✅ DONE

- [x] Go module + Makefile.
- [x] All core interfaces: `Brain`, `Tool`, `VoiceProvider`, `Handler`,
      `STT`, `TTS`, `Activator`, `OSAdapter`.
- [x] One real brain: OpenAI (GPT-5 family) via raw HTTP.
- [x] Mock brain for offline tests.
- [x] macOS adapter (`open`, `osascript`, `/bin/sh`).
- [x] macOS TTS via `say`.
- [x] Stdin "STT" placeholder.
- [x] Three tools: `open_app`, `run_applescript`, `run_shell`.
- [x] Config with three profiles: `free`, `cheap`, `premium`.
- [x] Compiles for linux/amd64 and darwin/arm64.
- [x] Documentation: README, HANDOFF, ARCHITECTURE, ROADMAP, TESTING.

**Exit criterion:** User runs `make run`, types "abre Google Chrome",
Chrome opens and the Mac speaks back.

## Fase 0.2 — Real speech-to-text

- [x] Microphone capture via `sox -d` (shells out — keeps the Go binary
      free of cgo audio bindings, ships portable, works on Mac out of
      the box once `brew install sox` is done).
- [x] Silence-based end-of-utterance detection using sox's `silence`
      filter. Threshold and silence-seconds configurable per profile.
- [x] `whisper.cpp` shells out to `whisper-cli` (Homebrew formula
      `whisper-cpp`). Model `small` recommended on M4 for accuracy/speed
      balance; user downloads it once.
- [x] `internal/stt/whisper.go` implementing `stt.STT` with a
      `PreflightCheck` that surfaces a helpful error at startup when sox,
      whisper-cli, or the model file are missing.
- [x] `internal/activator/enter.go` — Enter-to-talk activator for fase
      0.2. Real global hotkey is fase 0.3.
- [x] `cmd/agent/main.go` wires `whisper_cpp` and `enter`.
- [x] New `voice` profile in `config.example.yaml` covering mic →
      whisper.cpp → GPT-5 → macOS say end-to-end.
- [x] `docs/FASE_02_TESTING.md` — step-by-step setup on the Mac for an
      external AI to follow.
- [ ] Validation on the Mac mini (pending user run).

**Exit criterion:** User says "abre Chrome" out loud and the agent
hears it. Same loop, no typing.

## Fase 0.3 — Real activation, browser control, screenshot

- [x] Global hotkey activator using `golang.design/x/hotkey`. macOS
      requires Accessibility permission; we document that.
- [ ] `internal/tools/screenshot.go` returning an image to the brain.
      Use OpenAI's image input (`content` array with `image_url`).
- [ ] Chrome control via DevTools Protocol — connect to a
      user-launched Chrome with `--remote-debugging-port=9222`. Tools:
      `chrome_navigate`, `chrome_read_text`, `chrome_click`,
      `chrome_type`.
- [ ] First end-to-end scripted demo: "abre Chrome, busca circuitos
      RLC, léeme el primer párrafo del primer resultado."

**Exit criterion:** That sentence works without typing.

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

## Fase 0.4 — Two-tier brain + cost tracking  ✅ DONE (validated live 2026-05-22)

Live proof (X-004): "abre Mensajes" stayed on gpt-5-mini ($0.0012);
"explícame un transistor + plan de estudio" escalated gpt-5-mini→gpt-5
($0.052). P2 silent-skip and P3 startup hint both confirmed in voice.
Honest cost note: quality-first escalation means an educational answer
costs ~$0.05 on gpt-5; simple commands stay ~$0.001. That tradeoff is the
user's explicit choice.


- [x] `internal/brain/anthropic.go` — Claude with prompt caching (system
      prompt + tool block both marked `cache_control: ephemeral`).
- [x] Router/escalation: the cheap brain (`gpt-5-mini`) is the default
      and handles simple turns. It is offered an `escalate` tool; when it
      judges a turn needs deep reasoning it calls it, and the orchestrator
      switches to the deep brain (`gpt-5`) for the rest of that turn. The
      escalate tool is offered to the cheap brain only. Cost is attributed
      per tier (Lookup by active brain's Name()). Config: brain.deep
      {provider, model, ...}. Built + unit-tested in the sandbox; live
      validation pending on the Mac.
- [x] `internal/cost/tracker.go` — append-only NDJSON cost log
      (zero-deps, USB-portable, crash-safe). Records every brain call:
      timestamp, session id, brain name, tokens in/out/cached, USD.
- [x] Pricing table (`internal/cost/pricing.go`) — map of brain name
      → $ per 1M tokens (input, output, cached). Defaults shipped for
      OpenAI gpt-5 family and Anthropic Claude 4 family.
- [x] Tool: `show_cost` so the user can ask "cuánto llevo gastado hoy"
      / "este mes" — brain calls the tool and narrates the result.
- [x] Hard-stop when `monthly_budget_usd` exceeded: before spending on a
      turn, the orchestrator checks month-to-date cost; at/over the cap it
      refuses with a spoken message and never calls the brain. Crossing
      `warn_at_pct` appends a one-time spoken heads-up. 0 budget = no limit.
      Built + unit-tested in the sandbox.
- [ ] User-overrideable pricing via config (for when prices change).

**Exit criterion:** User can run a 30-minute session and see real cost
in the cost log. Cost on `cheap` profile is well below $1/hour.

## Fase 0.5 — Free tier (Ollama)  ✅ DONE → EXPERIMENTAL (X-009, 2026-05-25)

The humanitarian promise: a fully-local $0 option. Ollama brain works and
costs $0, but the live validation (X-009) found that qwen2.5:7b does NOT
reliably honor the tool-calling contract — it sometimes called open_app
but often hallucinated (made up the time instead of calling
run_applescript) or returned AppleScript as spoken text, inconsistently.
Basic local Q&A in Spanish: fine. Reliable Mac control: no.

Per the user's pre-set bounded-scope rule, we did NOT keep tuning:

- [x] `internal/brain/ollama.go` — local LLM via Ollama, $0 pricing.
- [x] `free` profile = real local voice tier (whisper small + Ollama +
      say + cues + hotkey), guarded by TestExampleConfig_FreeTierIsLocalAndFree.
- [x] Live validation (X-009): documented above.
- [x] Verdict: **free tier shipped as EXPERIMENTAL**, labeled honestly in
      config.example.yaml. Not the dependable accessibility path — that's
      `cheap` (gpt-5-mini) or `manos_libres` (gpt-5).

**REVISIT (deferred, user decision "experimental + revisitar a futuro"):**
when stronger local tool-calling models are available, re-run the C-011
checklist against, e.g., qwen2.5:14b (if it fits the M4's RAM), llama3.1:8b,
or a future release. If one reliably calls tools in Spanish, promote the
free tier from experimental to production. Until then it stays best-effort.

## Fase 0.4.1 — Barge-in / stop-speaking  ✅ DONE (validated live 2026-05-22, X-005)

Live proof: a long gpt-5 answer was cut <1s by a hotkey press mid-speech;
`voice.barge_in phase=speaking` logged; the agent stayed alive and was
ready to listen again WITHOUT re-pressing (one gesture = cut and talk).
Normal (non-interrupted) replies still play fully. Open follow-ups (non-
blocking): streaming for perceived latency (0.4.2); natural Spanish time
phrasing (fixed in system prompt — was "Son las jueves...").


Surfaced live (X-004): a gpt-5 educational answer took ~58s and was long
to listen to, with no way to interrupt. For a non-sighted user a long
uninterruptible monologue is real friction. Implemented:

- [x] The activation gesture interrupts: while the agent is thinking OR
      speaking, one hotkey/Enter press cancels the in-flight brain call
      and/or the TTS within ~1s and the loop returns to listening without
      exiting. Implemented in voice.Pipeline.processTurn via a single
      barge watcher + context cancellation. Only our own processes are
      cancelled (macOS `say` dies on ctx cancel) — no killall.
- [x] The interrupting gesture doubles as the next turn's activation
      (skipActivation), so it's one press to "cut and talk".
- [x] Activator.SupportsBargeIn() gates the behavior: true for hotkey and
      Enter; false for always-on (returns immediately) and USB presence
      (awkward to repeat mid-speech) to avoid false triggers.
- [x] Structured log `voice.barge_in` with phase=thinking|speaking.
- [x] Test: barge-in cuts a blocking TTS and the pipeline relistens
      without exiting (internal/voice/pipeline_test.go).
- [ ] Live validation on the Mac (queued as C-006): ask for a long
      answer, press the hotkey mid-speech, confirm it stops <1s and is
      ready to listen again.

## Fase 0.4.2A — Bounded answers in blocks  ✅ DONE (validated live X-006, polished c0ef94a)

Cheap win before real streaming, recommended by Codex after X-005 showed
31.9s and 88.4s educational answers. Pure system-prompt policy, no
Brain/TTS redesign. Live proof (X-006): cost dropped ~70%
($0.131 → $0.040). Two caveats from X-006 then polished via Codex's
Gravity-audited branch codex/ux-blocks-polish (c0ef94a), integrated by
Claude:

- [x] System prompt: first block max 5 sentences / 1000 chars (fewer is
      better — Gravity removed an earlier "700-1000" floor that induced
      padding); ends asking whether to continue; simple actions stay
      one-sentence.
- [x] Continuations must be equal-or-shorter (fixes X-006 caveat: block 2
      had been longer than block 1), no long lists/formulas unless asked.
- [x] When SPEAKING, use the Spanish app name ("Mensajes") even though the
      tool args use the English bundle name ("Messages") — fixes the
      "Abierto Messages" caveat.
- [x] Regression test covers all the above markers.
- [~] C-010 queued: short live re-check that block 2 ≤ block 1 and the
      spoken name is Spanish (polish confirmation; phase already counts as
      done on the X-006 result).
- First exercise of GOVERNANCE.md: Codex implemented on codex/*, Gravity
  audited, Claude integrated the code (mailbox left out — branch copy was
  stale).

## Fase resilience-hardening — agent survives transient errors  ✅ DONE (validated live 2026-05-28, C-016)

Critical accessibility bug found by code review: a transient brain error
(network timeout, 429, 503, bad key) propagated up from HandleUtterance and
TERMINATED the process — a non-sighted user with no keyboard could not
restart it.

- [x] A recoverable turn error is spoken and swallowed; the loop returns to
      listening and stays alive. Only a cancelled context (real shutdown /
      barge) stops it. `spokenError()` classifies the failure (connection /
      saturation / auth / permission) so the user hears a useful message.
      TTS hiccups are likewise non-fatal.
- [x] Tests: handler error keeps the agent listening; shutdown still exits
      cleanly; spokenError classification by kind.
- [x] Live proof (C-016): with a deliberately invalid key, the agent logged
      `turn.error`, did NOT crash or return to the shell, and relistened on
      the next activation.

## Read documents aloud — read_pdf + section nav + translation  — code done, live validation pending (C-017)

The core use case: a user who cannot see asks "léeme la sección X" of an
open document and hears it, translated to Spanish if the source is English.

- [x] `internal/tools/pdf.go` — read_pdf extracts PDF text via macOS-native
      PDFKit (JXA / osascript), no dependency to install. Scanned/image PDF
      returns a clear hint to fall back to a screenshot. Path shell-quoted.
- [x] System prompt: read ONLY the requested section (Chrome: find the
      heading, read to the next; PDF: path from Preview → read_pdf → locate
      section). Real-time translation to Spanish unless the user asks for
      the original language.
- [x] Tests (mock OS adapter): extraction, scanned hint, missing file,
      traversal rejection, truncation, shell-quote escaping. Green -race.
- [ ] Live validation (C-017): the JXA/PDFKit path runs only on macOS;
      confirm extraction works on a real English PDF and the section is read
      back in Spanish.

## Token-efficiency — history compaction  — code done (bench), no live data yet

Every turn re-sends the whole history; a single full-PDF tool result
(200 KiB) was getting re-billed on every later turn of the session. Now,
before each new user turn, the orchestrator compacts deterministically (no
extra LLM call, $0):

- [x] Tool results older than the last 4 user turns are cut to a 300-byte
      head + elision marker; old images dropped.
- [x] If still over the ~24 KiB soft limit, whole oldest turns are dropped at
      user-message boundaries (never orphans a tool_call/result pair). System
      prompt and the recent window always survive.
- [x] Log line `history.compact before_bytes=… after_bytes=…` for live audit.
- [x] Tests: under-limit untouched, old-result shrink keeps the pair intact,
      oldest-turn drop preserves system + recent window, recent window never
      shrunk, UTF-8-safe truncation. Green -race.
- [ ] Live observation: in a long session, grep `history.compact` and confirm
      in_tokens stops growing turn-over-turn.

## Fast-path robustness (bench, queued for C-019 live run)

- [x] "léeme la sección **de** introducción", "…**por favor**", "…**en su
      idioma original / en inglés / del pdf**" now clean to the bare heading
      before hitting the tool (the raw capture used to fail the heading
      match). "léame" (usted) accepted.
- [x] Synthetic fast-path tool-call ids unique per session (Anthropic rejects
      duplicate tool_use ids).
- [x] UTF-8-safe truncation in logs and PDF section windows (no split runes).

## Fase 0.4.2B — Streaming TTS (deferred)

- [ ] Stream the reply to TTS sentence-by-sentence so the user hears the
      first sentence while the rest generates (cuts perceived latency on
      slow deep-brain answers). Needs a Brain.ChatStream variant + a
      pipeline that pipes blocks to TTS; bigger change, kept separate.

## Fase 1 — Sub-agents (delegate_task)  — increment 1 ✅ DONE (validated live 2026-05-28, X-012)

Live proof (X-012): both file-creation turns called delegate_task and the
sub-agent ran `write_file` (not the frontal via run_applescript) — the
X-010 regression is fixed. Two files landed on the Desktop
("Transistor - cinco puntos.txt", "Transistores - 5 puntos.txt"); "abre
Mensajes" stayed on the frontal with open_app (no delegation). Cost
$0.217192 for the session. The X-010 fix (delegate_task as a hard rule:
explicit override + any file write + multi-step) held in voice.

The big capability leap from the original vision ("study a whole book",
"build an app", "analyze a circuit in depth"). The frontal agent stays
conversational and cheap; for heavy multi-step jobs it calls delegate_task,
which hands the work to an autonomous sub-agent that loops brain+tools many
times and reports back a summary the frontal reads aloud. User chose
"amplio: control casi total" power level.

- [x] `internal/subagent/subagent.go` — Runner: autonomous brain+tools loop
      on the strongest available brain, bounded by MaxRounds (default 24,
      caps worst-case cost/time), records cost per call, honors ctx cancel
      (so a hotkey barge-in aborts a long delegated task).
- [x] `internal/tools/file.go` — read_file / write_file with safety:
      ~ expansion, no `..` traversal, write refuses macOS system dirs.
- [x] `internal/tools/delegate.go` — delegate_task tool (depends on a
      TaskDelegate interface, so tools doesn't import subagent — no cycle).
- [x] Sub-agent gets the OS tools + read/write file; shell stays
      allowlist-gated (the safety boundary for "execute" power).
- [x] Prompts: frontal told to delegate big tasks (and say a one-liner
      first); sub-agent prompt is an autonomous worker that returns a short
      Spanish summary and skips destructive actions.
- [x] Wired in main.go behind `tools.delegate.enabled`; uses the deep brain
      if configured, else the main brain.
- [x] Tests: runner (tool loop, MaxRounds cap, ctx cancel), file tools
      (write+read, system-path refusal, traversal refusal, truncation),
      delegate tool (task passthrough, empty, nil, error). All green -race.
- [x] Live validation (C-014, X-012): both file-creation turns delegated;
      sub-agent wrote the files; "abre Mensajes" did not delegate. The
      stronger delegate_task rule (C-013/X-010 fix) held in voice.
- [x] Deeper regression tests integrated from Codex (C-015): write_file
      end-to-end with a mock brain, ctx-cancel during a blocking brain call,
      MaxRounds now returns a clear error (surfaced to the frontal as a tool
      error it narrates), home-expansion + overwrite, all system roots, and
      the default 200 KiB read_file truncation. All green -race.

Known limits of increment 1 (honest):
- No mid-task voice narration: delegate_task blocks while the sub-agent
  works; the frontal speaks only the final summary. (Barge-in DOES cancel
  a long task, since the sub-agent honors ctx.) Real-time progress needs
  concurrency/streaming — a later increment.
- No budget check mid-task: MaxRounds bounds cost; a per-round budget guard
  is a follow-up.
- No recursion: the sub-agent does not get its own delegate_task.

## Fase 2 — Realtime voice as premium option (renumbered; deferred)

- [ ] `internal/voice/realtime.go` — `voice.Provider` implementation
      using the OpenAI Realtime API over WebSocket.
- [ ] Tools must work identically: realtime supports function calls
      natively, so the same `tools.Registry` is reused.
- [ ] Toggle in config: `voice.mode: realtime` vs `pipeline`.
- [ ] Barge-in (interruption) support so the user can cut the agent off.

**Exit criterion:** Same agent, two modes. `pipeline` for cheap/free,
`realtime` for premium UX.

## Fase 2 — Commercial-grade Mac

- [ ] Apple Developer account, code signing, notarization.
- [ ] Distribution-ready bundle (`.app`).
- [ ] USB-presence activator: detect insertion of a labeled USB and
      auto-launch.
- [ ] Wake-word activator (Picovoice Porcupine).
- [ ] BLE button activator (Flic).
- [ ] 3.5mm accessibility-switch activator (via USB-Switch adapter).
- [ ] Menubar UI (Tauri or native Swift wrapper) — status only,
      not control.
- [ ] Cost dashboard (a simple HTML page served on localhost).
- [ ] Auto-update mechanism (Sparkle-compatible or GitHub Releases
      polling).
- [ ] Crash reporting (Sentry or local-only).
- [ ] Telemetry, **opt-in only**.
- [ ] Privacy policy, terms, support process.

**Exit criterion:** A non-developer can buy / receive the product,
plug a USB, and it works.

## Fase 3 — Windows port

- [ ] `internal/osadapter/windows.go` (build tag `windows`) using
      UI Automation and PowerShell.
- [ ] Confirm STT/TTS/Brain layers compile and run identically.
- [ ] Windows code signing certificate.
- [ ] Installer (MSIX or simple zip + .exe).

**Exit criterion:** Same agent, same behaviour, on a Windows machine.

## Fase 4 — Android

- [ ] Separate Kotlin codebase under `android/`.
- [ ] Reuses the *protocol* (brain JSON, tool schemas) but
      reimplements activation, OS control via `AccessibilityService`.
- [ ] Voice loop via either on-device APIs or the same realtime/cloud
      brain.
- [ ] Distribution via Play Store.

**Exit criterion:** Same conversational UX on an Android phone.

## Things not on the roadmap (deliberate non-goals)

- Web UI as primary surface (it is not the product).
- Multi-user / cloud-hosted version (local-first by design).
- A "studio" for non-developers to write their own tools.
- Mobile iOS (deferred indefinitely; macOS covers the Apple ecosystem
  for now).
