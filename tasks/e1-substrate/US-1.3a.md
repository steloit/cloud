---
id: US-1.3a
title: "Desired-state writers: edits and deletes bump generation and reach the cell"
epic: E1
status: in-progress
phase: MVP
priority: high
sprint: 2
estimate: 0.5ew
deps: [US-1.3, T3.3]
issue: 0
labels: [Platform, Backend]
module: M1 Substrate
contexts: [provisioning]
files:
  - services/api/internal/provisioning/services.go
  - services/api/internal/provisioning/services_test.go
  - services/api/db/queries/reconcile.sql
apis: []
tables: [services]
events: []
tests: [TestShapeEditBumpsGeneration, TestShapeEditBecomesOutstanding, TestDeleteWritesDeletingDesired, TestDeleteConvergesToGone]
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go test ./internal/provisioning/ ./internal/reconcile/"
owner: agent
---

## Goal

Wire the desired-state WRITERS so shape/scaling edits and deletes actually reach
the cell. US-1.3 landed the reconciler protocol and the read/writeback path;
this closes the producing half beyond creation.

## Why — a real gap US-1.3's review surfaced, recorded not hidden

US-1.3's poll returns OUTSTANDING work (`observed_generation < generation`), so a
freshly **created** service is picked up automatically (observed 0 < generation
1) — creation reaches the cell today. But:

- `updateService` (shape/scaling edit) writes `shape` and never bumps
  `generation`, so an edit leaves `observed == generation` and the cell never
  re-reconciles it.
- `deleteService` transitions status but never sets a `desired` deletion flag,
  so the AckRenderer's `desired["deleting"]` → `gone` branch is unreachable from
  real data.
- `desired` is `'{}'` at creation; the real renderer (T1.4/T3.4) needs the
  desired document populated from product + shape.

`BumpServiceGeneration` exists (US-1.3) with no production caller — that was left
deliberately, because the writers are provisioning-side and outside US-1.3's file
scope. This task is that wiring.

## Scope

- `updateService`: after a shape/scaling edit, call `BumpServiceGeneration` with
  the new desired document (product + shape + scaling + lifecycle flags) in the
  same transaction as the edit.
- `deleteService`: write `desired` with a `deleting: true` flag and bump
  generation, so the cell converges the teardown and reports `gone`. Coordinate
  with US-3.5's final-backup contract — deletion is not just a status flip.
- `createService`: populate `desired` from the created shape (not `'{}'`), so the
  renderer has a real document from birth.

## Acceptance criteria

- [ ] A shape edit bumps `generation`, making the service outstanding again;
  `Desired(cell, 0)` returns it. Proven against real Postgres.
- [ ] A delete writes a `deleting` desired flag and bumps generation; the
  AckRenderer returns `gone` for it (integration-level).
- [ ] `createService` populates `desired` (non-empty) from the shape.
- [ ] No customer-facing leak of the desired document (the D8 guard in
  `TestServiceViewDoesNotLeakReconcilerColumns` stays green).

## Also carry these US-1.3 review findings (recorded, not yet fixed)

These are real but out of US-1.3's file scope or genuinely renderer-side; they
land here so they are not lost:

- **Persistent converge failure is invisible to the control plane.** The agent's
  converge-error path is `continue` — no writeback, so a service whose render
  always errors shows `provisioning` forever with no signal. Needs a
  failure-reporting/backoff path once the real renderer (T1.4) can fail
  meaningfully. Today's AckRenderer never fails, so it is not yet reachable.
- **Page starvation past the poll LIMIT.** `ListDesiredForCell` is
  `ORDER BY generation LIMIT 100`; >100 permanently-stuck low-generation rows
  would shadow all higher-generation work. Low risk at one-cell alpha; needs
  rotation/backoff or offset before a cell carries >100 concurrently-outstanding
  services.
- **Writeback atomicity is by ordering, not a transaction.** US-1.3 fixed the
  stranding bug by advancing `observed_generation` only after a durable
  transition, with a microsecond read-then-check TOCTOU on a concurrent desired
  bump (backstopped by the exact-match SQL guard). Full atomicity wants
  `MarkObserved` + `Transition` (with its events + metering) in one transaction —
  a provisioning-side refactor to a tx-aware transition, best done when edits
  become genuinely concurrent.
- **`gone` on a non-deleting service** is currently inert w.r.t. status
  (observation-only) but still a nonsensical report; when the deletion pipeline
  (US-3.5) lands, decide whether it should be an explicit reject.
- **Mistake-bank note (QA, US-1.3 round 4):** the agent's HTTP client treats a
  409 writeback as a loggable non-error, which is behaviorally correct only
  because the loop is stateless (recovery is server-side via the outstanding
  poll). The moment the agent grows state keyed on report success — conditions,
  backoff, a local watermark — that 409-as-success becomes load-bearing and
  needs its own handling. Do not add such state without revisiting it.

## Related

US-1.3 (the protocol) · T3.3 (service writers) · US-3.5 (deletion + final backup)
· `docs/plan/e1-substrate-design.md` §2
