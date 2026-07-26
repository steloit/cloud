---
id: US-3.7
title: "The estimate gate matches on PRICE, so a colliding shape provisions something you did not price"
epic: E3
status: done
phase: MVP
priority: high
sprint: 4
estimate: 0.5ew
deps: []
issue: 0
labels: [Backend, Billing]
module: M4 Provisioning
contexts: [provisioning, api-conventions]
files:
  - services/api/internal/provisioning/services.go
  - services/api/internal/estimates/**
  - services/api/internal/identity/services_integration_test.go
  - tasks/e3-provisioning/US-3.7.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go build ./... && go vet ./... && go test ./..."
owner: agent
---

## The defect

`CreateService`'s estimate gate accepts a shape when the estimate contains a
line with the same **product and price** — not the same shape
(`internal/provisioning/services.go`, the `priceOf(sh) == line.MonthlyCents`
match). Prices collide, so this is reachable:

| priced | provisioned | both |
|---|---|---|
| `postgres` `{size: dev, storage_gb: 78}` | `postgres` `{size: standard}` | **5800¢** |

`dev` = 1900 + 50·78 = 5800. `standard` = 5800 base (50 GB included).
The gate matches, and the customer receives a **standard** instance having
priced a **dev** one — different CPU/memory class, and 50 GB of storage instead
of the 78 GB they priced.

The law it breaks is stated in the gate's own remediation string: *"Estimate the
exact shape you are creating, accept it, then create — the estimate IS the
contract."* Today the contract is only the price.

## Why it matters

Both shapes cost the same, so this is not a billing leak — it is a **contract**
leak. What provisions is not what was priced, in either direction:

- price `dev`+78 GB, receive `standard`+50 GB → the customer loses 28 GB they
  priced, and we hand out a larger compute class than we sold;
- price `standard`, receive `dev`+78 GB → we provision a cheaper compute class
  than the customer bought.

It requires a deliberately crafted request, not an accident, so severity is
moderate — but the gate exists precisely to make the crafted request impossible.

## The fix

Compare the **shape**, not its price. The estimate already stores the priced
shapes (`estimates.shapes`), so the gate can match on (product, canonical
shape) — canonical because `{size:"dev"}` and `{size:"dev", ha:false}` must not
be treated as different requests. Decide whether an omitted key equals its
default (they price identically, so they should match) and pin that decision.

Keep the price check as well: shape match AND price match is strictly stronger
than either, and a shape that matches but re-prices differently means the
pricing table moved under a live estimate — which should also refuse.

## Acceptance criteria

- [x] Estimating `{size: dev, storage_gb: 78}` and creating `{size: standard}` is
  **refused** with the conflict problem + remediation —
  `TestEstimateGateRefusesAPriceCollidingShape`, which also asserts the reverse
  substitution is refused, that the refusal does NOT burn the one-shot estimate,
  and that nothing was provisioned.
- [x] The same configuration spelled with omitted defaults still matches — the
  same test creates successfully with `{size: standard, ha: false, storage_gb: 0}`
  against a `{size: standard}` estimate, and `TestCanonicalIdentity` covers
  omitted-vs-explicit defaults, omitted-vs-default intent, differing names, and
  the valkey/web default cases.
- [x] A shape whose price has changed since the estimate was issued is refused —
  `TestEstimateGateRefusesAShapeRepricedSinceTheEstimate` moves the stored line
  under a live estimate (what a pricing deploy looks like from the gate's side)
  and asserts a 409 naming repricing, with nothing provisioned. Mutation-verified.
  The first implementation compared two freshly-computed prices, which can only
  ever agree with themselves — review caught that the branch was dead and the
  criterion was ticked with no test.
- [x] The refusal happens BEFORE the one-shot estimate is burned (unchanged
  ordering; asserted).
- [x] Mutation-verified: reverting the gate to a price-only match reproduces the
  exploit exactly — a `standard` instance provisioned from a `dev`+78 GB
  estimate, 201 with `"shape":{"size":"standard"}`.

## Found by

CK-M3's QA review (2026-07-26). The checkpoint certifies "estimate-gated
provisioning end-to-end" and did not exercise a colliding shape; the collision
was computed from `internal/estimates/pricing.json` and confirmed against the
gate's matching logic.

## Related

US-3.2 (the gate) · CK-M3 (checkpoint that surfaced it) · F2 (estimate-before-
provision law) · ADR-025 (integer cents)

## Beyond the original scope — what review found

The reported collision was one instance of a broader defect: the gate compared a
DERIVED property (price) instead of the contract itself. Review reproduced three
further substitutions live, using fields that are declared but unpriced —
`version` 16→17, `pgmq` `{dlq:true}`→`{dlq:false}`, `connections`
`{max:50}`→`{max:5000}` — each returning 201 with the substituted value
persisted into the row AND into the desired doc handed to the cell.

It also found a type-confusion hole: the shape helpers silently fell back to
defaults on a wrong type while `CreateService` persisted the RAW request map, so
an estimate for `{storage_gb: 0}` at 1900¢ accepted a create of
`{storage_gb: "78"}` — priced as 0 GB, stored as `"78"`. Not exploitable today
only because no driver reads that field yet.

Both are closed, along with the drift hazard of retyping defaults next to the
pricing path.

## Outcome

The gate binds to the **contracted configuration**, and the configuration has a
single definition. `shapeSchema` declares every key of each product's closed
schema with its type and default; `resolve` validates and defaults once; `Price`
computes cents from the resolved form, `Canonical` renders it as the contract
identity, and `Resolve` exports it so `CreateService` persists the resolved
configuration rather than the raw request map. `allowedShapeKeys` is derived
from the same table, so the allow-list cannot disagree with the schema either.

That structure is the actual fix. Retyping defaults beside `Price` was a live
hazard in both directions: a default that drifts stricter false-refuses a
legitimate create, and one that drifts looser silently reopens the substitution
hole — table-dependent, so it could be either. With one resolver, divergence is
impossible by construction rather than by discipline.

Unpriced fields are in the identity because they are contracted: a customer who
priced `pgmq` off did not agree to it on, however equal the bill. `connections`
and `pgmq` are carried opaquely and compared structurally — canon stores them as
objects, not scalars, which the first implementation got wrong and the canon
tests caught immediately.

The price check is kept alongside the identity check, but now compares the
STORED line — the number that was on the customer's screen. Comparing two
freshly-computed prices, as the first version did, can only ever agree with
itself.

`TestCanonicalCoversEveryDeclaredField` iterates `shapeSchema` rather than a
hard-coded list, so a field declared later and left out of the identity fails
automatically. That is what keeps the class closed rather than these instances.

Evidence: `services/api` 22 packages RC=0 (no `-race` — see Q10),
`apps/cli` 2 RC=0, `services/cell-agent` 5 RC=0 under `-race`; zero failures,
zero skips.

Reproducing it needs a container runtime, or the Postgres-backed tests SKIP and
the suite still prints `ok`:
`DOCKER_HOST=unix://$HOME/.colima/default/docker.sock go test ./...`

## Findings filed (pre-existing, out of this diff)

- **US-3.8** (high) — an instance `override` changes real capacity with no
  reprice and no spend-cap check. Reproduced live: created at 1900¢, override
  `instances: 9` → 200, still 1900¢, desired doc carrying 9. Nine provisioned,
  one billed. The same law this task strengthens, pointing at the update path.
- **T3.4c** (high) — the CNPG driver sizes the PVC from an unschema'd
  `shape["storage"]` key and never reads the priced `storage_gb`, so a customer
  billed for 78 GB receives the size-derived default. The contract is enforced at
  the gate and then discarded by the renderer.

- **Q10** (high) — `TestConcurrentWritebackAppliesOnce` fails under `-race` on a
  data race in its own test doubles. Introduced by US-3.6 (#300) — mine — and
  already on the base branch; `go test -race ./...` is unusable for
  `services/api` until it is fixed.
- **US-3.9** (low) — `connections`/`pgmq` are opaque, so canon's observed
  `connections.used` is inside the contract identity. Harmless today; springs
  the moment anything on the observe path writes a live value into
  `services.shape`. Filed with a precondition on that work.

All four are out of scope here, but each undercuts something this task asserts,
so they are filed rather than noted.

## Behaviour tightening, recorded

`intent` now participates in the match. A client that sends a non-default
`intent` on the estimate but omits it on create (or vice versa) is refused where
it previously succeeded. No current client does this — the CLI builds no intent
and no console path sends one — but a future client regression should be
diagnosable from this note.

Shapes are also now persisted resolved, so `services.shape` carries every
declared field explicitly. Anything reading a shape and distinguishing "absent"
from "default" would see a behaviour change; nothing does today.
