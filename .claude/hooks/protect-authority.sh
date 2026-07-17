#!/usr/bin/env bash
# PreToolUse hook: block agent writes to human-decision-only paths (AGENTS.md hard rule).
# Exit 2 = block the tool call; stderr is fed back to the agent.
set -euo pipefail
input="$(cat)"
path="$(printf '%s' "$input" | jq -r '.tool_input.file_path // .tool_input.notebook_path // empty' 2>/dev/null || true)"
[ -n "$path" ] || exit 0

case "$path" in
  *docs/product/00-sources/*|*docs/product/18-philosophy/decisions.md)
    echo "BLOCKED: $path changes by human decision only (AGENTS.md hard rules). Propose the change in your PR/finding instead." >&2
    exit 2 ;;
esac
exit 0
