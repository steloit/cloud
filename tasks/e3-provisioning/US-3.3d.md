---
id: US-3.3d
title: "A managed Postgres declares no resources, so any LimitRange default silently becomes its hard cap"
epic: E3
status: blocked
phase: MVP
priority: high
sprint: 4
issue: 0
labels: [Platform]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/cell-agent/internal/driver/cnpg/**
  - services/cell-agent/internal/render/**
  - services/api/internal/estimates/**
  - docs/founder-config.md
  - tasks/e3-provisioning/US-3.3d.md
verify:
  - "for every shape the catalog sells, the rendered Cluster declares resources matching the sold shape"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -race ./..."
owner: agent
---

## Goal

The Cluster we render must declare the resources the customer paid for.

## Why — US-3.3a's security review

`services/cell-agent/internal/driver/cnpg/templates/cluster.yaml.tmpl` contains no
`resources:`, no `memory` and no `cpu` at all. Two consequences, and the second is
the one that bit:

1. **A customer buying a larger shape gets a pod with no resource floor.**
   Scheduling and eviction ignore what was sold.

   *(This task originally said the estimate "resolves `memory_mb`
   (engine.go:167 — default 1024) and prices it". That line is **valkey's**.
   Verified: the postgres shape is `{size, storage_gb, ha, connections, pgmq,
   version}` — no cpu, no memory — and `memory_cents_per_gb` in `pricing.json`
   is valkey's knob. Nothing carries cpu or memory to a postgres pod because
   nothing produces them. See the block below.)*
2. **Any LimitRange in the namespace silently becomes the hard cap.** US-3.3a
   tried to ship `default: {cpu: 500m, memory: 512Mi}` as part of D7. With no
   `resources:` on the Cluster that default is applied at pod admission to every
   managed Postgres — *existing ones included*, since SSA reapplies each converge
   and it bites on the next restart or failover. A "standard" Postgres OOMKilled
   at 512Mi is a product regression arriving through an isolation task.

This is why the LimitRange was withdrawn from US-3.3a rather than shipped. It is
**not** blocked on NetworkPolicy enforcement (US-3.3c) — a LimitRange is enforced
by the API server at admission, needs no CNI, and the fix belongs with the CNPG
driver rather than with an isolation task.

## BLOCKED: the catalog does not say what a postgres size IS

AC 1 asks for resources "derived from the sold shape". **The sold shape does not
contain them, and neither does the catalog.** Verified three ways:

- `shapeSchema["postgres"]` is `{size, storage_gb, ha, connections, pgmq,
  version}` — no cpu, no memory.
- `pricing.json`'s postgres sizes carry `base_cents` and `included_gb` only;
  the single `memory_cents_per_gb` in that file is valkey's.
- No document in `docs/` states what `dev` / `standard` / `performance` mean in
  vCPU or RAM.

So implementing AC 1 means **inventing** the numbers — deciding that `standard`
is, say, 2 vCPU / 4 GiB. That is a product and pricing decision (the sizes are
priced at $19 / $58 / $112 a month and must correspond to something), and the
founder ruling of 2026-07-27 is that implementation code never makes those.

This is the same shape as US-3.3e's storage envelope: the mechanism is ready,
the numbers are not mine to choose.

## A researched proposal, so the decision is not a blank page

Market anchors for what a dollar buys in managed Postgres today:

| provider | price/mo | vCPU / RAM |
|---|---|---|
| DigitalOcean Basic | **$15** | 1 shared vCPU / 1 GiB |
| DigitalOcean Growth | **$61** | 2 vCPU / 4 GiB |
| DigitalOcean General Purpose | ~$120-130 | dedicated vCPU / 8 GiB |
| Aiven entry | $19 | ~2 vCPU |
| Aiven mid | ~$110 | 4 vCPU |
| Supabase Pro | $25 | (bundled, not compute-tiered) |

Against our own prices — `dev` $19, `standard` $58, `performance` $112 — a
defensible starting point that lands us at or slightly above market:

| size | proposed | sits against |
|---|---|---|
| `dev` | **0.5 vCPU / 1 GiB** | DO Basic $15 = 1 shared vCPU / 1 GiB |
| `standard` | **2 vCPU / 4 GiB** | DO Growth $61 = 2 / 4 GiB, and we are $58 |
| `performance` | **4 vCPU / 8 GiB** | DO GP ~$120 = 8 GiB, and we are $112 |

Two constraints the numbers must satisfy, both already ruled:

- **They must fit the plan envelope** (founder 2026-08-23), which is per
  ENVIRONMENT: `free` 1 vCPU / 2 GiB. A `dev` at 0.5/1 fits with room for a
  second; `standard` at 2 vCPU does NOT fit a free environment at all, which is
  consistent — `standard` is not a free-tier shape.
- **`ha: true` multiplies them by the instance count**, so `performance` + HA is
  12 vCPU / 24 GiB against `business`'s 12 / 24 — exactly at the ceiling. Worth
  seeing before ruling.

## How they should be applied — this part IS decided by evidence

**Guaranteed QoS: `requests == limits` for both cpu and memory.** CloudNativePG
recommends it explicitly — *"For a PostgreSQL workload it is recommended to set a
'Guaranteed' QoS"* — and it is what enables their OOM protection: the postmaster
keeps its `-997` OOM score while children run at `0`, so the kernel kills a
backend before the postmaster.

The counter-argument is real and was weighed: a CPU **limit** means CFS quota
throttling, and for latency-sensitive workloads the usual advice is to omit it.
It loses here for a specific reason — this is a MULTI-TENANT managed product that
sells a bounded shape. Without a limit, one tenant's database bursts into
another's CPU on a shared node, and the plan envelope (a quota on `requests.*`)
stops describing what actually runs. A customer who bought 2 vCPU should get 2
vCPU, predictably, and not be a noisy neighbour. CNPG also suggests sizing
`shared_buffers` at ~25% of pod memory, which is a follow-up once the numbers
exist.

## Acceptance criteria

1. The rendered Cluster declares `resources.requests` and `resources.limits`
   derived from the sold shape, for every product the driver renders.
2. A test asserts, for **each** shape in the catalog, that the rendered values
   match the shape rather than a constant — a single-shape test passes on a
   hardcoded literal.
3. Only then, a LimitRange whose defaults sit *underneath* the declared
   resources, with a test that no catalog shape is capped below what it sold.
3a. **US-3.3e already shipped the LimitRange, with `defaultRequest` only and no
   `default`** — so nothing is capped today, and this AC is now about what that
   deliberately left undone. Until the Cluster declares its own requests, every
   CNPG container is admitted at the environment's `defaultRequest`, which means
   the plan's cpu and memory ceilings bind as a **pod-count proxy** rather than
   as compute the customer paid for. Storage is the only dimension of the
   founder's envelope that truly binds. Closing AC 1 is what makes the compute
   half real; a `default` may only be added once AC 1 is green, and it must sit
   above the largest shape the catalog sells, not at a round number.
4. The golden fixtures in `internal/driver/cnpg/testdata/` are regenerated and
   the diff is read, not just accepted.

## Read first

- `services/cell-agent/internal/driver/cnpg/templates/cluster.yaml.tmpl`
- `services/api/internal/estimates/engine.go` (`memory_mb` resolution)
- commit `7e94f26` — the withdrawn LimitRange, and why its defaults were wrong
- `services/cell-agent/internal/driver/tenancy/tenancy.go` — the LimitRange as
  US-3.3e reinstated it (`defaultRequest` only), and its package doc
- `docs/founder-config.md` §5 — the ruled envelope, and the row recording that
  the compute half does not yet bind
