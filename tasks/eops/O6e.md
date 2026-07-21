---
id: O6e
title: validate.mjs has no tests; its checks can be neutered silently
epic: EOPS
status: ready
phase: V1
priority: medium
sprint: 1
estimate: 0.5ew
deps: [O6c]
issue: 0
labels: [DevOps, Tooling, QA]
module: Engineering OS
contexts: []
files:
  - scripts/spec-sync/validate.test.mjs
  - .github/workflows/ci.yml
verify:
  - "node --test scripts/spec-sync/validate.test.mjs passes"
  - "deleting any one check from validate.mjs makes the suite fail"
  - "the suite runs in CI and leaves the working tree byte-identical"
owner: agent
---

## Goal

`validate.mjs` enforces the repo's structural invariants and has **no tests**. Give it
a fault-injection suite so its own checks cannot regress silently.

## Why

O6c added four checks (+147 lines, taking validate.mjs from 72 to 213 — roughly tripling its rule surface) and verified each by
hand-injecting its fault once. That evidence is real but not repeatable: the next edit to
`validate.mjs` can neuter all four and CI stays green.

This is the identical argument O6c's own Why section makes about the reviewers — *a
guarantee nothing checks degrades on the first careless edit* — applied one level up. The
validator became load-bearing without becoming tested, and O6c's checks are now the only
thing standing between a careless frontmatter edit and a writable reviewer.

Two O6c review findings make the case concrete. `agents-readonly` originally **passed when
`tools:` was absent** — the widest possible grant, since an omitted list inherits every
tool including `Write`/`Edit` — while catching only the narrower "tool added" fault. And a
case-variant `QA.md` escaped the check entirely while still serving `subagent_type: qa`.
Both were found by review, not by any test, and both are exactly what a suite pins.

## Acceptance criteria

- [ ] `scripts/spec-sync/validate.test.mjs` using `node:test` (already available; no new
  dependency, consistent with the "No dependencies" header).
- [ ] Each check has at least one **positive** case (fault injected → non-zero exit) and the
  suite has one **negative** case (clean repo → exit 0). Assertions must match the check
  prefix **and the offending file or agent name** — `agents-table-sync` alone carries eight
  distinct faults and `agents-readonly` four, so a prefix-only assertion lets a deleted
  branch be masked by a sibling branch's error.
  The 21 fault classes O6c verified by hand: symlink replaced / `./AGENTS.md` accepted /
  `CLAUDE.md` missing; README moved; `tools:` deleted, emptied, widened; case-variant
  reviewer name; either reviewer absent; unlisted agent; malformed fence; stem mismatch;
  table File-column drift; table casing drift; `AGENTS.md` missing; nested subdirectory;
  **symlinked subdirectory**; **symlinked agent file**; case-fold duplicate names;
  uppercase `.MD` extension; prose line containing pipes not parsed as a table row.
- [ ] **Case-sensitivity is detected at setup, never assumed.** Three of those classes
  (case-fold duplicate, `.MD` extension, case-variant name) cannot execute on a
  case-insensitive filesystem — the injected file silently *overwrites* its lowercase twin
  and the test "passes" having proved nothing. This is not hypothetical: it happened during
  O6c, where writing `Docs.md` destroyed `docs.md` on APFS and the run had to be redone on
  a case-sensitive volume (`hdiutil create -fs "Case-sensitive APFS"`, which works and is
  cheap). The suite must probe the temp dir, then either build on a case-sensitive volume
  or `t.skip()` with the reason stated in the output — never silently pass.
- [ ] Injection happens in a **temp copy of the repo**, never the working tree — the suite
  must leave `git status` clean even when it fails midway. O6c's manual runs mutated the
  real tree and depended on the operator restoring it correctly.
- [ ] Wired into `.github/workflows/ci.yml` in the existing `validate` job.
- [ ] Deleting any single check from `validate.mjs` makes the suite fail — verify this by
  actually doing it once per check, and paste the output.

## Note

Error ORDER is already deterministic (`agentFiles` is sorted, task walk order is stable),
but nothing asserts it. If the suite compares stderr verbatim rather than matching on the
check prefix, it will flake the moment a filesystem returns a different `readdir` order.
Match on prefixes, not on whole-output equality.

## Related

O6c (the checks under test) · ADR-0008 · scripts/spec-sync/validate.mjs
