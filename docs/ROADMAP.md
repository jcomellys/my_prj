# Roadmap

Phases are ordered so each one produces something working end-to-end and
testable on the user's Mac mini before moving on. No phase is "skeleton
code only" — every phase ends with a real user-visible behaviour.

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

- [ ] Integrate `whisper.cpp` with Metal acceleration (best free option
      on M4).
  - Option A: shell out to a `whisper-cli` binary the user installs via
    Homebrew (`brew install whisper-cpp`). Simpler.
  - Option B: link against `libwhisper` via cgo. More portable in a
    single binary but harder.
  - Decision: start with Option A; revisit if portability requires it.
- [ ] Microphone capture in Go (`malgo` or `portaudio` via cgo, or
      `cmd/say`-style native helper).
- [ ] Silence-based end-of-turn detection.
- [ ] `internal/stt/whisper.go` implementing `stt.STT`.
- [ ] Update `cmd/agent/main.go` switch to wire `whisper_cpp`.

**Exit criterion:** User says "abre Chrome" out loud and the agent
hears it. Same loop, no typing.

## Fase 0.3 — Real activation, browser control, screenshot

- [ ] Global hotkey activator using `golang.design/x/hotkey`. macOS
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

## Fase 0.4 — Two-tier brain + cost tracking

- [ ] `internal/brain/anthropic.go` — Claude with prompt caching.
- [ ] Router/escalation: agent picks `gpt-5-mini` (or Haiku) first;
      a `delegate_to_deep_brain` tool calls the expensive model when
      needed. The expensive model returns text that the cheap model
      narrates to the user.
- [ ] `internal/cost/tracker.go` — SQLite (modernc.org/sqlite, pure
      Go) recording every brain call: provider, model, tokens in/out,
      cached tokens, USD cost, session id, timestamp.
- [ ] Pricing table (`internal/cost/pricing.go`) — maintainable map of
      model → $ per 1M tokens (input, output, cached).
- [ ] Tool: `show_cost_today` so the user can ask "cuánto llevo
      gastado hoy".
- [ ] Hard-stop when `monthly_budget_usd` exceeded (configurable).

**Exit criterion:** User can run a 30-minute session and see real cost
in the cost log. Cost on `cheap` profile is well below $1/hour.

## Fase 0.5 — Free tier complete (Ollama)

- [ ] `internal/brain/ollama.go` — local LLM via Ollama HTTP API.
      Must support tool calling (Llama 3.3 and Qwen 2.5 both do).
- [ ] Verify Llama 3.3 8B can reliably invoke `open_app` and a
      handful of AppleScript-based tools on M4.
- [ ] Document tradeoff: local models are slower and weaker; the user
      should not expect "abre Word y escribe un ensayo sobre Newton"
      to work perfectly on local. Simple control: yes. Heavy
      reasoning: escalate.
- [ ] Update the `free` profile to use Ollama by default.

**Exit criterion:** With no API keys configured, the `free` profile
gives a working agent for the "open / search / navigate" use cases.

## Fase 1 — Realtime voice as premium option

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
