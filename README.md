# Voice Mac Agent

> A voice-first computer agent designed to give everyone — including people
> who cannot see, cannot type, or cannot use a keyboard — full conversational
> control of their computer. Cross-platform by architecture; macOS first.

This project's goal is humanitarian: make the computer usable by anyone
through natural conversation, with a brain (LLM) that is swappable so the
system never becomes obsolete as AI advances, and with a free tier so that
no one is priced out.

**Status:** Fase 0.1 — skeleton with one end-to-end path working.
See [`docs/ROADMAP.md`](docs/ROADMAP.md) for the phased plan and
[`docs/HANDOFF.md`](docs/HANDOFF.md) if you are an AI or developer picking
this project up.

## What works today (fase 0.1)

- Layered architecture: every provider is an interchangeable interface.
- Pipeline voice mode: STT → Brain → TTS, each piece swappable.
- One real brain wired: OpenAI (GPT-5 and GPT-5-mini) via raw HTTP.
- Mock brain for tests and offline development.
- Three tools the brain can call:
  - `open_app` — launch any Mac application
  - `run_applescript` — full AppleScript dictionary of any scriptable app
  - `run_shell` — shell commands with an allowlist for safety
- macOS adapter using `open`, `osascript`, `/bin/sh`.
- Native macOS TTS via the `say` command (Tahoe voices are excellent).
- Stdin "STT" placeholder: you type, agent reads it aloud.
- Three configuration profiles: `free`, `cheap`, `premium`.
- Compiles clean for linux/amd64 (CI) and darwin/arm64 (Mac mini).

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
