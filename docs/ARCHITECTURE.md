# Architecture

## One-paragraph summary

The agent is a Go program that owns a conversation loop. A
`voice.Provider` captures user speech and plays back the assistant's
reply. A `brain.Brain` (any LLM provider) decides what to say and may
request tool calls. A `tools.Registry` dispatches those calls. Tools
operate through an `osadapter.Adapter` that wraps the operating system's
control surface. Every layer is an interface; concrete providers are
chosen by config. Adding a new LLM, a new TTS, a new tool, or porting to
Windows means writing one new file that implements one interface.

## Diagram

```
                    ┌───────────────────────────────────┐
                    │           cmd/agent/main           │
                    │  (config → wire deps → run)         │
                    └──────────────────┬─────────────────┘
                                       │
                                       ▼
                    ┌───────────────────────────────────┐
                    │   internal/agent/Orchestrator      │
                    │  - holds conversation history       │
                    │  - drives N rounds of brain+tools   │
                    │  - implements voice.Handler         │
                    └───┬───────────────┬───────────────┬─┘
                        │               │               │
            ┌───────────▼──┐  ┌─────────▼────────┐  ┌──▼──────────┐
            │ voice.Provider│  │   brain.Brain     │ │tools.Registry│
            │  (interface)  │  │  (interface)      │ │              │
            └───────┬───────┘  └─────────┬─────────┘ └──┬───────────┘
                    │                    │              │
       ┌────────────┼────────┐   ┌───────┴───────┐  ┌───▼───────────────┐
       │            │        │   │               │  │ tools.Tool        │
       │            │        │   │               │  │ - open_app        │
       │   ┌────────▼─────┐  │   │ ┌──────────┐  │  │ - run_applescript │
       │   │ Pipeline     │  │   │ │ OpenAI   │  │  │ - run_shell       │
       │   │  stt+tts+act │  │   │ │ (GPT-5)  │  │  └───┬───────────────┘
       │   └─┬──────┬─────┘  │   │ └──────────┘  │      │
       │     │      │   ┌────┘   │ ┌──────────┐  │      ▼
       │  ┌──▼──┐ ┌─▼──┐ │       │ │ Mock     │  │ ┌───────────────────┐
       │  │ STT │ │TTS │ │       │ └──────────┘  │ │osadapter.Adapter  │
       │  └──┬──┘ └──┬─┘ │       │ (anthropic,    │ │  (interface)      │
       │     │      │   │       │   gemini,      │ └──┬────────────────┘
       │     │      │   │       │   ollama: TBD) │    │
       │  ┌──▼──┐ ┌─▼──┐│       └────────────────┘    │
       │  │Stdin│ │macOS││                        ┌───▼───────────┐
       │  └─────┘ │ say ││                        │  macOS (real) │
       │          └─────┘│                        │  Stub (linux) │
       │ ┌───────────────▼─┐                      └───────────────┘
       │ │ activator.AlwaysOn (placeholder)
       │ │ later: Hotkey, WakeWord, USBPresence, BLEButton, Switch
       │ └───────────────────
       │
       │  Realtime (fase 1): a second voice.Provider implementing the
       │  same interface; routes voice and brain through OpenAI Realtime.
```

## Core interfaces

### `brain.Brain`
```go
type Brain interface {
    Name() string
    Chat(ctx, messages []Message, tools []ToolSpec) (*Response, error)
}
```
- One `Message` per conversation turn (`system|user|assistant|tool`).
- `Response` carries assistant text, requested tool calls, and `Usage`
  (tokens in / out / cached) for cost tracking.
- Stateless: the orchestrator owns the conversation history and passes
  it on every call. This makes brains trivially swappable mid-session.

### `tools.Tool`
```go
type Tool interface {
    Spec() brain.ToolSpec      // JSON-Schema description for the brain
    Execute(ctx, argsJSON string) (string, error)
}
```
- The schema is the *only* surface the brain sees. Write the description
  for the brain, not for the user.
- Returns a short string the brain can read. Long outputs must be
  truncated (we cap at ~2000 chars) so token budgets stay bounded.

### `voice.Provider`
```go
type Provider interface {
    Name() string
    Start(ctx, h Handler) error
}
type Handler interface {
    HandleUtterance(ctx, userText string) (reply string, err error)
}
```
- The provider owns the loop. The orchestrator is the `Handler`.
- This lets `Pipeline` (STT/TTS) and `RealtimeVoice` (single bidi WS) both
  fit the same interface even though their internals are very different.

### `osadapter.Adapter`
```go
type Adapter interface {
    Name() string
    OpenApp(ctx, name string) error
    RunShell(ctx, cmd string) (string, error)
    RunAppleScript(ctx, script string) (string, error)
}
```
- The *only* place that knows about the OS. Adding Windows or Linux means
  one new file behind a build tag.
- Build tags: `//go:build darwin` for `macos.go`, `//go:build !darwin`
  for `stub.go`. Both expose `NewMacOS()` so `main.go` does not branch.

### `activator.Activator`
```go
type Activator interface {
    Name() string
    WaitForActivation(ctx) error
}
```
- Abstracts "how does the user say 'I want to talk now'".
- Today: `AlwaysOn` (no-op, paired with stdin STT).
- Planned: `HotkeyActivator`, `WakeWordActivator`, `USBPresenceActivator`,
  `BLEButtonActivator`, `AccessibilitySwitchActivator`.

## Conversation loop

```
1.  voice.Provider.Start(ctx, orchestrator)
2.  loop:
3.    activator.WaitForActivation         (no-op in fase 0.1)
4.    stt.Listen                          → user text
5.    orchestrator.HandleUtterance(text):
6.      append user message to history
7.      for round = 0..MaxRounds:
8.        brain.Chat(history, tool_specs)
9.        append assistant message (with tool_calls) to history
10.       if no tool_calls: return assistant text
11.       for each tool_call:
12.         tool.Execute(args) → result
13.         append tool message to history
14.     return "could not finish in N steps"
15.   tts.Speak(reply)
```

The loop is in `internal/voice/pipeline.go` and the inner brain+tool loop
is in `internal/agent/orchestrator.go`.

## Configuration

`config.yaml` selects an `active_profile`. Each profile bundles:
- `voice.mode` — `pipeline` (today) or `realtime` (later).
- `voice.stt.provider` — `stdin` | `whisper_cpp` | `macos_speech` | …
- `voice.tts.provider` — `macos_say` | `piper` | `elevenlabs` | …
- `brain.provider` + `brain.model` — `openai` | `anthropic` | `gemini` |
  `ollama` | `mock`.
- `activator.kind` — `stdin` | `hotkey` | `wake_word` | `usb_presence` | …

Three example profiles ship:
- `free` — Ollama + local STT + macOS say.
- `cheap` — gpt-5-mini cloud + local STT + macOS say.
- `premium` — gpt-5 cloud + best STT + macOS say.

## Token-efficiency strategy

Built in, not added later:

1. **Per-tool output cap** (`tools/shell.go`, `tools/applescript.go`):
   2000-char truncation prevents large command output from poisoning
   the context.
2. **Cost tracking surface ready** (`brain.Usage`): every brain response
   carries token counts; persisting them to SQLite is a small task in
   fase 0.4.
3. **Two-tier brain (planned 0.4)**: cheap router classifies intent,
   escalates to expensive model only when needed.
4. **Prompt caching (planned 0.4)**: Anthropic explicitly via
   `cache_control`; OpenAI implicitly via stable prompt prefixes ≥1024
   tokens.
5. **Conversation summarization (planned later)**: after N turns,
   compress old history into a single system note.
6. **Screenshots on demand (planned 0.3)**: brain requests a screenshot
   tool; we never auto-attach.
7. **Tool descriptions over context**: tools fetch what the brain needs
   (`read_page(selector)`), avoiding pasting whole web pages.

## Cross-platform port strategy

When the time comes:

- **Capa 5 (osadapter)** is the only file that must be rewritten per OS.
  `macos.go` is darwin, `stub.go` is everything else today. A future
  `windows.go` (build tag `windows`) replaces the stub on that OS.
- **Capa voice/STT/TTS** has portable implementations possible everywhere
  (Whisper, Ollama, Piper) plus OS-specific shortcuts (macOS `say`,
  Windows SAPI). Each is just another provider.
- **Activator** is also platform-specific (different APIs for hotkeys,
  Bluetooth, etc.) but the interface holds.
- **Brain, orchestrator, tools, config** are 100% portable.

Android is the exception. Android cannot run a Go binary as a regular
app — apps must be Kotlin/Java on the JVM/ART. We will write a separate
Android client in Kotlin that reuses **the same protocol** (talking to a
local brain via API) but reimplements activation and OS control with
`AccessibilityService`. Treated as a separate codebase.

## Security and permissions

- Tool allowlist (`tools/shell.go`) prevents the brain from running
  arbitrary shell commands silently. Default profile keeps
  `allow_unrestricted: false` and only lets `open ` and `osascript `
  through unprompted.
- API keys live in `.env` (gitignored) for fase 0–1. They will move to
  the macOS Keychain in fase 2 for commercial distribution.
- macOS will prompt for Accessibility / Automation permission on first
  use of `osascript` against another app. Permissions are tied to the
  binary path: moving the binary re-prompts. This is a Gatekeeper
  feature, not a bug.
- The agent itself is *not* a system extension. It needs no
  super-user privileges. Everything is user-space.

## What was deliberately *not* built (yet)

- **Realtime voice** — too expensive for fase 0; comes as a peer
  `voice.Provider` in fase 1.
- **Pixel-based computer use** — fragile; only ever as a fallback.
- **A graphical app** — voice is the UI; a tiny menubar shell comes in
  fase 2 as a status indicator only.
- **Cloud sync / accounts** — the product is local-first by design.
- **Mobile** — separate codebase; not started.
- **Auto-updater, telemetry, crash reporting** — fase 2 (commercial).
