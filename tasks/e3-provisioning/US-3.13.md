---
id: US-3.13
title: "A row whose observed_generation never advances has no owner — a steadily degraded service 409s forever"
epic: E3
status: ready
phase: MVP
priority: medium
sprint: 4
estimate: 0.5ew
deps: []
issue: 0
labels: [Backend, Platform]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/api/internal/provisioning/**
  - services/api/internal/reconcile/**
  - tasks/e3-provisioning/US-3.13.md
verify:
  - "a service that reports the same unsettled status every tick reaches a state an operator can see and act on, rather than 409ing forever"
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go test -count=1 -race ./internal/reconcile/ ./internal/provisioning/"
owner: agent
---

## The gap

US-3.3h made convergence depend on the destination: a generation finishes only
when the row rests on a **settled** status (not `provisioning`, not `degraded`)
that the cell actually reported. That is right — `degraded` bills, and a row
parked there unwatched bills indefinitely.

It leaves a case with no owner. A cluster that reports `degraded` on **every**
tick never converges: `ready`+`degraded` takes the edge, then `degraded`+`degraded`
takes none and never settles. `observed_generation` never advances, the row stays
in `ListDesiredForCell`, and the agent re-applies and 409s forever.

Measured during US-3.3h's review: 25 consecutive ticks, no convergence.

This is **strictly better** than the defect US-3.3h fixed — the customer sees
`degraded`, where before they saw `ready` for a broken database — but "visible to
the customer" is not the same as "someone is going to do something about it", and
nothing escalates, alerts, or terminalises.

**US-3.11 does NOT cover this.** Its ACs are about the *agent* distinguishing an
unrecoverable render error from a transient one. This is a control-plane row that
never advances, with a perfectly healthy agent. US-3.3h originally cited US-3.11
here and the citation was wrong; that is why this task exists.

## Not reachable today, and exactly one change away

`phaseStatus` maps no CNPG phase to `degraded` (unknown phases map to `failed`),
so no agent currently reports it. But `render.terminal()` already accepts
`degraded`, so the first phase mapping that yields it makes the loop live.

`last_reconciled_at` is written by `MarkObserved` only — i.e. never on an
unconverged hop — and has **no reader anywhere in the repo**, so it cannot serve
as the staleness signal without that being built.

## Acceptance criteria

1. A row that has not advanced its `observed_generation` for a bounded number of
   attempts (or a bounded time) is distinguishable from one converging normally —
   in a query an operator or an alert can run.
2. The mechanism does not depend on `last_reconciled_at` unless this task also
   gives it a writer on the unconverged path and a reader.
3. Whatever escalation is chosen does not re-introduce the harm US-3.3h removed:
   it must not advance observation onto an unsettled status, and it must not stop
   the metering span while the resource still runs.
4. Mutation-verified on a GREEN-baseline harness.

## Read first

- `services/api/internal/provisioning/services.go` — `ObservedStatus`, `settledStatuses`
- `services/api/internal/reconcile/reconcile.go` — `Writeback`, `ErrNotConverged`
- `tasks/e3-provisioning/US-3.11.md` — the adjacent, different problem
