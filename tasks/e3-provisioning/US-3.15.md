---
id: US-3.15
title: "A shape PATCH reprices and commits in one call — no estimate, no breakdown, no consent"
epic: E3
status: ready
phase: MVP
priority: high
sprint: 3
estimate: 1ew
deps: []
issue: 0
labels: [Backend, API, Billing]
module: M3 Provision
contexts: [provisioning, billing, api-conventions]
files:
  - services/api/internal/provisioning/**
  - services/api/internal/estimates/**
  - docs/product/08-api/openapi.yaml
  - tasks/e3-provisioning/US-3.15.md
verify:
  - cd services/api && go test -count=1 -race ./...
  - node scripts/spec-sync/validate.mjs
owner: agent
---

## Goal

`PATCH /services/{id}` changes what a customer pays and commits the change in the
same call. Give the repricing half the same gate `createService` has.

## Why

`createService` requires an accepted `estimate_id` — nothing provisions or bills
before acceptance, and CLAUDE.md makes that an **API-layer** rule, not a client
one. `UpdateService` has no equivalent: `services_http.go`'s handler takes
`shape`, `scaling` and `override`, calls `h.svc.UpdateService`, and returns 200.
The only money-shaped check on the path is the org's hard spend cap.

T3.4c's founder ruling makes this concrete rather than theoretical. Pricing the
**effective** configuration is correct — but it means a `standard`→`dev` PATCH,
which the customer reads as a downgrade, is `1900 + 50×50¢ = 4400¢` where the
smaller size alone would suggest 1900¢. The arithmetic is ruled and right. What
is missing is that the customer sees the number **before** the row is written.

This was named in `docs/founder-config.md` as half of option (a) — *"keep the
arithmetic **and surface a breakdown + consent before the PATCH commits**"* —
and the founder's ruling of 2026-08-25 spoke only to the arithmetic. So the
consent half is neither ruled out nor built; it was simply un-owned until now.

## Acceptance criteria

1. A shape PATCH whose effective price differs from the current price is
   **refused** without an accepted estimate, with problem+json and a
   `remediation` naming the estimate endpoint.
2. The estimate for a PATCH prices the **merged, resolved** shape — the same
   value `UpdateService` would commit — so the accepted number and the charged
   number cannot differ. Assert them equal in one test rather than asserting each
   against a literal.
3. A PATCH that does not move the price (scaling metadata, an override, a
   no-op shape) does **not** require an estimate. A gate that fires on every
   PATCH would make `override` unusable.
4. The estimate response carries the **breakdown**, not just a total: base and
   storage as separate lines, so a rise on a nominal downgrade is legible rather
   than surprising. The line grammar is the invoice's line grammar, verbatim.
5. Deleting the gate makes a test fail. Prove it with a mutation, not an
   assertion that the gate exists.

## Notes

Found by the T3.4c review. The finding is not that the ruling is wrong — it is
that recording the ruling closed a row that had *two* halves in it, and only one
was ruled. See `docs/founder-config.md` §5 and T3.4c's Outcome.

Open question for the implementer, not for the founder: whether the estimate a
PATCH accepts is the same `estimates` row type `createService` uses, or a
distinct kind scoped to a service. Prefer the same type if it can carry a
subject; a second estimate kind is a second grammar.
