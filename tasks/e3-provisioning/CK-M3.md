---
id: CK-M3
title: Estimate-gated provisioning end-to-end
epic: E3
status: done
phase: MVP
priority: critical
sprint: 4
estimate: 0ew
deps: [US-3.2, US-3.3]
issue: 187
labels: [milestone-checkpoint]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files:
  - services/api/internal/identity/ckm3_checkpoint_test.go
  - tasks/e3-provisioning/CK-M3.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && STELOIT_CHECKPOINT=1 go test -count=1 -run CKM3 -v ./internal/identity/ 2>&1 | grep -q '^--- PASS: TestCKM3'"
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go test -count=1 -run TestCanonArithmetic -v ./internal/estimates/ 2>&1 | grep -q '^--- PASS: TestCanonArithmetic'"
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go build ./... && go vet ./... && go test ./..."
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go build ./... && go vet ./... && go test -race ./..."
  - "cd \"$(git rev-parse --show-toplevel)/apps/cli\" && go build ./... && go vet ./... && go test ./..."
owner: agent
---

## Goal

Estimate-gated provisioning end-to-end

## Summary

**Exit criteria:**
- [x] `steloit db create` → estimate → approved → `ready`, metered —
  `TestCKM3EstimateGatedProvisioningEndToEnd` drives the REAL binary (built from
  `apps/cli`, not imported) against the real API on real Postgres. It asserts:
  the operator is shown the EXACT price (`$24/mo`) as a per-service line and a
  total, and is shown it BEFORE anything provisions (proved by declining the
  prompt and finding zero services); the shape provisioned is the shape priced
  (product, size, storage_gb, and cents); nothing is billed before `ready`; the
  accepted estimate is burned exactly once while the declined one is not; the
  row is reachable by a cell (`reconcile.Desired`) and reaches `ready` through
  `reconcile.Writeback`; the span is BILLABLE — `service_span`, `open`, rate
  2400, tagged to the right org and env; and `db list` round-trips `ready`.
- [x] Canon arithmetic invariants green against the estimate engine —
  `internal/estimates.TestCanonArithmetic` prices every canon service through the
  real engine, asserts each line against canon, the $208 Σ anchor imported from
  the shared package, and the four equations.

## Unblocked (2026-07-26)

The blocker text was stale: P1 landed 2026-07-19, and US-3.3 proved the CELL
half against real GKE+CNPG. Both deps are done. What US-3.3 did NOT do — and
this checkpoint does not do either — is the two-process live drill; see
`## Exit evidence` below.

## Exit evidence — scope and carry-forward

**Proved here:** the CLI seam. The binary a customer runs walks the estimate gate
the API enforces, and a CLI-created row is reachable by a cell and reaches
`ready` through the control plane's half of the reconciler protocol
(`Desired`/`Writeback`) — the same path a cell-agent's report takes.

**NOT proved here, carried forward:** a live two-process drill (a real
cell-agent converging a real CNPG cluster and reporting `ready`). US-3.3 proved
the cell half against real GKE, but its own Outcome records that the api+agent
two-process run was not re-executed. This checkpoint deliberately runs without a
cluster so it can gate CI; the live drill remains outstanding and belongs to the
first task that needs a standing cell.

## Outcome

The checkpoint existed because every layer was covered in isolation and the SEAM
was not: nothing proved that the binary a customer runs walks the estimate gate
the API enforces. It does, and the proof is a chain rather than an assertion —
real binary, real API (the same `httpapi.Chain` main.go builds), real Postgres.

Mutation-verified — each of these fails the checkpoint: hiding the estimate; a
wrong total above correct line items; deleting the per-service lines; dropping a
shape flag; a prompt whose answer never takes (so only `--yes` works); a desired
doc that loses its shape or namespace; and opening a billing span before `ready`.
The price is derived from `canon.Load()` rather than retyped, so the engine is
asserted against canon rather than against this test. The billing direction is also covered by
`TestMeteringStartsAtReadyE2E` and `TestFailedProvisioningNeverBills` — what is
new here is that none of it was ever exercised THROUGH THE CLI, whose own suite
runs against a fake API.

Two incidental findings, neither blocking: the task's `## Blocked` note was
stale (it cited P1, which landed 2026-07-19, via US-3.3, which shipped a live
cell), and a personal token defaults to `read_only`, so the checkpoint mints a
`full`-scope one — which is the CLI's documented behaviour, surfaced here because
the first run failed on it.

Evidence: `services/api` 22 packages RC=0, `apps/cli` 2 packages RC=0,
`services/cell-agent` 5 packages RC=0 under `-race`; zero failures, zero skips.
