---
id: US-3.16
title: "ha: true is charged $19/mo and renders a single instance — the replica does not exist"
epic: E3
status: ready
phase: MVP
priority: critical
sprint: 3
estimate: 0.5ew
deps: []
issue: 0
labels: [Backend, Billing, Reliability]
module: M3 Provision
contexts: [provisioning, billing]
files:
  - services/cell-agent/internal/render/**
  - services/cell-agent/internal/driver/cnpg/**
  - services/api/internal/estimates/**
  - tasks/e3-provisioning/US-3.16.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -count=1 -race ./..."
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go test -count=1 ./internal/estimates/"
owner: agent
---

## The defect, measured

Rendered through the real driver, changing nothing but `ha`:

```
ha=false -> instances: 1
ha=true  -> instances: 1
```

`estimates.Price` adds `ha_cents` = **1900** ($19/month) when `ha: true`
(`engine.go:404`). The cell renders **the same single-instance cluster either
way**. A customer who buys high availability pays for it every month and has no
replica, no standby, and no failover.

## Why nothing caught it

`render.instancesOf` reads `desired["override"]["instances"]` and otherwise
returns `1`. It never reads `shape["ha"]` — grep for `"ha"` across
`services/cell-agent/` returns nothing. The shape reaches the desired doc intact;
the driver simply has no code that consumes that key.

So this is the **priced / rendered / enforced** split the mistake bank already
records, in its most expensive form: the dimension is priced and the manifest is
correct *for one instance*, and no test compares the two sides because no test
renders with `ha` at all.

The estimate side is well tested. The driver side is well tested. **The seam
between "what we sold" and "what we build" has no test**, which is the same shape
as T3.4c's `storage_gb` defect — a driver that never read the key the customer
was billed for.

## What is a decision, and what is not

**Not a decision:** that `ha: true` must provision more than one instance. It is
sold as high availability; one instance is not.

**A decision, and it is the founder's:** *how many*. CNPG's own documentation
treats `instances: 3` as the standard HA shape (one primary, two replicas);
`instances: 2` is a valid smaller form with weaker failure tolerance. The number
changes both the price justification and the quota arithmetic:

| ha renders | postgres `performance` + ha, under US-3.3d's *proposed* sizing | `business` envelope |
|---|---|---|
| 2 instances | 8 vCPU / 16 GiB | 12 / 24 GiB — fits |
| 3 instances | 12 vCPU / 24 GiB | 12 / 24 GiB — **exactly the ceiling** |

That second row is the consequence **US-3.3d** states, and it is true only at
three instances. So this task and US-3.3d share one decision and should be ruled
together.

## Acceptance criteria

1. `ha: true` renders more than one instance, and `ha: false` renders exactly one.
2. **A test drives the seam**: for a shape the estimate charges `ha_cents` for,
   the rendered cluster has >1 instance. Assert on the two sides of the sale, not
   on either alone — the existing tests each pass while the product is broken.
3. Deleting the `ha` handling makes that test fail. Prove by mutation.
4. The instance count comes from **one** place. `instancesOf` already merges an
   override pin; `ha` must not become a second, independent source that can
   disagree with it — decide the precedence and test it (an override of 1 on an
   HA service: refused, or honoured?).
5. Existing HA services, if any, are reconciled — or the absence is verified and
   recorded. (Fleet is empty today: 0 GKE clusters, verified 2026-08-25.)

## Notes

Found while verifying US-3.3d's stated consequences against the ruled
per-environment envelope. Both of US-3.3d's claims check out arithmetically; this
turned up underneath them.

**Billing consequence, for whoever rules the count:** every month a customer has
been charged for HA is a month they did not receive it. The fleet is empty, so
today that is zero customers — which is the cheapest moment this will ever be
fixable.
