---
id: US-3.16
title: "ha: true is charged $19/mo and renders a single instance — the replica does not exist"
epic: E3
status: done
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
  - tasks/e3-provisioning/US-3.3d.md
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

**NOT a founder decision after all — the repository answers it.** The create
frame sells HA as, verbatim:

> **High availability · standby + auto-failover · +$19/mo**

One **standby**, singular, plus failover. Microcopy in `00-sources/` is
verbatim-binding, so primary + one standby = **2 instances** is what was sold.
CNPG's more common production shape is 3 (surviving a node loss while keeping a
standby), and that would have been the easy guess — but it is not what this
product promises, and inventing a bigger number would silently change both the
price justification and the quota arithmetic:

| ha renders | postgres `performance` + ha, under US-3.3d's *proposed* sizing | `business` envelope |
|---|---|---|
| 2 instances | 8 vCPU / 16 GiB | 12 / 24 GiB — fits |
| 3 instances | 12 vCPU / 24 GiB | 12 / 24 GiB — **exactly the ceiling** |

US-3.3d's stated consequence — "performance + ha is exactly business's ceiling" —
is therefore **wrong**: at the two instances the frame sells, `performance` + `ha`
is 8 vCPU / 16 GiB and fits `business` (12 / 24 GiB) comfortably. That correction
is recorded on US-3.3d.

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

## Outcome

**Fixed structurally, not by adding a branch.** `instancesOf` now takes the
**maximum** of the HA floor and any manual capacity pin, rather than letting one
"win". A precedence rule would leave a pin of 1 able to remove the standby a
customer is billed $19/month for — silently, for 24 hours at a time, with the
price untouched. `max()` makes that unrepresentable: **no value of `override`
drops a service below what it was sold.**

`ha: true` renders **2** instances, and the number is cited to the frame that
sells it rather than chosen.

### The root cause was the seam, so that is what got the guard

The defect was not that someone forgot `ha`. It is that *priced* and
*provisioned* were two representations with nothing tying them together — the
estimate suite was green, the driver suite was green, and customers got one
instance. `TestEveryPricedDimensionChangesWhatIsApplied` asserts the property:
for each shape key the catalog charges for, two converges differing **only** in
that key must apply different manifests. A dimension that changes the bill and
not the cluster now fails, whoever adds it next.

### A row I had to remove from that test, because it passed for the wrong reason

`base_cents` (`{size:dev}` vs `{size:standard}`) **passed** — but only because
their storage floors differ, so it was answered by the storage dimension one row
down. An assertion satisfied by a different object is the exact trap the test
exists to catch, so it must not commit it itself.

Measured instead, and it is a real finding:
`TestTheSizeYouPayForBuysOnlyItsStorageFloor` holds storage equal and shows
`dev` ($19) and `standard` ($58) render **byte-identical** manifests. The $39
difference buys nothing the cell builds. It asserts today's truth deliberately, so
it **fails when US-3.3d lands** — a reminder in the right place rather than a
comment nobody reads.

### Evidence

| mutation | result |
|---|---|
| revert the fix — `ha` unread again | **RED** (3 tests) |
| `haInstances = 1` — a standby that is not one | **RED** (3 tests) |
| `max()` → precedence, so a pin can drop the standby | **RED**, exactly the test that owns it |

`services/cell-agent` full suite green under `-race`; `estimates` green.

**Billing:** every month a customer was charged for HA is a month they did not
receive it. The fleet is empty (0 clusters, verified), so that is zero customers —
which is why this was the cheapest possible moment to find it.

## Review + QA round 1 — the guard I wrote could pass with nothing provisioned

**QA broke the seam guard, and the way they broke it is the finding.** Neutralise
both priced reads AND add a `steloit.dev/shape` annotation to the template — a
routine SSA drift-detection pattern, not a contrived change — and
`TestEveryPricedDimensionChangesWhatIsApplied` **passed**, both subtests, with the
cell provisioning neither dimension. The manifests differed; they just differed
somewhere that builds nothing. *That is US-3.16's own bug, reported green by the
test written to prevent it.*

Rewritten: each row now names the **provisioned field** its dimension must move
(`ha_cents` → `spec.instances`, `storage_cents_per_gb` → `spec.storage.size`) and
the assertion reads that field by path out of the applied Cluster. A row without a
reader does not compile. Plus `TestTheSeamGuardCanStillFail`, which renders two
shapes differing in an **unpriced, unread** key (`version`) and asserts the
manifests are identical — the alarm the guard cannot raise about itself.

**The row set is now bound to the catalog.** Review measured that adding
`postgres.connections_cents` to `pricing.json` left every package `ok` — a
hand-written list claiming "whoever adds it next" while nothing binds it to the
bill. It reads `pricing.json` from the repo root (the cell-agent must not import
the API's table; precedent is `cnpg_test.go`'s `catalogSizes`) and fails on any
priced key with no row. `base_cents` is a **named** exemption, so deleting the
US-3.3d reminder test cannot silently drop a $39/month dimension.

**The input space.** QA probed six classes that silently returned the wrong count:
`ha: "true"` meant no standby — this task's own bug arriving by type rather than
by omission — and a pin held as an `int` or `json.Number` vanished. `asFlag` and
`asCount` now accept every shape a desired doc can carry, which is the rule
`cnpg.asGB` already recorded one file away.

**A floored pin is logged.** The doc said `max()` made dropping the standby
"unrepresentable"; the code made it *silently ignored*. An operator's pin carries
a stated reason (D22), and the expiry path emits an event — so this one warns.

**Reachability, stated.** A postgres pin cannot be set through the API at all:
`PriceWithInstances` 422s it because postgres's shape has no `instances` key
("capacity we cannot price is capacity we must not provision"). The `max()` is a
floor for hand-planted rows, and now says so rather than reading as a live guard.

### Filed, not fixed here

- **US-3.17** — `storage_node_count = 1` plus CNPG's default *preferred*
  anti-affinity means both instances land on one node, silently. Pod-level
  failover works; node loss takes both. That is an infrastructure/capacity
  decision, not a renderer change.
- **US-3.18** — valkey has the identical priced-not-provisioned defect in
  committed code (`memory_cents_per_gb` charged, no `resources:` rendered), and a
  valkey create is accepted by the API while no driver is wired, so it can never
  converge. Reachable today.

### Not retroactive, and that is recorded

`instancesOf` is cell-side, so no desired-doc rewrite and therefore no generation
bump: a service already `ready` with `ha: true` stays at one instance until
something touches it. The fleet is empty (0 clusters, verified 2026-08-25), so
that is zero customers — but it is a remediation gap, not just a billing note, and
US-3.17 carries it.