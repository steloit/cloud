#!/usr/bin/env bash
# PreToolUse hook: block agent writes to human-decision-only paths (AGENTS.md hard rule).
# Exit 2 = block the tool call; stderr is fed back to the agent.
#
# Two call shapes:
#   1. Edit/Write/MultiEdit/NotebookEdit — structured and exact (.tool_input.file_path).
#   2. Bash — a command string, covered since O6f. `echo x >> decisions.md` previously
#      wrote a founder-owned file unseen.
#
# Shape 2 targets ACCIDENTS with high precision. It is deliberately NOT exhaustive:
# a false positive gets this hook disabled, which is strictly worse than a known gap.
# The enforceable control is the CI check on the PR diff (ci.yml: authority-paths),
# which binds at merge and cannot be talked around. See "Residual surface" at the bottom.
#
# Founder escape hatch: prefix the command with STELOIT_RATIFY=1 to apply a ratified
# decision (the documented stamped-copy flow — see docs/plan/consistency-audit-2026-07-18.md,
# where the founder's `!` stamp failed and the fallback was an explicit cp). The CI check
# still reports the change, which is the point: bypass is possible but never silent.
set -euo pipefail

PROTECTED='docs/product/00-sources(/|$|[^A-Za-z0-9_-])|docs/product/18-philosophy/decisions\.md'
# command-position: start of string, or after a separator — so `grep -n 'rm' file` is a READ
CMDPOS='(^|[;&|]|&&|\|\|)[[:space:]]*'

deny() {
  echo "BLOCKED: $1 changes by human decision only (AGENTS.md hard rules). Propose the change in your PR/finding instead." >&2
  echo "If you are applying a founder-ratified decision, prefix the command with STELOIT_RATIFY=1 (CI still reports it)." >&2
  exit 2
}

command -v jq >/dev/null 2>&1 || {
  echo "BLOCKED: protect-authority hook cannot run without jq — failing closed rather than letting a write through unchecked." >&2
  exit 2
}

input="$(cat)"

# Shape 1: structured file-path tools.
path="$(printf '%s' "$input" | jq -r '.tool_input.file_path // .tool_input.notebook_path // empty' 2>/dev/null || true)"
if [ -n "$path" ]; then
  case "$path" in
    *docs/product/00-sources/*|*docs/product/18-philosophy/decisions.md) deny "$path" ;;
  esac
  exit 0
fi

# Shape 2: Bash.
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // empty' 2>/dev/null || true)"
[ -n "$cmd" ] || exit 0
# Explicit, auditable founder bypass.
printf '%s' "$cmd" | grep -qE '(^|[[:space:]])STELOIT_RATIFY=1([[:space:]]|$)' && exit 0

# Strip heredoc BODIES before matching. Without this you cannot document, review, or file a
# finding about this hook using the very shapes it blocks — which is how a hook gets removed.
scan="$(printf '%s' "$cmd" | awk '
  /<<-?[[:space:]]*['"'"'"]?[A-Za-z_][A-Za-z0-9_]*/ { print; inbody=1; next }
  inbody && /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*$/ { inbody=0; next }
  !inbody { print }
')"
# Gate is deliberately WIDER than PROTECTED: it must also admit a bare `cd` into a protected
# directory, where the filename arrives separately (rule g).
printf '%s' "$scan" | grep -qE "$PROTECTED"'|docs/product/18-philosophy' || exit 0

# a) redirect whose TARGET is protected (covers >, >>, >|, &>, 1>)
printf '%s' "$scan" | grep -qE '[0-9]?>>?\|?[[:space:]]*['"'"'"]?[^[:space:];|&]*('"$PROTECTED"')' \
  && deny "a protected path (shell redirect)"

# b) in-place editors naming a protected path (command position)
# -[a-z]*i, not -i: `perl -pi -e` combines the flags and a bare -i never matches it.
printf '%s' "$scan" | grep -qE "$CMDPOS"'(sed|perl|ruby|gawk|awk|ex|ed|vim|sort|sponge)\b[^;|&]*(-[a-zA-Z]*i([[:space:]]|$)|inplace|-o[[:space:]]|wq)[^;|&]*('"$PROTECTED"')' \
  && deny "a protected path (in-place edit)"

# c) destructive verbs, command position, path anywhere (rm/truncate/shred delete wherever named)
printf '%s' "$scan" | grep -qE "$CMDPOS"'(rm|truncate|shred|unlink|dd)\b[^;|&]*('"$PROTECTED"')' \
  && deny "a protected path (delete/truncate)"

# d) tee / cp / mv / ln — only when a protected path is the FINAL (destination) argument.
#    `cp decisions.md /tmp/backup.md` is a READ and must stay allowed.
printf '%s' "$scan" | grep -qE "$CMDPOS"'(tee|cp|mv|ln|install)\b[^;|&]*('"$PROTECTED"')[^[:space:];|&]*[[:space:]]*($|[;&|])' \
  && deny "a protected path (copy/move destination)"

# e) git-mediated writes — the highest-value way to revert a founder decision
printf '%s' "$scan" | grep -qE "$CMDPOS"'git[[:space:]]+(checkout|restore|apply|stash[[:space:]]+pop|revert|rm)\b[^;|&]*('"$PROTECTED"')' \
  && deny "a protected path (git-mediated write)"

# f) interpreter write — require a call shape and a write mode, not the bare substring
#    ("open" unanchored blocked `rg openapi docs/product/00-sources/`, an everyday command)
printf '%s' "$scan" | grep -qE '(open|write_text|write_bytes|writeFile|writeFileSync|appendFileSync|createWriteStream)[[:space:]]*\([^)]*('"$PROTECTED"')' \
  && deny "a protected path (scripted write)"
# The path is often an argument to a *constructor* (Path('…').write_text('x')), so the write
# verb and the path sit in different call parens. Pair an interpreter + path + write verb.
printf '%s' "$scan" | grep -qE "$CMDPOS"'(python3?|node|ruby|perl|deno|bun)\b[^;|&]*('"$PROTECTED"')' \
  && printf '%s' "$scan" | grep -qE "(write_text|write_bytes|writeFile|writeFileSync|appendFileSync|createWriteStream|['\"][waxr]\+?['\"]|,[[:space:]]*['\"]w)" \
  && deny "a protected path (scripted write)"

# g) cd into a protected directory, then a bare-basename write
printf '%s' "$scan" | grep -qE 'cd[[:space:]]+[^;|&]*docs/product/(18-philosophy|00-sources)' \
  && printf '%s' "$scan" | grep -qE '[0-9]?>>?\|?[[:space:]]*['"'"'"]?(decisions\.md|[A-Z]+-[0-9]+\.md)' \
  && deny "a protected path (cd + relative redirect)"

exit 0

# ── Residual surface — enumerated so no reader mistakes this for a sandbox ─────────────
# Reachable write shapes this hook does NOT stop:
#   • variable indirection — F=decisions.md; echo x > "docs/product/18-philosophy/$F"
#   • an interpreter taking the path from a variable, argv, stdin, or a config file
#   • a script written elsewhere, then executed
#   • path aliasing — symlinks, ../ traversal, globs, absolute-vs-relative spellings
#   • xargs and other command-splitting (echo PATH | xargs rm)
#   • any tool shape not matched here (a future write tool, an MCP server)
#   • STELOIT_RATIFY=1, deliberately
# This is an accident floor, not enforcement. The control that actually binds is the
# authority-paths job in .github/workflows/ci.yml, which fails any PR whose diff touches
# these paths without the founder-approval marker. Regression tests: protect-authority.test.sh.
