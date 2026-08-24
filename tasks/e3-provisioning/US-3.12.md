---
id: US-3.12
title: "The reconcile status contract advertises two values the API now refuses, and one that no longer means what it says"
epic: E3
status: ready
phase: MVP
priority: high
sprint: 4
estimate: 0.25ew
deps: []
issue: 0
labels: [Backend, API]
module: M4 Provisioning
contexts: [provisioning]
files:
  - docs/product/08-api/openapi.yaml
  - services/api/internal/httpapi/gen/**
  - packages/contracts/**
  - services/api/internal/reconcile/**
  - tasks/e3-provisioning/US-3.12.md
verify:
  - "the request enum for POST /reconcile/{cell}/status admits exactly what a cell can report, asserted against provisioning.ReportableByCell rather than a retyped list"
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go build ./... && go test -count=1 -race ./internal/reconcile/"
owner: agent
---

## The drift

US-3.3h made the control plane refuse two reported statuses and changed what a
third means. `openapi.yaml` still advertises the old contract, and it is the
authority generated clients are built from.

`docs/product/08-api/openapi.yaml:1245`:

```yaml
status: {type: string, enum: [provisioning, ready, degraded, failed, suspended, deleting, gone],
         description: "ADR-024 vocabulary; omit for an observation-only heartbeat; gone reports a completed teardown"}
```

Three problems, all measurable against the implementation:

1. **`suspended` and `deleting` are now 422** (`provisioning.ReportableByCell`,
   enforced in `reconcile.Handlers.status`). They are in the enum because it was
   copied from the customer-facing `ServiceStatus`, but a cell cannot *observe* a
   lifecycle decision — and accepting them let one POST with the reconciler token
   brick a service permanently, which is why US-3.3h closed it. An agent
   generated from this contract believes both are valid.
2. **"omit for an observation-only heartbeat"** now returns 409 on a
   `provisioning` or `degraded` row: an ack asserts nothing about where the row
   rests, so it finishes a generation only on a settled one.
3. **"gone reports a completed teardown"** is true only from `deleting`. From a
   live row `gone` means the workload vanished while desired still wants it
   alive, and the row deliberately stays outstanding so the agent re-creates it.

## Why it is a separate task

`openapi.yaml` is outside US-3.3h's `files:` globs, and narrowing a request enum
regenerates `httpapi/gen` and `packages/contracts` — an owner-level contract
change, not a drive-by (AGENTS.md §7).

## Acceptance criteria

1. The request enum admits exactly the statuses a cell can report, plus `gone`.
2. The description states when an omitted status finishes a generation and what
   `gone` means from a live row.
3. A test binds the enum to `provisioning.ReportableByCell` (plus `gone`/"") so
   the two cannot drift again — reading the contract, not a retyped list. The
   T3.4c/US-3.3e pattern.
4. Generated types are regenerated, not hand-edited (AGENTS.md hard rule).

## Read first

- `services/api/internal/provisioning/services.go` — `ReportableByCell`, `ObservedStatus`
- `services/api/internal/reconcile/http.go` — `statusVocab` and the 422 arm
