# Testing — step by step on the Mac mini

These instructions are written for the user. Copy-paste each block into
Terminal on the Mac mini in order. Stop at the first error and paste it
back in chat; do not try to fix it yourself.

## One-time setup

```bash
# 1. Install Homebrew (skip if already installed)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 2. Install Go
brew install go
go version    # must show go1.22 or newer

# 3. Get the code on this branch
cd ~
git clone https://github.com/jcomellys/my_prj.git    # or your remote URL
cd my_prj
git checkout claude/voice-mac-agent-V98uX
git pull origin claude/voice-mac-agent-V98uX

# 4. Create your env file and put your API key in it
cp .env.example .env
open -e .env    # opens in TextEdit; replace sk-... with your real key, save, close

# 5. Create your config file
cp config.example.yaml config.yaml
```

By default `config.yaml` uses the `free` profile which expects Ollama
(not wired yet in fase 0.1). For the first run, edit `config.yaml` and
change the top line to:

```yaml
active_profile: premium
```

Save and close.

## First smoke test — mock brain, no API calls

This proves the loop works without spending any OpenAI tokens.

```bash
cd ~/my_prj

# Temporarily switch the brain to "mock" for this test.
# (Open config.yaml and under profiles.premium.brain change provider: openai
# to provider: mock — or copy/paste the snippet below into a shell.)

# Or just run with a tiny one-off config:
cat > config.test.yaml <<'EOF'
active_profile: test
profiles:
  test:
    voice:
      mode: pipeline
      stt: { provider: stdin }
      tts: { provider: macos_say }
    brain:
      provider: mock
    activator: { kind: stdin }
tools:
  open_app: { enabled: true }
  applescript: { enabled: true }
  shell:
    enabled: true
    allow_unrestricted: false
    allowlist: [ "open " ]
cost: { enabled: true }
EOF

go run ./cmd/agent --config config.test.yaml
```

You should see:

```
config.loaded profile=test
brain.ready name=mock
tools.registered count=3
voice.ready name=pipeline(stt=stdin,tts=macos_say,act=always_on)
Agente listo. Escribe lo que quieras decir y presiona Enter. Ctrl-C para salir.
you>
```

Type: `abre Google Chrome` and press Enter.

Expected:
- Chrome opens.
- The Mac says aloud "Opened Google Chrome." (or similar)

Ctrl-C to exit.

If it asks for Accessibility / Automation permission: click Allow, then
re-run. macOS keeps permissions per-binary; you'll only see the dialog
the first time.

## Second test — real GPT-5

Edit `config.yaml` and ensure under `profiles.premium.brain` you have:
```yaml
    brain:
      provider: openai
      model: gpt-5
      temperature: 0.3
      max_tokens: 4096
```

Then:

```bash
go run ./cmd/agent --config config.yaml
```

Try these prompts one at a time:

1. `abre Google Chrome`
   → expects: Chrome opens, agent confirms briefly.

2. `qué hora es`
   → expects: agent answers in voice from its own knowledge (no tool).

3. `abre Chrome y navega a la wikipedia de Albert Einstein`
   → expects: agent uses `run_applescript` to drive Chrome's "open
     location" command.

If any step fails:
- Copy the full terminal output from the moment you typed the prompt
  to the error, including the `brain.response` log lines.
- Paste it in chat.

## How to capture logs cleanly

```bash
go run ./cmd/agent --config config.yaml -v 2>&1 | tee run.log
```

After the session, `run.log` has everything. Send the relevant portion.

## How to update the code

```bash
cd ~/my_prj
git pull origin claude/voice-mac-agent-V98uX
```

Then re-run. No rebuild is needed if you use `go run`; if you used
`make build`, rerun `make build`.

## Common issues and fixes

| Symptom | Cause | Fix |
|---|---|---|
| `OPENAI_API_KEY is unset` | `.env` not loaded or key missing | Re-check `.env` file, no quotes, no spaces around `=` |
| `osascript` permission dialog | macOS asking for Automation permission | Click Allow; re-run |
| `open -a "Google Chrome"` fails | Chrome not installed under that exact name | Check: `ls /Applications | grep -i chrome` |
| Long pause then no response | API request hanging | Check internet; OpenAI status; try `--config config.test.yaml` to confirm loop works without network |
| `say: command not found` | Highly unlikely on macOS | Reinstall macOS Command Line Tools: `xcode-select --install` |

## What to report when something is wrong

Paste:
1. The exact command you ran.
2. The full terminal output (or attach `run.log`).
3. Which step of which section you were on.
4. What you expected vs what happened.

Do **not** rewrite parts of the code yourself unless instructed.
