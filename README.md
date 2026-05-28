# Voice Mac Agent

> A voice-first computer agent designed to give everyone — including people
> who cannot see, cannot type, or cannot use a keyboard — full conversational
> control of their computer. Cross-platform by architecture; macOS first.

This project's goal is humanitarian: make the computer usable by anyone
through natural conversation, with a brain (LLM) that is swappable so the
system never becomes obsolete as AI advances, and with a free tier so that
no one is priced out.

**Status:** Fase 1 — sub-agents (increment 1) in code; live voice validation
pending. Everything through Fase 0.4.2A has been validated live on the Mac
mini. See [`docs/ROADMAP.md`](docs/ROADMAP.md) for the full phased plan and
[`docs/HANDOFF.md`](docs/HANDOFF.md) if you are an AI or developer picking
this project up.

## What works today

**Voice loop (validated live):**
- Layered architecture: every provider is an interchangeable interface
  (Brain, STT, TTS, Activator, OSAdapter, Tools) chosen by config.
- Real speech in/out: microphone → `whisper.cpp` (STT) → Brain → macOS
  `say` (TTS), with silence-based end-of-utterance detection.
- Earcons: a tone when the mic opens and closes, so a non-sighted user
  always knows the listening state.
- Global hotkey activation (⌃⌥Space) and USB-presence activation.
- Barge-in: one gesture cuts the agent off mid-thought or mid-speech and
  starts the next turn — it never leaves the user talking over a monologue.
- Resilient by design: a transient brain/network error is spoken and the
  agent keeps listening; it does not terminate (a non-sighted user cannot
  restart it from a keyboard).

**Brains (swappable):**
- OpenAI (GPT-5 / GPT-5-mini) and Anthropic (Claude, with prompt caching)
  via raw HTTP. Ollama for the local $0 tier (EXPERIMENTAL — see roadmap).
- Two-tier routing: a cheap brain handles simple turns and escalates to a
  deep brain for teaching/analysis/code/math. Cost attributed per tier.
- Mock brain for tests and offline development.

**Cost control:**
- Append-only NDJSON cost log; `show_cost` tool answers "cuánto llevo
  gastado". Monthly-budget hard-stop plus a one-time spoken warning.
- Bounded answers in blocks (≈ −70% cost on long educational replies).

**Tools the brain can call:**
- `open_app`, `run_applescript`, `run_shell` (allowlist-gated),
  `screenshot`, `show_cost`, `read_file`, `write_file`.
- `delegate_task` — hands a heavy multi-step job to an autonomous
  sub-agent (Fase 1, increment 1; live validation in progress).

Compiles clean for linux/amd64 (CI) and darwin/arm64 (Mac mini).
Configuration profiles: `free`, `cheap`, `cheap_claude`, `voice`,
`manos_libres`, `usb_switch`, `premium`.

## Quick start (on the Mac mini)

```bash
# 1. Install Go if needed
brew install go

# 2. Clone (or pull) the branch
git clone <repo>
cd my_prj
git checkout claude/voice-mac-agent-V98uX

# 3. Set up env and config
cp .env.example .env
# edit .env: set OPENAI_API_KEY=sk-...
cp config.example.yaml config.yaml
# edit config.yaml: change active_profile to "premium" (uses gpt-5)

# 4. Run
make run

# 5. Talk to the agent
# Type a message at the "you>" prompt and press Enter.
# Try: "abre Google Chrome"
# The agent will call the open_app tool, Chrome opens, and macOS speaks back.
```

See [`docs/TESTING.md`](docs/TESTING.md) for the full step-by-step test plan.

## Vision

We are building for users who today cannot use a computer the way most
people do:

- People with no vision: cannot see icons or windows.
- People with no arm/hand mobility: cannot use a mouse or keyboard.
- People who never learned to type or use software.
- Anyone who prefers conversation over clicks.

The product is a **conversational colleague**: it understands intent, takes
action, narrates what it did, asks one question when ambiguous, and runs
on the user's own machine.

## Principles

1. **Free tier must exist.** Tier 0 runs entirely on the user's Mac with a
   local LLM (Ollama), local STT (whisper.cpp) and local TTS (macOS say).
   Zero ongoing cost.
2. **Brain is swappable.** GPT, Claude, Gemini, Ollama, future models —
   each is an implementation of a single `Brain` interface. The product
   never goes obsolete with model progress.
3. **Token-efficient by design.** Cheap router + escalation, prompt caching,
   summarization, screenshots on demand. Built into the architecture, not
   bolted on.
4. **Portable.** The final binary is a single self-contained file that can
   eventually run from a USB stick. No installer.
5. **Accessibility first.** Voice is the primary UI. Visual UI is a thin
   optional status panel.
6. **Permission-aware.** Every tool declares the OS permissions it needs.
   Nothing happens silently behind the user's back.

## Architecture (1-paragraph summary)

The agent is a Go orchestrator that owns a conversation loop. It talks to
the user through a `voice.Provider` (today: `Pipeline` = STT + TTS), and
talks to an LLM through a `brain.Brain`. The brain may request tool calls;
the orchestrator dispatches them through a `tools.Registry` whose tools
sit on top of an `osadapter.Adapter` (today: macOS). Each layer is an
interface — providers are chosen by config, not by code change.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full design.

## License

Not yet chosen. The project's intent is humanitarian — likely an OSI-approved
license (MIT or Apache-2.0) once we are out of fase 0.

## For the next AI / developer

Read [`docs/HANDOFF.md`](docs/HANDOFF.md) first. It is written specifically
to bring a fresh agent up to speed in one read: where we are, why each
decision was made, what is in flight, and what to do next.
