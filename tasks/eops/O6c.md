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

- [ ] Rule `agents-readonly`: parse each `.claude/agents/*.md` frontmatter;
  assert `reviewer` and `qa` declare `tools ⊆ {Read, Grep, Glob, Bash}`. Failure
  message cites ADR-0008's reviewer-identity clause.
- [ ] Rule `agents-table-sync`: the set of frontmatter `name`s equals the
  README table's `subagent_type` column, and each `name` matches its filename
  stem. README.md itself is excluded (it has no agent frontmatter).
- [ ] Both rules run in the existing `validate.mjs` invocation — no new command.
- [ ] Malformed or unparseable frontmatter fails loudly rather than being skipped.

## Note on what this cannot check

Frontmatter is the *requested* grant. O6 observed the effective runtime grant to
be narrower (`reviewer` resolved to `Read, Bash` from a four-tool list), so these
rules pin intent, not the harness's behavior. That is still worth pinning: the
harness may narrow a grant, but it will not narrow one that was never requested.

Detecting a **stale** served prompt (file edited, old version still in effect)
remains unsolved; a `registration-token` fingerprint each agent echoes was
proposed and deferred as ceremony not yet justified by an observed failure.

## Related

ADR-0008 · O6 · scripts/spec-sync/validate.mjs
