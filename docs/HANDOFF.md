# Handoff document — for the next AI or developer

Read this **first**. It is intentionally exhaustive so you can be productive
in a single read without spelunking through commits.

## What this project is

A voice-controlled computer agent that lets anyone — including people who
cannot see, type, or use a keyboard — operate their computer by speaking.
The agent listens, thinks (via an LLM "brain" that is swappable), and acts
(via tools that wrap the operating system).

The product is humanitarian in intent. A free tier (all-local models)
must always exist so that users without money can still use it. A paid
tier exists for users who want premium voice latency and stronger reasoning.

## The user and how we coordinate

The user (jcomellys) is a non-developer with a strong product vision. They
have a **Mac mini M4 running macOS Tahoe (macOS 26)**. They are often
travelling and operate from an Android phone, so they cannot test on the
Mac mini continuously.

Coordination model:
- We work on branch `claude/voice-mac-agent-V98uX`.
- The agent (you) writes code in the cloud sandbox (Linux), which can
  build and vet but **cannot run audio, macOS APIs, or hotkeys**.
- The user pulls and tests on the Mac mini when they can.
- They paste logs and screenshots; you fix from there.

Honesty mandate: the user explicitly asked for "the truth and nothing but
the truth". Do not oversell timelines. Do not claim "1 week" when the
honest answer is "3 months for that quality". Do not skip risks.

## Current state — fases 0.1 through 0.4 validated live on the Mac (2026-05-22)

All of these are confirmed working in real voice sessions on the user's
Mac mini M4 / macOS Tahoe, coordinated with Codex via the git mailbox
(docs/agent_comms/):

- 0.1 typing + GPT-5 + tools; 0.2 voice (whisper); 0.3.1 vision
  (screenshot); 0.3.2 physical activators (hotkey + USB);
  0.3.3 Chrome via AppleScript; activation-feedback-UX (earcons +
  listen timeout); 0.4 two-tier router + monthly budget hard-stop.
- Router proof (X-004): simple turn stayed on gpt-5-mini ($0.0012),
  educational turn escalated to gpt-5 ($0.052). Cheap default, quality-
  first escalation for teaching/analysis/planning/code/math.
- Earcons (Tink/Pop), silent-turn skip with visible note, activator-
  aware startup hint, all live-validated.

Next (user's product choice, not yet started): barge-in/stop-speaking
(Fase UX — queued from X-004), free tier validation (Ollama, fase 0.5),
or sub-agents (fase 1+, the "study a book / build an app" leap).

### How collaboration works now (read this)
Two AIs: Claude (cloud sandbox, writes code) and Codex (on the Mac, runs
live validation). They coordinate through the git mailbox in
docs/agent_comms/ (PROTOCOL.md + inbox_codex.md + inbox_claude.md),
single-writer per file. The human triggers each side ("check your inbox")
but no longer relays content. Neither AI can wake the other.

Continuity when Claude is out of tokens is governed by
docs/agent_comms/GOVERNANCE.md: Codex may implement on `codex/*` branches,
Gravity (external auditor) reviews, and the merge to the trunk branch
waits for Claude OR explicit user authorization. Read GOVERNANCE.md
before implementing as a fallback engine.

### Earlier note (historical)
Fase 0.3.2 added two physical activators behind the existing
`activator.Activator` interface: a macOS global hotkey profile
(`manos_libres`, default Ctrl+Option+Space) and an edge-triggered
USB-presence profile (`usb_switch`, default volume name `AGENT`).

Fase 0.1 is verified end-to-end on the user's Mac mini M4 / macOS Tahoe
(2026-05-19): user typed prompts, GPT-5 picked tools, Chrome opened,
macOS spoke replies, cost tracker recorded ~$0.045 across 3 utterances.

Fase 0.2 (voice input) is **coded and tests pass in CI** but has not yet
been run on the Mac. It adds `internal/stt/whisper.go` (sox → whisper.cpp
shell-out) and `internal/activator/enter.go` (press-Enter-to-talk). The
new `voice` profile in `config.example.yaml` wires everything. See
`docs/FASE_02_TESTING.md` for the install/test protocol the user (or
their on-device AI) should follow.

Three brains implemented (OpenAI, Anthropic with prompt caching, Ollama
for the free tier). Suite of unit tests covers orchestrator, brains,
tools, config, cost, and whisper helpers (~30 tests, all passing with
`-race`). Compiles clean for `linux/amd64` and `darwin/arm64`.

One end-to-end path works (verified on Mac):

```
user types at terminal
   ↓ (stt.Stdin — placeholder)
text
   ↓ (brain.OpenAI calling GPT-5)
assistant text + optional tool calls
   ↓ (tools.Registry → tools.OpenApp / AppleScript / Shell)
osadapter.MacOS (`open`, `osascript`, `/bin/sh`)
   ↓
tool result fed back to brain
   ↓
final assistant text
   ↓ (tts.MacOSSay using the `say` command)
audio out the Mac's speakers
```

The user has not yet run it on the Mac mini at the moment of writing this
doc (they were traveling). When they do, they will paste the log here and
we iterate.

## Architectural decisions and why

These are **load-bearing** decisions. Do not change them without reading
the chat history of why they exist.

### Go for the core, not Python
Python was tempting for iteration speed, but the requirements include
"commercial-grade, portable on a USB stick" — that means a single
self-contained binary, fast startup, no runtime, no shipping a Python
interpreter. Go gives that. We accept slower iteration as the cost.

If you ever feel the urge to switch back to Python, re-read the requirements
discussion in the conversation. The user explicitly chose Go knowing the
tradeoff.

### Pipeline voice first, Realtime later
OpenAI Realtime API is the dream UX but costs ~$10–15/hour of conversation.
That is incompatible with a free tier and a low-cost commercial tier. So
we build `voice.Pipeline` (STT + Brain + TTS as separate, swappable parts)
first. It supports the free tier natively. `voice.RealtimeVoice` will be a
second `voice.Provider` implementation later, behind the same interface.

### macOS native operations, not pixel-based clicking
Computer-use models that move the cursor and click pixels are slow,
fragile, and redundant on macOS, which already exposes apps via
AppleScript and the Accessibility API. We deliberately picked the
high-level route:

- `open -a "App Name"` for app launch.
- `osascript -e "..."` for in-app automation (covers Chrome, Word, Pages,
  Finder, Mail, Music, Keynote, Numbers natively).
- Accessibility API (AX) reads for app state — to be added in fase 0.3.
- Pixel clicking only as a fallback if all else fails.

This is *especially* the right call for a blind user: AppleScript and AX
expose semantic structure that pixel-clicking destroys.

### Chrome via the DevTools Protocol, not Safari and not Playwright
The user explicitly said "no Safari, no Apple-app lock-in — Chrome." Chrome
also exposes the Chrome DevTools Protocol (CDP) which we can drive in Go
without an external runtime. We will **not** use Playwright because it
requires Node and downloads multi-hundred-megabyte browser binaries — bad
for our portable-binary goal. CDP comes in fase 0.3.

### Two-tier brain by default
Calling GPT-5 or Claude Opus for every utterance is wasteful when 80% of
utterances are simple ("abre Chrome"). The plan is:

- A fast/cheap "router" model (Claude Haiku 4.5 or GPT-5-mini) handles
  intent classification and trivial tool calls.
- A deep model (Opus, GPT-5) is invoked only when the router decides the
  task is complex.

This is wired into the architecture but not yet implemented — see fase 0.4.

### Prompt caching from day 1
Anthropic and OpenAI both support prompt caching. The system prompt and
tool definitions go in the cached region. The OpenAI implementation
currently reads the `cached_tokens` from the API response for cost
tracking; we have not yet aggressively structured the prompt for cache
hits. That work belongs to fase 0.4.

### Free tier specifics
- LLM: Ollama with Llama 3.3 8B (fits comfortably on Mac mini M4 base,
  ~30–50 tok/s).
- STT: whisper.cpp with Metal acceleration (real-time or faster on M4).
- TTS: macOS `say` with a high-quality Tahoe voice. Already wired.
- Wake word (later): Picovoice Porcupine, free for personal use.

### One USB = the portable product = the on-off "switch"
The user wanted a physical on/off switch as an accessibility feature.
Elegant idea: insertion of the USB stick *is* the on switch; removal is
off. Many other activators (hotkey, wake word, BLE button, 3.5mm
accessibility switch) will be supported behind the `activator.Activator`
interface, but USB presence is the headline activator for the commercial
product.

## Repository layout

```
.
├── cmd/agent/main.go            entry point; wires everything via config
├── config.example.yaml          three profiles: free, cheap, premium
├── .env.example                 API keys; .env is gitignored
├── Makefile                     build, run, test, fmt, vet, tidy, clean
├── docs/
│   ├── HANDOFF.md               THIS FILE
│   ├── ARCHITECTURE.md          design rationale
│   ├── ROADMAP.md               phased plan
│   └── TESTING.md               step-by-step user test plan
├── internal/
│   ├── activator/               how the user triggers "start listening"
│   ├── activator/               also: SupportsBargeIn() for interrupt
│   ├── agent/                   orchestrator (router/escalate, budget), prompts
│   ├── audiocue/                earcons (mic open/close) via afplay
│   ├── brain/                   LLM iface + OpenAI/Anthropic/Ollama/Mock
│   ├── cost/                    NDJSON cost tracker + pricing
│   ├── osadapter/               OS control surface (macos.go, stub.go)
│   ├── stt/                     STT iface + stdin + whisper.cpp
│   ├── subagent/                autonomous Runner for delegate_task (Fase 1)
│   ├── tools/                   registry + open_app/applescript/shell/
│   │                            screenshot/show_cost/read_file/write_file/delegate_task
│   ├── tts/                     text-to-speech interface + macOS say
│   └── voice/                   Pipeline (STT→brain→TTS) + barge-in
├── go.mod
└── README.md
```

## How to extend it (the common cases)

### Add a new brain (e.g., Claude)
1. Create `internal/brain/anthropic.go`.
2. Implement `brain.Brain`: `Name()` and `Chat(ctx, messages, tools)`.
3. Map `brain.Message` and `brain.ToolSpec` to Anthropic's `messages` API
   shape. Anthropic supports prompt caching via `cache_control` — use it
   on the system message and tool list.
4. Wire it in `cmd/agent/main.go` → `buildBrain`.
5. Add a config example under `profiles:` in `config.example.yaml`.

### Add a new tool (e.g., `screenshot`)
1. Create `internal/tools/screenshot.go`.
2. Implement `tools.Tool`: `Spec()` returning a `brain.ToolSpec` (JSON
   Schema for arguments) and `Execute(ctx, argsJSON)`.
3. The schema is the *only* thing the brain sees about your tool — write
   the description from the brain's point of view, not the user's.
4. Register it in `cmd/agent/main.go` based on a flag in `tools:` section
   of config.
5. Make sure long outputs are truncated to ~2000 chars (see `shell.go`)
   so they do not blow up token budgets.

### Add a new STT (e.g., whisper.cpp)
1. Create `internal/stt/whisper.go`.
2. Implement `stt.STT`: `Name()` and `Listen(ctx) (string, error)`.
3. `Listen` should block until the user finishes a phrase (silence
   detection), then return the transcript. Honor ctx cancellation.
4. Wire in `cmd/agent/main.go` → `buildVoice` switch on `vc.STT.Provider`.

### Add a new OS adapter (e.g., Windows)
1. Create `internal/osadapter/windows.go` with `//go:build windows`.
2. Move the `Stub` to a `linux.go` (or rename) so darwin/windows each
   have a real adapter and linux remains the stub.
3. Implement the same `Adapter` interface using UI Automation /
   PowerShell.

## What is NOT done yet — common pitfalls

- **No tests.** Add unit tests as you go. Mocks already exist
  (`brain.Mock`, `activator.AlwaysOn`) so any layer is testable in
  isolation without hardware.
- **No cost tracker yet.** `Usage` is captured in `brain.Response` and
  logged via slog, but nothing persists it to SQLite. Planned for
  fase 0.4. Pricing tables must live somewhere — probably
  `internal/cost/pricing.go`.
- **No streaming.** The current `Brain.Chat` is request/response. TTS
  starts only when the brain finishes. Acceptable for fase 0; will need
  streaming for "colleague feel". Add a `ChatStream` method later or a
  channel-based variant; don't break the synchronous one.
- **No prompt caching applied.** OpenAI's prompt cache is automatic for
  identical prefixes ≥1024 tokens. Our system prompt + tool definitions
  are too short to trigger it today. Either lengthen the system prompt
  with stable preferences, or wait for Claude where caching is explicit.
- **No conversation summarization.** `history` grows unbounded. Add a
  summarize-at-N-turns mechanism before long sessions become expensive.
- **No real activator yet.** `activator.AlwaysOn` is a no-op; the
  effective activator is the user pressing Enter at the stdin prompt.
  Global hotkey via `golang.design/x/hotkey` is the next real step (must
  be on darwin only — uses Cocoa).
- **No screenshot tool.** Important for "describe my screen" use cases
  and for the blind-user persona. Will need image-capable model on the
  brain side; OpenAI Chat Completions handles images via `content` array
  with `image_url` parts.
- **macOS permissions.** The first run of any tool will pop a system
  dialog asking for Accessibility / Automation permission. The user must
  click Allow. Permissions are keyed to the binary's path — moving the
  binary re-prompts. Document this clearly in TESTING.md (already done).

## Things that will surprise you

- **macOS Tahoe is very new.** AX API and some `osascript` behaviors may
  differ from Sequoia. When something breaks on the user's Mac mini, do
  not assume the docs you trained on are current. Verify on-device.
- **The user is non-technical and on a phone.** Write commands they can
  copy-paste. Do not ask them to "edit lines 42–47"; tell them
  "open config.yaml, change `active_profile: free` to `active_profile:
  premium`". When debugging, request a complete log paste, not a
  selective excerpt — they cannot judge what is relevant.
- **The user is paying for API tokens.** Every Brain call costs them
  cents. When asking them to test, design the test to fail fast and
  cheap. Use `mock` brain when you only need to test plumbing.
- **They explicitly want professional code.** No half-shipped half-mocked
  intermediate states. Keep `master` and the working branch always
  buildable.

## Conversation conventions with the user

The user writes Spanish from a phone with typos. Read past the typos.
When you respond, use Spanish, keep it under ~300 words unless they
asked for depth, and avoid bullet-point overload — they prefer dense
paragraphs that scan well on a phone screen.

Honesty over enthusiasm. If something will take 3 months, say 3 months.
If a tool is more fragile than it looks, say so before they invest.

## Where you came from

This document was produced at the end of the first work session, which
was bootstrapping the project from an empty repo. The full design
discussion happened in chat before any code was written; recover it from
the conversation history if you can. If the chat is gone, this doc and
`ARCHITECTURE.md` together should be enough to continue without it.

Welcome aboard. Build something that matters.
