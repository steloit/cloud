---
id: O6f
title: "\"hook-enforced\" is false: a Bash redirect writes the human-only authority files"
epic: EOPS
status: ready
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
  - .claude/settings.json
  - AGENTS.md
  - docs/adr/0008-mandatory-review-pipeline.md
  - .githooks/**
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

- [ ] A `Bash` redirect targeting a protected path is **blocked**. This is the
  criterion; the task exists because prose alone is not an acceptable resting
  state for a founder-owned authority boundary.
- [ ] *Escape hatch, not an equal option:* if every mechanism in Options is
  rejected, AGENTS.md's "hook-enforced" claim must be corrected to what the
  mechanism really provides **and** the rejection reasoned in the Outcome with
  evidence per option. Closing this way without that record is not permitted.
- [ ] `Edit`/`Write`/`MultiEdit`/`NotebookEdit` to protected paths still blocked —
  no regression in the path that already works.
- [ ] Ordinary `Bash` (`git diff`, `go test`, `grep`, writes anywhere else) is
  unaffected. A hook that fires on unrelated commands gets disabled.
- [ ] The residual bypass surface is **written down** in the hook file itself, not
  left for the next reader to rediscover.

## Related

O6d (same root cause, prose fix) · ADR-0002 · AGENTS.md hard rules ·
`.claude/hooks/protect-authority.sh`
