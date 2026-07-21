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

O6c added four checks (≈75 lines, doubling the file's rule surface) and verified each by
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
- [ ] Each check has at least one **positive** case (fault injected → non-zero exit, error
  names the check prefix) and the suite has one **negative** case (clean repo → exit 0).
  At minimum the 15 fault classes O6c verified by hand: symlink replaced / `./AGENTS.md`
  accepted / `CLAUDE.md` missing; README moved; `tools:` deleted, emptied, widened;
  case-variant reviewer name; either reviewer absent; unlisted agent; malformed fence;
  stem mismatch; table File-column drift; table casing drift; `AGENTS.md` missing.
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
