---
id: US-3.3e
title: "The per-environment resource envelope is a product decision with no owner"
epic: E3
status: blocked
phase: MVP
priority: medium
sprint: 4
issue: 0
labels: [Platform]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/cell-agent/internal/driver/tenancy/**
  - docs/founder-config.md
  - tasks/e3-provisioning/US-3.3e.md
verify:
  - "the rendered ResourceQuota reads its numbers from the founder-owned source, per plan"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -race ./..."
owner: founder
---

## Goal

Give an environment a resource ceiling that someone owns.

## Blocked on — NEEDS FOUNDER INPUT

**What is the per-plan, per-environment resource envelope?** CPU, memory, PVC
count and Service count, for each plan in `plans.json`.

`docs/founder-config.md` §5 owns quota knobs and has no per-environment row. This
task is `blocked` rather than `ready` because AC 1 cannot be executed without that
number, and a `critical`/`ready` task nobody can take is the exact failure O27 was
filed for.

## Why — US-3.3a's security review

US-3.3a shipped `requests.cpu: 8 / limits.cpu: 16 / 16Gi / 32Gi /
persistentvolumeclaims: 16 / count/services: 32` as literals in a driver. Three
problems:

- **Unsourced.** No owner, no derivation, no `NEEDS FOUNDER INPUT` row. A
  mutation inflating every number 1000× (8000 CPU / 16000Gi) survived the whole
  suite green — correct only once an owner exists to pin them against.
- **Plan-independent.** A Free org and a Business org get the identical envelope,
  which contradicts the tiering in `plans.json`. Retyped constants in a driver are
  what the canon rule exists to prevent.
- **Quietly too small in one dimension.** A 3-instance CNPG cluster consumes 3
  PVCs and 3 Services, so `persistentvolumeclaims: 16` caps an environment at ~5
  HA Postgres clusters *before branches* — and there is no error surface when it
  hits. The customer sees provisioning stop.

## Acceptance criteria

1. The envelope is read from the founder-owned source, per plan — not retyped.
2. Hitting the quota produces a problem+json error with `remediation`, not a
   stalled converge.
3. A test asserts the rendered quota matches the configured envelope for at least
   two different plans, so a single-plan test cannot pass on a constant.

## Read first

- `docs/founder-config.md` §5
- commit `7e94f26` — the withdrawn ResourceQuota
