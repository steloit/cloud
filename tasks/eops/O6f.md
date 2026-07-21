---
id: O6f
title: "\"hook-enforced\" is false: a Bash redirect writes the human-only authority files"
epic: EOPS
status: done
phase: V1
priority: high
sprint: 1
estimate: 0.5ew
deps: [O6d]
issue: 0
labels: [DevOps, Security, Tooling]
module: Engineering OS
contexts: []
files:
  - .claude/hooks/protect-authority.sh
  - .claude/hooks/protect-authority.test.sh
  - .claude/settings.json
  - AGENTS.md
  - docs/adr/0008-mandatory-review-pipeline.md
  - .github/workflows/ci.yml
verify:
  - "a Bash redirect writing docs/product/18-philosophy/decisions.md is blocked (or, only if every mechanism is rejected with recorded evidence, AGENTS.md stops claiming hook enforcement)"
  - "an Edit/Write to the same path is still blocked (no regression)"
  - "ordinary Bash commands that touch no protected path are unaffected"
owner: agent
---

## Goal

AGENTS.md states that `docs/product/00-sources/**` and
`docs/product/18-philosophy/decisions.md` change by **human decision only
(hook-enforced)**. The hook does not enforce that. Close the gap, or stop
claiming it.

## Why

Found while closing O6d, which corrected the identical false claim about
reviewers being read-only. This instance is worse: it guards a **human-only
authority boundary**, not a process convention.

`.claude/settings.json` registers `protect-authority.sh` with
`"matcher": "Edit|Write|MultiEdit|NotebookEdit"` — `Bash` is not matched. The
hook itself then reads `.tool_input.file_path` / `.notebook_path` and exits 0
when neither is present, which is always the case for a `Bash` call. So:

```
echo "ratified" >> docs/product/18-philosophy/decisions.md   # not seen by the hook
```

Any agent holding `Bash` — every implementation agent, and both reviewers — can
write the founder-owned files. Verified in O6 by the same mechanism:
`echo x > file` executed under `reviewer` with no prompt.

This is the third instance of one root cause: **a `file_path`-only hook cannot
see shell redirects**, so every "hook-enforced" claim in this repo is
approximately decorative. O6d corrected the reviewer claim in prose; this one
needs the mechanism fixed, because the alternative — documenting that the
founder's authority files are unprotected — is not an acceptable resting state.

## Options (decide with evidence, do not assume)

1. **Extend the matcher to `Bash` and inspect `.tool_input.command`.** Deny when
   a protected path appears as a redirect target, `tee` argument, `sed -i`
   target, or similar. Cheapest, and it also closes the reviewer-write gap O6d
   left open — the two are the same defect. Risk: command parsing is
   adversarially hard; a determined agent can obfuscate. **It raises the floor
   against accidents, not against intent** — say so rather than overclaiming
   again, which is the exact failure this task family keeps finding.
2. **Deny `Bash` to the reviewers only.** Narrower, does nothing for
   implementation agents, and costs the reviewers `git diff`/`go test`/`grep`.
   Note `docs/plan/kernel-workflow-review.md:10-12` records a shell-free reviewer
   catching real blockers, so this cost is smaller than assumed.
3. **A `git` pre-commit hook** asserting the protected paths are unchanged unless
   a human-signoff marker is present. Catches the outcome rather than the call,
   survives obfuscation, but is bypassable with `--no-verify` and does not
   protect an uncommitted read of a mutated file.

Options 1 and 3 compose; that is likely the answer, but **measure before
claiming.** Whatever ships, AGENTS.md's wording must end up matching what is
actually true — including, if that is the outcome, saying the protection is
best-effort against accidents.

## Acceptance criteria

- [x] A `Bash` redirect targeting a protected path is **blocked**. This is the
  criterion; the task exists because prose alone is not an acceptable resting
  state for a founder-owned authority boundary.
- [x] *Escape hatch — not taken; the mechanism shipped.* Original text: if every mechanism in Options is
  rejected, AGENTS.md's "hook-enforced" claim must be corrected to what the
  mechanism really provides **and** the rejection reasoned in the Outcome with
  evidence per option. Closing this way without that record is not permitted.
- [x] `Edit`/`Write`/`MultiEdit`/`NotebookEdit` to protected paths still blocked —
  no regression in the path that already works.
- [x] Ordinary `Bash` (`git diff`, `go test`, `grep`, writes anywhere else) is
  unaffected. A hook that fires on unrelated commands gets disabled.
- [x] The residual bypass surface is **written down** in the hook file itself, not
  left for the next reader to rediscover.

## Related

O6d (same root cause, prose fix) · ADR-0002 · AGENTS.md hard rules ·
`.claude/hooks/protect-authority.sh`

## Outcome

Two controls, because one was not enough and the review proved it.

**CI `authority-paths` reports on every PR.** It reads the PR diff, so no call-shape
trick reaches it and `--no-verify` does not apply. It was not in the original plan:
the first draft dismissed Option 3 by considering only the *local* pre-commit
variant (advisory) and never the CI variant. The reviewer caught that.

**But it does not BIND, and I claimed it did.** Verified after the review pushed
on it: `gh api .../branches/main/protection` and `/rulesets` both return 403
"Upgrade to GitHub Pro or make this repository public" — private repo, free org
plan. CI reports; a red PR merges exactly like a green one. CODEOWNERS is
decorative for the same reason, and the `FOUNDER-RATIFIED` marker is written by
the same agent that opens the PR, so it constrains nobody.

I had written "**CI is what binds**" into AGENTS.md. That would have been the
**fourth** restatement of this family's one recurring defect — a document
asserting a guarantee no mechanism provides — and the worst, because it replaced
a claim I had just correctly weakened with a stronger false one. AGENTS.md now
says nothing blocks a merge. **O6g** carries the founder decision that would
change it.

**The PreToolUse hook is an accident floor.** `settings.json`'s matcher gains
`Bash`; the hook inspects `.tool_input.command` when no `file_path` is present.
The first implementation was badly wrong in both directions and adversarial
review found it:

- *False positives that would have got the hook disabled.* `rg openapi
  docs/product/00-sources/` was blocked — `open` was an unanchored substring and
  `openapi` is this repo's central noun. `grep -n 'rm' decisions.md` was blocked
  because `rm` appeared inside a search term. **`cp decisions.md /tmp/backup.md`
  was blocked though it is a read** — and worse, so was the reverse `cp`, which
  is the *founder's documented ratification fallback* (`consistency-audit`
  records the founder's `!` stamp failing twice and an explicit `cp` applying the
  stamped copy). I would have broken the ratification path this task exists to
  protect. Fixed: verbs match only in command position, `cp`/`mv`/`ln` only when
  the protected path is the destination, `open` requires a call shape, heredoc
  bodies are stripped (you could not previously *document* this hook without
  tripping it), and `STELOIT_RATIFY=1` is an explicit, auditable founder bypass.
- *Bypasses of the most obvious shapes.* `rm -rf docs/product/00-sources` —
  no trailing slash, the most natural spelling of the most destructive
  operation — was allowed. So were `git checkout HEAD~5 -- decisions.md` (the
  highest-value way to revert a founder decision), `>|`, `perl -pi`,
  `Path(...).write_text(...)`, and `cd 18-philosophy && echo >> decisions.md`.
- *Fail-open.* With `jq` absent the hook exited 127, which is non-blocking — a
  missing dependency silently disabled the control. It now fails closed.

**57 regression tests** in `protect-authority.test.sh`, wired into CI. Every case
came from a review finding rather than imagination: the BLOCK cases are bypasses
the first version allowed, the ALLOW cases are false positives it produced. Both
reviewers independently called the absence of tests the load-bearing gap for a
hand-tuned security control, and they were right — five regexes with no pinning
test is the same "guarantee nothing checks" pattern this whole task family exists
to close.

AGENTS.md now says what is true: a hook stops common accidental writes, **CI is
what binds**. The residual surface the hook cannot reach — variable indirection,
interpreter argv/stdin, script-then-execute, symlink and `../` aliasing, xargs
splitting, future tool shapes, and the deliberate `STELOIT_RATIFY=1` — is
enumerated in the hook header, because the honesty of the claim rests on that
list being complete, and the first version's list was not.
