# CLAUDE.md

## Project Overview

`my_prj` is a new project in its initial setup phase. The repository currently contains only a README.

## Repository Structure

```
.
├── CLAUDE.md          # AI assistant guidelines (this file)
└── README.md          # Project readme
```

## Development Setup

This is a Git-based project hosted on GitHub under `jcomellys/my_prj`.

- **Default branch:** `master`
- **Remote:** `origin`

## Git Workflow

- Create feature branches off `master` for new work.
- Write clear, descriptive commit messages.
- Push feature branches and open pull requests for review before merging to `master`.

## Conventions for AI Assistants

- Read existing files before suggesting modifications.
- Do not create unnecessary files; prefer editing existing ones.
- Keep changes minimal and focused on what is requested.
- Do not add speculative features, over-engineer, or introduce unused abstractions.
- Avoid introducing security vulnerabilities (command injection, XSS, SQL injection, etc.).
- When committing, stage specific files rather than using `git add -A`.
