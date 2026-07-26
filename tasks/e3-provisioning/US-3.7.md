---
id: US-3.7
title: "The estimate gate matches on PRICE, so a colliding shape provisions something you did not price"
epic: E3
status: in-progress
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

- [ ] Estimating `{size: dev, storage_gb: 78}` and creating `{size: standard}`
  is **refused** with the existing conflict problem + remediation.
- [ ] The same shape spelled with omitted defaults still matches (no false
  refusal for `{size:"dev"}` vs `{size:"dev", ha:false}`).
- [ ] A shape whose price has changed since the estimate was issued is refused.
- [ ] The refusal happens BEFORE the one-shot estimate is burned (the existing
  ordering at the spend-cap check is the precedent).
- [ ] Mutation-verified: reverting the gate to a price-only match fails the new
  test.

## Found by

CK-M3's QA review (2026-07-26). The checkpoint certifies "estimate-gated
provisioning end-to-end" and did not exercise a colliding shape; the collision
was computed from `internal/estimates/pricing.json` and confirmed against the
gate's matching logic.

## Related

US-3.2 (the gate) · CK-M3 (checkpoint that surfaced it) · F2 (estimate-before-
provision law) · ADR-025 (integer cents)
