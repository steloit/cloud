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

## Implementation analysis, 2026-08-25 — the engineering half

The commercial number is **not** decided here. What follows is what HA actually
provisions, how that enters ADR-0018's allocation meter, and what has to change.

### What `ha: true` provisions, measured

| resource | HA effect | evidence |
|---|---|---|
| instances | 1 → **2** | `render.haInstances`, cited to the create frame's "standby + auto-failover" |
| vCPU | **×2** | each instance is a full Postgres pod |
| memory | **×2** | same |
| **storage (PVC)** | **×2** | **one PVC per instance** — CNPG: *"The operator creates a PVC for each PostgreSQL instance"*. `spec.storage.size` is per-instance, so `instances: 2` + `50Gi` provisions **100Gi**. Storage is **duplicated, not shared.** |
| **backups / WAL (GCS)** | **×1** | WAL archives as a **single stream** to the object store, and a backup runs on one instance. Retained bytes do **not** double. |
| egress | ≈×1 | customer traffic reaches the primary; replication is intra-cluster |

**The asymmetry is the finding.** HA is not a uniform multiplier: it doubles the
three *allocated* meters and leaves the *consumed* backup meter alone. That falls
straight out of ADR-0018's split — we pay GCP twice for reserved CPU/memory/disk
and once for the bytes actually in GCS.

It also lands independently on the same semantics Cloud SQL publishes ("an
HA-configured instance costs twice as much… includes CPU, RAM, and storage",
backups priced separately) — derived from our own infrastructure rather than
copied.

### How it enters the allocation meter

Under ADR-0018 it does not enter as anything. **HA stops being a priced dimension**
and becomes a consequence of allocation:

    allocated_vcpu_seconds   = instances × vcpu_per_instance × seconds
    allocated_memgb_seconds  = instances × gb_per_instance   × seconds
    allocated_storagegb_secs = instances × storage_gb        × seconds
    backup_gb_seconds        =             retained_bytes    × seconds   (unchanged by HA)

`instances` is already computed (`render.instancesOf`) and already correct. The
meter needs the *allocation*, which the control plane knows at accept time — which
is exactly why the F9 cap survives.

### Where `ha_cents` enters today, and what assumes flat

| site | what it does |
|---|---|
| `estimates/pricing.json:10` | `"ha_cents": 1900` — the flat constant |
| `estimates/engine.go:38` | `HACents int64` on the catalog struct |
| `estimates/engine.go:400-410` | reads `shape["ha"]`, adds `HACents` once, size-independent |
| `estimates/engine.go:161` | `"ha": {"bool", false}` in `shapeSchema` |

Only the *pricing* side assumes flat. The **cell side is already correct** —
`instancesOf` returns 2 and every instance gets its own pod and PVC — so the
mis-pricing is confined to `estimates`, which is the cheapest place for it to be.

`shape["ha"]` itself **stays**: it is the customer's expressed intent, it is in the
closed schema and the API contract, and it is what `instancesOf` reads. What goes
is the *flat price attached to it*.

### What changes when the rates land

1. **Delete `ha_cents`** from `pricing.json` and `engine.go` — not re-rate it. A
   per-size `ha_cents` table would reproduce the same numbers with more places to
   drift, and would still be a price for a thing rather than for the resources it
   provisions.
2. **Price from allocation**: `Price` multiplies the resource meters by
   `instancesOf(shape)`, so HA needs no branch at all. A future third replica, or
   a read replica, prices correctly without touching the pricing code — which is
   the test that this is the right abstraction.
3. **Backups stay ×1** — they are a consumed meter and must not inherit the
   instance multiplier. This is the one place a naive "multiply everything by
   instances" would over-bill.

### Tests that protect the invariant

The invariant is: **infrastructure pricing reflects the actual additional
allocated resources introduced by HA.**

1. `TestHAPricesExactlyTheResourcesItProvisions` — price `{ha:false}` vs
   `{ha:true}` and assert the delta equals *one instance's* allocated
   vCPU+memory+storage at the unit rates. Not "double the total": the total also
   contains backups, which must not double.
2. `TestBackupsDoNotDoubleUnderHA` — the discriminator for the asymmetry above,
   and the one a naive implementation fails.
3. `TestNoFlatHAConstantSurvives` — grep-free structural check: `pricing.json` has
   no `ha_cents`, and the estimate for `{ha:true}` changes when a *resource* rate
   changes and not when any HA-specific constant does (there is none).
4. Extend `render.TestEveryPricedDimensionMovesItsOwnProvisionedField` — with
   `ha_cents` gone, HA is covered by the vCPU/memory/storage rows automatically,
   which is the point. The catalog-completeness check then proves no orphan
   remains.
5. A cap test: an org near its bound cannot accept an HA service whose *doubled*
   allocation crosses it — the F9 promise has to hold for the resources HA adds,
   not just the base ones.

### Blocked on the commercial decision only

Everything above is engineering. What remains is the founder's: **the unit rates**,
from which the HA price is then *derived* rather than set. Deriving a commercial
price from today's `$19` would be reverse-engineering a number already established
as wrong (17% uplift on `performance` where every comparable is 2×–3×), so it is
not used as an anchor.