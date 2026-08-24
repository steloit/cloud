---
id: US-3.14
title: "Nothing binds the statuses the cell-agent can emit to the ones the contract accepts"
epic: E3
status: ready
phase: MVP
priority: medium
sprint: 4
estimate: 0.25ew
deps: []
issue: 0
labels: [Backend, Platform]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/cell-agent/internal/render/**
  - services/cell-agent/internal/agent/**
  - packages/contracts/**
  - tasks/e3-provisioning/US-3.14.md
verify:
  - "every status the renderer can emit is one the reconcile contract advertises, asserted against the contract rather than a retyped list"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -count=1 -race ./..."
owner: agent
---

## The gap

US-3.12 narrowed the `POST /reconcile/{cell}/status` request enum to exactly what
a cell may report. **Nothing checks that the agent stays inside it.**

The cell-agent posts a hand-written `Report` struct
(`internal/agent/agent.go`), does **not** import `packages/contracts/go`, and its
`terminal()` allows `{ready, failed, gone, degraded}`
(`internal/render/cnpg_renderer.go`). Narrowing the enum is precisely the change
that could strand a status the agent still emits, and nothing goes red if it
does — the failure would surface as a 422 in production, per service, per tick.

Today the sets happen to agree: `statusFromPhase` emits only
`ready`/`provisioning`/`failed`, plus `gone` from the teardown path, and all four
are advertised. That is agreement by coincidence, not by construction.

## Why it is its own task

The two live in **separate Go modules** with no import between them, so this is
not a one-line assertion — it needs a shared artifact or a generated constant,
and choosing one is the work.

Note the premise that must be checked first: `apps/cli/go.mod` already imports
`packages/contracts/go` across a module boundary with a `replace`, so "separate
modules, therefore they cannot share" is **false** and was already recorded as a
false premise once (US-3.3a round 12, via the US-3.3h task file). The cell-agent
simply does not import it yet.

## Acceptance criteria

1. The set of statuses the renderer can emit is bound to the contract's request
   enum — reading the contract (or a generated constant derived from it), never a
   retyped list.
2. Adding a status to `terminal()`/`phaseStatus` that the contract does not
   advertise is RED.
3. Removing a status from the contract that the agent still emits is RED.
4. The binding does not require the agent to import the control plane's internal
   packages (ADR-0001 D9/A2.5 — the data plane holds no copy of control-plane
   policy; a generated contract artifact is not a copy of the machine).

## Read first

- `services/cell-agent/internal/render/cnpg_renderer.go` — `terminal`, `phaseStatus`, `statusFromPhase`
- `services/api/internal/reconcile/contract_test.go` — the control-plane-side binding US-3.12 added
- `packages/contracts/go/oapi.cfg.yaml` — the generated Go client the agent does not yet use

## Found by

US-3.12's QA review (2026-08-24), as the regression its own premise implies:
*"an agent generated from the contract would have believed both were valid"* —
but this agent is not generated from the contract at all.
