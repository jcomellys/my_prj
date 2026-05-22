# Governance — continuity protocol (dual engine + external audit)

Purpose: the project must not stall when one engine is unavailable (e.g.
Claude runs out of tokens). This defines who may do what, so work continues
safely without an architect or a human babysitting every step.

Adopted 2026-05-22 (user proposal + Claude refinements). This file is the
durable source of truth; it supersedes ad-hoc arrangements in chat.

## Roles

**Claude** — primary architect.
- Implements on `claude/voice-mac-agent-V98uX`.
- Reviews and merges `codex/*` branches.
- Owns ROADMAP / HANDOFF / phase close-criteria and this governance doc.

**Codex** — primary validator on the real Mac; fallback implementer.
- Always: runs live validation, reports evidence via the git mailbox.
- When Claude is unavailable: MAY implement, but only on a separate
  `codex/<area>-<desc>` branch. Never pushes code to
  `claude/voice-mac-agent-V98uX` without explicit authorization.
- Keeps changes small, reversible, tested, documented.

**Gravity** — independent external auditor.
- Reviews `codex/*` branches before merge, and any sensitive change.
- Reports findings with severity, file/line, and a verdict.

## Continuity rule

When **Claude is available**:
  Claude implements → Codex validates on Mac → Gravity audits if needed.

When **Claude is unavailable / out of tokens**:
  Codex implements on `codex/*` → runs tests → writes a report →
  pushes the review branch → Gravity audits → **Claude reviews & merges
  on return.**

### Refinement 1 — the trunk stays protected (Claude addition)
Even after Gravity approves a `codex/*` branch, the merge into
`claude/voice-mac-agent-V98uX` waits for **Claude OR explicit user
authorization**. No merge to trunk happens without the architect or a
human in the loop. This bounds un-integrated debt and prevents a bad
auto-merge while the architect is away. If integration is urgent and
Claude is out, the **user** may authorize Codex to merge a specific,
Gravity-approved branch — that authorization must be explicit and per
branch.

### Refinement 2 — one report dialect (Claude addition)
The git mailbox (`PROTOCOL.md` + `inbox_codex.md`/`inbox_claude.md`) stays
the channel for **tasks and validations**. The implementation-report
fields below are used when Codex (or anyone) pushes a `codex/*` branch for
review; paste that block into `inbox_claude.md` as the report body.

## Hard rules (apply to everyone, always)

- Never read or print `.env` / secrets.
- Codex: no commits/pushes to the trunk branch without explicit per-task
  authorization.
- No destructive actions (no force-push, no history rewrite of shared
  branches, no `rm -rf` outside a scratch worktree).
- Use clean worktrees for live tests; never test against the user's dirty
  main checkout.
- Tests are mandatory before requesting review:
  - `go test ./...`
  - `go test -race -count=1 ./...`
- Live tests must include evidence: `you>`, `brain.response`,
  `tool.ok`/`tool.error`, and `cost`.
- Every phase has an explicit close criterion (see ROADMAP).

## Codex branch naming

`codex/<area>-<description>` — e.g.
`codex/time-format-spanish`, `codex/ux-streaming-blocks`,
`codex/ollama-tier-validation`, `codex/chrome-readability-fix`.

## Implementation report format (Codex → review)

```
BRANCH:
COMMIT:
SCOPE:
TESTS:
  - go test ./...:
  - go test -race -count=1 ./...:
EVIDENCE:
RISKS:
ROLLBACK:
VERDICT:
```

## Escalation / tie-breaking

- Code correctness disputes: Gravity's audit + the test suite decide.
- Product/scope/cost-vs-quality decisions: the user decides.
- If two `codex/*` branches conflict, Claude reconciles on return; until
  then they stay unmerged.

## Hard limits (stated honestly)

- Neither AI can wake the other; a human triggers each side.
- Claude's sandbox is ephemeral; on resume it re-reads HANDOFF.md + the
  mailbox + this file. There is no hidden memory.
