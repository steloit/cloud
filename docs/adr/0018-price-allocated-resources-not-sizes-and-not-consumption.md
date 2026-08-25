# ADR-0018 — Price allocated resources, not SIZE bundles and not consumption

**Status:** **ACCEPTED — founder-ratified 2026-08-25** · engineering
**Supersedes nothing.** Implements ADR-041's ratified terminal form. Rates are
founder-owned and are **not** decided here.

## Context

The founder asked whether to move from fixed monthly SIZE pricing (`dev` $19 /
`standard` $58 / `performance` $112, + $0.50/GB storage, + $19 HA) toward
usage-based pay-as-you-go.

Two things had to be established before answering: what the repository already
rules, and what mature platforms actually do.

## What the repository already rules

**PAYG is not a new direction. It is the ratified one.**

- **ADR-022** (founder-ratified): "subscription = platform capabilities,
  **pay-as-you-go = infrastructure**, overage = beyond quotas."
- **ADR-041** (founder-ratified, 2026-07-18): "every product priced by **its
  natural meter** … product line → cost components → **meter rows**; the
  generalized exact-sum invariant; terminal condition: **published unit price ×
  metered quantity** or a stated plan allowance, nothing else exists."

The infrastructure was built that way and then not used that way:

| already PAYG-shaped | verified |
|---|---|
| `usage_events` append-only by trigger, `meter` + `quantity` + open/close edges, no FKs | its own column comment lists `compute_seconds \| cu_hours \| gb_months \| egress_bytes` |
| `money.Cents` — a struct that will not compile `+` or `*`; `money.Accrual` is 128-bit | built for Σ(rate × quantity) |
| `Estimate.basis: "fixed \| usage_projection"` | in the OpenAPI contract |

| the fixed-SIZE artifact | where |
|---|---|
| `base_cents` per SIZE, flat `ha_cents` | `pricing.json` |
| `Σ(seconds × MONTHLY rate)` → needs a month divisor | `rollup.go` — this **is** O30 |
| every line hardcoded `Basis: "fixed"`; `usage_projection` unused | `engine.go:367` |
| exactly **one** meter emitted: `service_span` | `metering.go` |

## What the research found

Full note and citations: `docs/plan/pricing-model-research-2026-08-25.md`.

1. **Nobody meters CPU *consumption* for a database.** Neon (CU-hours), Aurora
   (ACU-seconds), Supabase, PlanetScale, Crunchy, Render, Fly, Cloud SQL all
   meter **allocated capacity × wall-clock time**. Railway is the exception and
   is selling containers, not a managed database. So "PAYG for compute" is, in
   practice, still allocation × time — which is the meter we already have.
2. **Storage splits, and the shrink problem is universal.** Cloud SQL cannot
   shrink; a Kubernetes PVC cannot shrink. Timescale moved provisioned → consumed
   and named the failure ("disk lockage").
3. **HA is a 2×–3× multiplier everywhere it is published** — Cloud SQL "twice as
   much", Crunchy "doubles the cluster price", PlanetScale ≈3×.
4. **Every PAYG vendor documents that a hard cap cannot be hard.** GCP: budgets
   "don't automatically cap". Neon's spending limit is alerts-only. Vercel: usage
   accrues "for several minutes after you cross". Upstash keeps the bound only by
   rate-limiting the database.
5. **PlanetScale moved a database product *away* from usage billing** and wrote
   why: "when predictability, stability, and availability matter most, worrying
   about unbounded and unknown costs is a distraction nobody can afford."

## The decision

**Price ALLOCATED resources, per unit, per unit-time. Do not price consumption.**

The question is not "fixed vs PAYG". It is three options, and the hard spend cap
separates them.

`enforceBudget` projects **"committed monthly run-rate = plan fee + Σ active
services' monthly estimates"**. It does not need a *fixed price*. It needs a
**knowable run-rate at accept time**.

| model | meter | hard cap | HA | Valkey |
|---|---|---|---|---|
| fixed SIZE bundles (today) | span × monthly rate | ✅ price is constant | flat $19 — broken | separate `memory_cents_per_gb` |
| **allocated resources** | allocated units × time | ✅ **allocation is known at accept** | **automatic 2×** | **same vCPU/GB meters** |
| consumed resources | measured usage | ❌ lag; alerts-only | — | — |

Allocation keeps the cap because a service with 2 vCPU + 4 GB + 50 GB allocated
has a fully determined run-rate the moment it is accepted. Consumption does not,
because nobody can know what a query will burn.

**The cap is the deciding factor and it is not merely a feature.** It is the F9
flagship (`openapi.yaml:1025`: "crossing it is impossible by construction, never
alerts-only") and it is the North Star — *eliminate uncertainty* — made
mechanical. Consumption pricing would trade the product's differentiator for
alignment with a cost structure our existing meter already tracks.

### Bill what our supplier bills us, at the same concept

Requested / provisioned / consumed differ per resource, because our own cost does:

| resource | bill on | why |
|---|---|---|
| vCPU | **allocated** | Guaranteed QoS reserves it; GCP charges for the node whether or not it burns |
| memory | **allocated** | same reservation |
| storage | **allocated** | a PD is billed on provisioned GB, and a PVC cannot shrink |
| backups / WAL | **consumed bytes** | GCS bills bytes actually retained |
| egress | **consumed bytes** | GCP bills bytes actually sent |

*Requested* is never billed: a request that is not provisioned costs nothing.
Margin predictability follows from cost-alignment rather than from forecasting.

### The unit is an hour, and that is the point

`line = unit_seconds × rate_per_unit_hour ÷ 3600`. 3600 is exact and carries no
convention. **The month never enters the arithmetic**, which is what dissolves
O30's divisor ambiguity rather than answering it — there is no monthly sticker
price for infrastructure to reproduce, so February legitimately costs less than
January. "One arithmetic everywhere" is about lines summing to the total and is
unaffected.

## Ratification

Founder, 2026-08-25: *"use allocated-resource metering for infrastructure … not
consumption-based CPU/memory metering"*; compute/memory/storage are **allocated
resources × time**, backups/egress are **consumed**; and *"the hard spend cap must
continue to work from a knowable committed monthly run-rate at acceptance time —
do not change the F9 hard-cap model into an alert-only budget."*

Explicitly ruled out: introducing CPU-consumption metering "merely to call the
system PAYG".

## Consequences

- **Not an architecture delta** under ADR-040: the ledger, money types, cap,
  reconciler and estimate contract are unchanged. This is a rate table, a meter
  set, and a presentation — all three of which ADR-041 already fixed the form of.
- **HA stops being priced separately.** Two instances meter twice. `ha_cents` is
  deleted, not re-rated.
- **Valkey becomes consistent for free** (US-3.18) — the same vCPU/GB meters.
- **SIZE becomes a preset**, not a price: a named bundle of (vCPU, GB) whose cost
  is derived. That matches the create frame, which already shows
  `Dev · 1 vCPU · 2 GB · $19/mo` — resources first, price second.
- **Scale-to-zero becomes expressible**: allocation → 0, so cost → 0 (A1.2).
- **T3.4c's storage ratchet survives unchanged** and gets *more* natural: you are
  billed for provisioned GB, which ratchets because the volume ratchets.

## What this ADR does NOT decide — founder-owned

1. **The unit rates.** A single rate cannot reproduce today's ruled prices:
   per (1 vCPU + 2 GB) bundle, `standard` implies **$29.00** and `performance`
   **$28.00** (3.4% apart), while `dev` implies **$19.00** — **34% below**. `dev`
   is a subsidised entry tier and canon pins its totals (`dev+10GB=$24`,
   `dev+4GB=$21`).
2. **Whether `dev`'s subsidy becomes a plan allowance.** ADR-041 already permits
   this shape — "unit price × metered quantity **or a stated plan allowance**" —
   so one honest rate everywhere with the entry subsidy expressed as an allowance
   is available without amending any ratified decision.

Until (1) is ruled, no rate table changes. The meter set, the arithmetic and the
correctness fixes below do not depend on it.
