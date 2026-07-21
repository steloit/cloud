---
id: US-1.3
title: "Reconciler protocol: desired state written, cell agent converges actual"
epic: E1
status: in-progress
phase: MVP
priority: critical
sprint: 2
estimate: 1ew
deps: [T1.1]
issue: 41
labels: [Platform, Backend]
module: M1 Substrate
contexts: [provisioning, api-conventions]
files:
  - docs/product/08-api/openapi.yaml
  - services/api/internal/platform/db/migrations/*_reconciler.*.sql
  - services/api/db/queries/reconcile.sql
  - services/api/internal/reconcile/**
  - services/api/internal/identity/store/reconcile.sql.go
  - services/api/cmd/api/main.go
  - services/cell-agent/**
  - packages/contracts/**
  - tasks/e1-substrate/US-1.3.md
apis: [getDesiredState, postReconcileStatus]
tables: [cells, services]
events: [service.status_changed]
tests: [TestDesiredRequiresReconcilerToken, TestDesiredScopedToCell, TestSinceGenerationFiltersRows, TestStatusWritebackAdvancesObservedGeneration, TestStaleGenerationWritebackIsRejected, TestStatusWritebackIsIdempotent, TestConcurrentWritebackAppliesOnce, TestHeartbeatUpdatesAgentLastSeen, TestForeignCellIs404, TestDeletingConvergesToGone, TestStatusEmitsEventOnEveryEdge, TestMeteringStartsAtReady]
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go build ./... && go vet ./... && go test ./..."
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go build ./... && go vet ./... && go test ./..."
  - "make gen-go && git diff --exit-code -- services packages   # contract drift"
  - "control-plane-outage drill: agent loop returns error, converge is skipped, no panic, no state mutation"
owner: agent
---

## Goal

Reconciler protocol: the control plane writes **desired** state; the cell-agent
converges **actual** and reports back. Control-plane outage degrades to "cannot
make changes", never "apps down" (D9/A2.5).

## Authority

`docs/plan/e1-substrate-design.md` §2 is the contract — written under SP0-1
(done, founder-reviewed) and cited by name for this task. Do not re-derive the
protocol; implement what §2 specifies. `contexts/provisioning.md` governs the
invariants.

## Finding to record, not silently fix

§2 says "the reconciler endpoints enter openapi.yaml via S-process **before**
T1.2 implements". **T1.2 is `done` and the endpoints are absent from
openapi.yaml** (`grep -n reconcile docs/product/08-api/openapi.yaml` → only
unrelated prose matches). This task adds them, citing §2 as the ratified
shape rather than inventing one — but the ordering was violated upstream and
that belongs in the PR as a finding.

## Scope

**1 · Schema** (`*_reconciler.up.sql`, additive only)
- `cells (id text pk, region text not null, status text not null, agent_last_seen_at timestamptz)`;
  seed `cell-0` to match the existing `services.cell_id` default.
- `services` gains `desired jsonb not null default '{}'`, `generation bigint not null default 1`,
  `observed_generation bigint not null default 0`, `last_reconciled_at timestamptz`.
- `services.cell_id` gains an FK to `cells(id)` and an index on `(cell_id, generation)` — the
  desired-poll query shape.
- Down migration drops exactly what the up added.

**2 · Contract** (openapi.yaml, internal-plane tag — "no internal-only protocols")
- `GET /v1/reconcile/{cell}/desired?since_generation=` → `{services: [{id, generation, desired, cell_id}], ...}`
- `POST /v1/reconcile/{cell}/status` ← `{service_id, observed_generation, status, conditions[], event}`
- problem+json with `remediation` on every error (ADR/api-conventions). Regenerate:
  `make gen-go`; the op ids go in `include-operation-ids` only once handlers exist and are tested.

**3 · API handlers** (`internal/reconcile/`)
- Reconciler-scoped token, distinct from user sessions and org API keys. A user token must NOT
  reach these endpoints, and a cell token must reach only its own cell (foreign cell → **404**,
  not 403 — org-fencing convention).
- Status writeback is the only writer of `services.status` on this path: it advances
  `observed_generation`, sets `last_reconciled_at`, emits `service.status_changed` on every edge
  (D10), and **starts metering at `ready`** (ADR-024 vocabulary).
- **Stale-generation writeback is rejected**, not applied — an agent reporting on an older
  generation must not clobber a newer desired state.

**4 · cell-agent** (`services/cell-agent/`, new Go module)
- Poll → converge → writeback loop, level-triggered. Renders from `desired` every pass; **never
  diffs by memory** (§2).
- At alpha: poll only. No long-poll/SSE, no controller-runtime dependency yet — the loop is the
  contract, the renderer is a seam.
- Converge is **idempotent**; deletes converge to absence then report `deleting → gone`.
- Heartbeat rides the status call (§2 step 4) — no separate endpoint.

## Acceptance criteria

- [ ] **kill the control plane; customer test pod keeps running.** The agent's poll fails, converge
  is skipped, nothing panics, and no state is mutated — the drill in `verify:`. This is the
  task's headline criterion and the reason the protocol exists.
- [ ] Desired-poll is cell-scoped and token-scoped; a foreign cell id returns 404.
- [ ] `since_generation` returns only rows whose `generation` exceeds it.
- [ ] Writeback advances `observed_generation`; a **stale** generation is rejected; repeating the
  same writeback is a no-op (idempotent); concurrent writebacks apply exactly once.
- [ ] Every status edge emits an event; metering starts at `ready`, never before.
- [ ] No imperative provisioning anywhere — a handler that provisions is a defect (D9). The only
  write path to the cell is desired-state rows.
- [ ] Substrate names (CNPG/ZFS/GKE/GCS/gVisor) appear in **no** customer-visible surface (D8).
  These are internal-plane endpoints; that is not a licence to leak them outward.

## Out of scope

Actual CNPG/K8s manifest rendering (T1.4 and the driver work in T3.4 own it — this task lands the
**protocol** and a renderer seam). No GKE cluster is required, and none should be created: the
trial burn stays ~$0 and the loop is provable against the API alone.

## Read first

`docs/plan/e1-substrate-design.md` §2 · `contexts/provisioning.md` ·
`services/api/internal/platform/db/migrations/20260718203138_services.up.sql` ·
`services/api/internal/provisioning/services.go` (the desired-state writer today)
