---
id: O6c
title: Pin the agent directory's load-bearing properties in validate.mjs
epic: EOPS
status: ready
phase: V1
priority: medium
sprint: 1
estimate: 0.25ew
deps: [O6]
issue: 0
labels: [DevOps, Tooling]
module: Engineering OS
contexts: []
files:
  - scripts/spec-sync/validate.mjs
  - .claude/agents/README.md
verify:
  - "node scripts/spec-sync/validate.mjs"
  - "adding Edit to reviewer.md frontmatter makes validate.mjs fail, citing ADR-0008"
  - "adding an agent file without a README table row makes validate.mjs fail"
owner: agent
---

## Goal

`validate.mjs` covers `tasks/`, epics, and packs, but **nothing under
`.claude/**`**. Add rules pinning the two properties ADR-0008 and O6 depend on.

## Why

ADR-0008 makes "the two reviewers are read-only" load-bearing — it is what makes
an independent review independent. Today that property lives only in
`reviewer.md`/`qa.md` frontmatter and in README prose. **Adding `Edit` to that
frontmatter would silently convert a reviewer into a writer**, and no hook, CI
job, or validator would notice. A guarantee nothing checks is a guarantee that
degrades on the first careless edit.

Second, O6's README carries a hand-maintained agent table. The README instructs
authors to add a row, but instruction is not enforcement, and a stale table is
the re-derivation cost O6 exists to remove.

## Acceptance criteria

`validate.mjs` is 72 flat lines with a single `err(file, msg)` helper and no rule
registry — add two checks in that style, with failure messages prefixed
`agents-readonly:` and `agents-table-sync:`. Do not refactor to introduce a rule
abstraction for two checks.

- [ ] `agents-readonly`: parse each `.claude/agents/*.md` frontmatter; assert
  `reviewer` and `qa` declare `tools ⊆ {Read, Grep, Glob, Bash}`. Failure cites
  ADR-0008's reviewer-identity clause.
- [ ] `agents-table-sync`: the set of frontmatter `name`s equals the README
  table's `subagent_type` column, and each `name` matches its filename stem.
  **Exclude `README.md` by filename**, not by "has no frontmatter" — `lib.mjs`
  `parseFrontmatter` throws the same `missing frontmatter` error for absent and
  malformed blocks, so those two cases are indistinguishable at the parser and a
  content-based discriminator would silently skip a typo'd agent file.
- [ ] A file with a malformed frontmatter block fails loudly rather than being
  skipped — test with a `notes.md` carrying a typo'd `---` fence.
- [ ] Both checks run in the existing `validate.mjs` invocation — no new command.

## What this cannot check — read before assuming it closes ADR-0008

**`agents-readonly` does not make a reviewer read-only, and must not be described
as if it does.** O6 verified that `reviewer`'s `Bash` grant executes
`echo x > file` with no prompt, and `protect-authority.sh` inspects only a tool's
`file_path`, so a shell redirect bypasses it. Withholding `Write`/`Edit` while
granting `Bash` leaves writing fully available. This rule pins the *declared*
grant against a careless edit — worth having, but ADR-0008's "the two reviewers
are read-only" stays a behavioral constraint until something constrains `Bash`.
If that gap is worth closing, it needs its own task and probably its own ADR.

Frontmatter is also only the *requested* grant: O6 observed the effective runtime
grant to be narrower (`reviewer` → `Read, Bash` from a four-tool list). The
harness may narrow a grant; it will not grant one that was never requested.

Detecting a **stale** served prompt (file edited, old version still in effect)
remains unsolved. A `registration-token` fingerprint each agent echoes was
proposed and deferred — ceremony not yet justified by an observed failure.

## Related

ADR-0008 · O6 · scripts/spec-sync/validate.mjs
