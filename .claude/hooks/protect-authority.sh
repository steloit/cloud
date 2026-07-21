#!/usr/bin/env bash
# PreToolUse hook: block agent writes to human-decision-only paths (AGENTS.md hard rule).
# Exit 2 = block the tool call; stderr is fed back to the agent.
#
# Covers two call shapes:
#   1. Edit/Write/MultiEdit/NotebookEdit — structured and exact (.tool_input.file_path).
#   2. Bash — a command string. Until O6f this was NOT covered at all, so
#      `echo x >> decisions.md` wrote a founder-owned file unseen. Shape 2 raises the
#      floor against ACCIDENTS; it does not stop a determined agent. See "Residual
#      bypass surface" at the bottom, and never describe it as full enforcement.
set -euo pipefail

PROTECTED='docs/product/00-sources/|docs/product/18-philosophy/decisions\.md'
deny() {
  echo "BLOCKED: $1 changes by human decision only (AGENTS.md hard rules). Propose the change in your PR/finding instead." >&2
  exit 2
}

input="$(cat)"
path="$(printf '%s' "$input" | jq -r '.tool_input.file_path // .tool_input.notebook_path // empty' 2>/dev/null || true)"

# Shape 1: structured file-path tools.
if [ -n "$path" ]; then
  case "$path" in
    *docs/product/00-sources/*|*docs/product/18-philosophy/decisions.md) deny "$path" ;;
  esac
  exit 0
fi

# Shape 2: Bash. Only a protected path that is the TARGET of a write is blocked —
# reading these files (cat/grep/git log) is legitimate and must stay unimpeded, because
# a hook that fires on ordinary commands is a hook someone turns off.
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // empty' 2>/dev/null || true)"
[ -n "$cmd" ] || exit 0
printf '%s' "$cmd" | grep -qE "$PROTECTED" || exit 0

# a) redirect whose target is a protected path: > path, >> path, >"path", 1> path
if printf '%s' "$cmd" | grep -qE '[0-9]?>>?[[:space:]]*['"'"'"]?[^[:space:];|&]*('"$PROTECTED"')'; then
  deny "a protected path (shell redirect)"
fi
# b) in-place / destructive verbs naming a protected path
if printf '%s' "$cmd" | grep -qE '\b(tee|truncate|dd|install|shred|unlink)\b[^;|&]*('"$PROTECTED"')'; then
  deny "a protected path (write command)"
fi
if printf '%s' "$cmd" | grep -qE '\bsed\b[^;|&]*-i[^;|&]*('"$PROTECTED"')'; then
  deny "a protected path (sed -i)"
fi
if printf '%s' "$cmd" | grep -qE '\b(rm|mv|cp|ln)\b[^;|&]*('"$PROTECTED"')'; then
  deny "a protected path (rm/mv/cp/ln)"
fi
# c) interpreter one-liner opening a protected path for writing
if printf '%s' "$cmd" | grep -qE "(open|writeFile|writeFileSync)[^;|&]*($PROTECTED)"; then
  deny "a protected path (scripted write)"
fi
exit 0

# ── Residual bypass surface — written down so the next reader need not rediscover it ──
# This blocks the obvious and accidental shapes. It does NOT stop intent:
#   • indirection — VAR=decisions.md; echo x > "docs/product/18-philosophy/$VAR"
#   • an interpreter reading the path from a variable, argv, or stdin
#   • a script written elsewhere, then executed
#   • path aliasing — symlinks, ../ traversal, absolute vs relative spellings
#   • any tool shape not matched here (a future write tool, an MCP server)
# The durable control is review plus git history, not this hook. AGENTS.md must therefore
# not claim these paths are "hook-enforced" without qualification — see O6f.
