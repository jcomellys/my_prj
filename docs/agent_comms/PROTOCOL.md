# Agent comms — git mailbox

Two AIs build this project together. They have **no direct channel**; they
communicate by reading/writing files in this repo, synced through git. The
human triggers each side but no longer relays content by hand.

## Who is who
- **Claude** — runs in a cloud sandbox. Owns: architecture, code, audits,
  merges to `claude/voice-mac-agent-V98uX`. Cannot run mic/audio/hotkey/GUI.
- **Codex** — runs on the user's Mac mini M4. Owns: on-device validation
  (microphone, earcons, hotkey, Chrome, AppleScript) and reporting.

## The two mailboxes (single-writer each — never edit the other's file)
- `inbox_codex.md`  — **Claude writes**, Codex reads. Tasks for Codex.
- `inbox_claude.md` — **Codex writes**, Claude reads. Reports for Claude.

Single-writer-per-file avoids merge conflicts. Each side only appends to its
own outbound file (newest entry on top) and only reads the other's.

## Cadence (each side, when the human says "check your inbox")
1. `git pull --rebase origin claude/voice-mac-agent-V98uX`
2. Read your inbound file; find the top entry whose STATUS is NEW.
3. Do the work.
4. Append your result as a new top entry in your OUTBOUND file.
5. `git pull --rebase` then `git push` (Codex pushes ONLY the mailbox file
   and report artifacts — never code, unless the task explicitly grants it).

## Entry format
```
## <id> | <utc-timestamp> | <from>→<to> | STATUS
<compact body>
---
```
- `id`: short, increasing, e.g. C-014, X-014 (C=from Claude, X=from Codex).
- STATUS values: NEW → DONE (task finished) | SEEN (report read) | BLOCKED.
- The reader acknowledges by flipping the STATUS token **in place** on the
  entry's header line (this is the one allowed cross-edit). Optionally add a
  short parenthetical note after the STATUS. Do not rewrite the body.

## Task body shape (Claude → Codex)
```
TASK: <one line>
COMMIT: <sha>
RUN:
  <exact copy-paste commands>
PASS_IF:
  - <verifiable criterion>
REPORT:
  - <exact field to return>
CONSTRAINTS: no commit, no push of code, no read .env, no touch real config
```

## Report body shape (Codex → Claude)
```
RE: <task id>
COMMIT_TESTED: <sha>
RESULTS:
  - <test>: PASS|FAIL — <1-line detail>
DIFF_AUDIT: <findings | none>
COST: $X.XXXX
BLOCKERS: <none | list>
VERDICT: <one line>
```

## Hard limits (state honestly, do not pretend otherwise)
- Neither agent can trigger the other. A human (or a scheduled loop) must
  invoke each side to "check your inbox".
- Claude's sandbox is ephemeral; its memory does not persist between
  sessions. On resume, Claude re-reads docs/HANDOFF.md + this mailbox.
- The mailbox is the source of truth for in-flight work, not chat memory.
