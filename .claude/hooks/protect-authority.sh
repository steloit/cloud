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
# The companion control is the CI check on the PR diff (ci.yml: authority-paths). It is
# call-shape-independent and --no-verify-proof, so it REPORTS reliably — but it does not
# block a merge: this repo has no branch protection (O6g). See "Residual surface" below.
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

# Strip heredoc BODIES before matching — otherwise you cannot document, review, or file a
# finding about this hook using the shapes it blocks, which is how a hook gets removed.
# Two constraints learned from review:
#   • track the ACTUAL delimiter and end only on it; ending on any bare word resumed scanning
#     mid-body and re-blocked multi-line findings.
#   • never strip the body of an INTERPRETER heredoc (bash/sh/python/node/…) — that body is
#     executed, so stripping it turned `bash <<'EOF' … EOF` into a bypass.
scan="$(printf '%s' "$cmd" | awk '
  {
    if (inbody) { if ($0 ~ "^[[:space:]]*" delim "[[:space:]]*$") { inbody=0 }; next }
    line=$0
    if (match(line, /<<-?[[:space:]]*['"'"'"]?[A-Za-z_][A-Za-z0-9_]*/)) {
      head=substr(line, 1, RSTART-1)
      # interpreter heredoc: the body runs, so keep it in the scan
      if (head ~ /(^|[[:space:];&|])(bash|sh|zsh|ksh|python3?|node|ruby|perl|deno|bun|env)([[:space:]]|$)/) { print; next }
      d=substr(line, RSTART, RLENGTH); gsub(/^<<-?[[:space:]]*['"'"'"]?/, "", d)
      delim=d; inbody=1
    }
    print line
  }
')"
# PATHLESS restores run BEFORE the path gate: they rewrite protected files without ever naming
# them, so a command-string gate would let every one of them through.
printf '%s' "$scan" | grep -qE "$CMDPOS"'git[[:space:]]+(checkout[[:space:]]+\.|restore[[:space:]]+\.|reset[[:space:]]+--hard|stash[[:space:]]+pop|clean[[:space:]]+-[a-z]*f)' \
  && deny "the whole tree (pathless git restore — it would rewrite protected files too)"

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

# d) tee / cp / mv / ln / patch / sponge — protected path in DESTINATION position.
#    `cp decisions.md /tmp/backup.md` is a READ and must stay allowed. Trailing flags and
#    redirects are permitted after the destination: `cp x dec.md 2>/dev/null` is ordinary,
#    and anchoring hard to end-of-line made every one of them a bypass.
printf '%s' "$scan" | grep -qE "$CMDPOS"'(tee|cp|mv|ln|install|patch|sponge|touch)\b[^;|&]*('"$PROTECTED"')[^[:space:];|&]*([[:space:]]+([0-9]*[<>]|-)[^;|&]*)*[[:space:]]*($|[;&|])' \
  && deny "a protected path (copy/move destination)"

# e) git-mediated writes — the highest-value way to revert a founder decision.
#    Split in two: path-naming forms, and PATHLESS forms that restore wholesale and so can
#    never mention the path at all (the command-string gate would miss them entirely).
printf '%s' "$scan" | grep -qE "$CMDPOS"'git[[:space:]]+(checkout|restore|apply|rm|clean)\b[^;|&]*('"$PROTECTED"')' \
  && deny "a protected path (git-mediated write)"
printf '%s' "$scan" | grep -qE "$CMDPOS"'git[[:space:]]+(checkout[[:space:]]+\.|restore[[:space:]]+\.|reset[[:space:]]+--hard|stash[[:space:]]+pop|clean[[:space:]]+-[a-z]*f)' \
  && deny "the whole tree (pathless git restore — it would rewrite protected files too)"

# f) interpreter write — require a call shape and a write mode, not the bare substring
#    ("open" unanchored blocked `rg openapi docs/product/00-sources/`, an everyday command)
# Write-only verbs: the path appearing as an argument is enough.
printf '%s' "$scan" | grep -qE '(write_text|write_bytes|writeFile|writeFileSync|appendFileSync|createWriteStream|unlinkSync|rmSync)[[:space:]]*\([^)]*('"$PROTECTED"')' \
  && deny "a protected path (scripted write)"
# open() needs a WRITE MODE — bare open('path') is a read, and blocking it made
# "print(open(decisions).read())" a false positive.
printf '%s' "$scan" | grep -qE 'open[[:space:]]*\([^)]*('"$PROTECTED"')[^)]*,[[:space:]]*['"'"'"][wax]' \
  && deny "a protected path (scripted write)"
# The path is often an argument to a *constructor* (Path('…').write_text('x')), so the write
# verb and the path sit in different call parens. Pair them WITHIN ONE command segment —
# matching across segments blocked "read an authority file && write somewhere else".
segments="$(printf '%s' "$scan" | awk '{gsub(/&&/,"\n"); gsub(/\|\|/,"\n"); gsub(/;/,"\n"); print}')"
scripted=0
while IFS= read -r seg; do
  printf '%s' "$seg" | grep -qE '(python3?|node|ruby|perl|deno|bun)\b.*('"$PROTECTED"')' || continue
  printf '%s' "$seg" | grep -qE "(write_text|write_bytes|writeFile|writeFileSync|appendFileSync|createWriteStream|,[[:space:]]*['\"][wax])" || continue
  scripted=1; break
done <<< "$segments"
[ "$scripted" = 1 ] && deny "a protected path (scripted write)"

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
# This is an accident floor, not enforcement. The authority-paths job in
# .github/workflows/ci.yml flags any PR whose diff touches these paths without the founder
# marker — reliably, but it cannot BLOCK the merge (no branch protection on this repo: O6g).
# Nothing here binds. Review and the diff still do. Tests: protect-authority.test.sh.
