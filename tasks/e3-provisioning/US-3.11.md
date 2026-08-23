---
id: US-3.11
title: "A permanently unrenderable service retries forever with nothing written back"
epic: E3
status: ready
phase: MVP
priority: medium
sprint: 4
issue: 0
labels: [Platform, Backend]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/cell-agent/internal/agent/**
  - services/cell-agent/internal/render/**
  - services/api/internal/reconcile/**
  - tasks/e3-provisioning/US-3.11.md
verify:
  - "a service the driver can never render reaches a terminal status the customer can see"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -race ./..."
owner: agent
---

## The defect

`agent.go:149` logs a render error and continues. For a **transient** error that
is correct — the loop is level-triggered and the next poll retries. For a
**permanent** one it means the service sits in `provisioning` forever with
nothing written back: no status change, no event, nothing the customer or an
operator can see.

The renderer already argues this exact point against itself
(`cnpg_renderer.go`, on why an unknown CNPG phase fails closed to `degraded`):
*"retried forever, which is invisible, the opposite of failing closed."* Failing
closed is right; failing closed **invisibly** is not.

## Why now

Found by T3.4c's architecture review. T3.4c added a permanent render error — an
uncatalogued postgres size is refused rather than silently sized — which makes
this reachable in a new way under ordinary deploy skew (the API accepts a size
the moment it is in `pricing.json`; a cell on the previous agent binary does not
know it). T3.4c removed the worst consequence by taking teardown off that path,
so such a service is still **deletable**; it is not yet **visible**.

## Acceptance criteria

1. The agent distinguishes an unrecoverable render error from a transient one.
2. An unrecoverable one writes back a terminal status the reconciler accepts
   (`provisioning → failed` is already in the ADR-024 machine) with a
   problem+json `remediation`, not just a log line.
3. A transient error keeps its current retry behaviour — no regression in the
   level-triggered loop.
4. Mutation-verified on a GREEN-baseline harness: collapsing the two classes back
   into one is RED in both directions.

## Read first

- `services/cell-agent/internal/agent/agent.go` (the converge loop)
- `services/cell-agent/internal/render/cnpg_renderer.go` (`statusFromPhase`, and
  the comment that makes this argument)
- ADR-024 (the status vocabulary)
