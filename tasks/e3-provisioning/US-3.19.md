---
id: US-3.19
title: "HA is priced as a flat $19 uplift while every comparable is a 2x-3x multiplier"
epic: E3
status: blocked
phase: MVP
priority: high
sprint: 3
estimate: 0.25ew
deps: []
issue: 0
labels: [Billing]
module: M3 Provision
contexts: [billing, provisioning]
files:
  - services/api/internal/estimates/**
  - docs/founder-config.md
  - tasks/e3-provisioning/US-3.19.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go test -count=1 ./internal/estimates/"
owner: founder
---

## The defect, measured

`pricing.json` charges `ha_cents: 1900` regardless of size:

| size | base | + HA | uplift |
|---|---|---|---|
| `dev` | $19 | $19 | **100%** |
| `standard` | $58 | $19 | **32.8%** |
| `performance` | $112 | $19 | **17.0%** |

Every published comparable is a multiplier, not a flat fee:

- **Cloud SQL**, verbatim: "An HA-configured instance costs twice as much as a
  standalone instance. This price includes CPU, RAM, and storage."
- **Crunchy Bridge**: HA "doubles the cluster price"; replicas bill at full rate.
- **PlanetScale**: HA is a 3-node cluster, ≈3× a single node.
- **Aurora**: tier-0/1 readers have "minimum capacity defined by the current
  writer capacity" — a standby costs *at least* the primary.

A 2× multiplier on `performance` would be **$112**, not $19.

## Why this is urgent now and was not before

**US-3.16 made it real on 2026-08-25.** Until then `ha: true` was charged and
rendered `instances: 1` — HA provisioned nothing, so the mis-pricing cost nothing
and was invisible. Now a `performance` + `ha` customer consumes a genuine second
cluster — its own pod, its own CPU and memory reservation, its own PVC — and pays
$19 for it.

The fleet is empty (0 clusters, verified 2026-08-25), so no customer is on it yet.
That is the cheapest moment to correct it and the reason this is filed rather than
absorbed.

## The decision, and it may be moot

**If ADR-0018 is adopted** (price allocated resources, not SIZE bundles), this
question **disappears rather than being answered**: two instances meter twice, and
`ha_cents` is deleted rather than re-rated. HA stops being a priced dimension at
all and becomes a consequence of allocation. That is the structural fix.

**If ADR-0018 is declined**, `ha_cents` must become a multiplier, and the founder
must rule it — it is a published price. Options: 2× (Cloud SQL / Crunchy
precedent), or a per-size `ha_cents` table, which reproduces the same numbers with
more places to drift.

Implementation must not pick either. `owner: founder`.

## Acceptance criteria

1. The uplift is proportionate to what HA actually provisions, by whichever route
   is ruled.
2. Whatever is chosen, a test asserts the relationship rather than the constant —
   e.g. "HA on any size costs at least the resources of a second instance" — so a
   new size cannot reintroduce a flat uplift by inheriting it.
3. Canon's `db-main standard+50GB=$58` and the other pinned totals still
   reproduce, or the canon change is deliberate and recorded.
