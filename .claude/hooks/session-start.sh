#!/bin/bash
set -euo pipefail

# Only run in remote (Claude Code on the web) environments
if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi

# Install dependencies here as the project grows.
# Examples:
#   npm install
#   pip install -e ".[dev]"
#   cargo build

echo "Session start hook completed."
