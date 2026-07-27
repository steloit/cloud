---
id: US-3.8
title: "An instance override changes real capacity with no reprice and no budget check"
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
  - services/api/internal/identity/services_integration_test.go
  - tasks/e3-provisioning/US-3.8.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go build ./... && go vet ./... && go test ./..."
owner: agent
---

## The defect

`UpdateService` sets `params.Override` OUTSIDE the `if shape != nil` reprice
branch, so an override is applied without repricing and without the hard spend
cap that every other capacity change must pass. The cell renders exactly that
count (`cell-agent/internal/render/cnpg_renderer.go` reads the override).

Reproduced during US-3.7's review, against real Postgres:

```
create postgres           → monthly_estimate_cents 1900
PATCH {"override":{"instances":9,"reason":"load test"}} → 200
                          → monthly_estimate_cents STILL 1900
                          → desired doc carries instances: 9
```

Nine instances provisioned, one instance billed.

## Why it matters

It is the same law US-3.7 just strengthened, pointing at the update path: what
runs must be what was priced. A shape change reprices and re-checks the cap
(services.go, the scale-up branch); an override does neither, so the cap is
bypassable by anyone who can PATCH a service — the exact bypass the scale-up
comment says it exists to prevent.

## Founder ruling (2026-07-27)

**Pinned capacity is METERED.** The pin is repriced through the engine, the
increase clears the same hard cap a scale-up does, and the row carries the
pinned rate — which is what the billing span is opened at.

**The missing 24h expiry stays in scope**, because it is what made the pin
permanent, and a permanent pin is what made "nine provisioned, one billed"
unbounded rather than a 24h ceiling.

## A second defect, found implementing this

D22 requires the pin to auto-expire in 24h, and the API's own error message
promises the customer it does. **`expires_at` was written and never read.**
Nothing consulted it — not the renderer's `instancesOf`, not any sweep, not any
query. The only `expires_at` enforcement in the codebase is for tokens,
sessions and environments.

So the "temporary" pin was permanent. That removed the only argument for not
charging for it, and it is why the two halves had to land together.

## Ratified principle (founder, 2026-07-27)

> 1. Never provision what cannot be billed correctly.
> 2. Never invent commercial pricing in implementation code.
> 3. Restore operator affordances only after the pricing model exists.
>
> The temporary loss of an operator feature is acceptable. Silent commercial
> assumptions are not.

**Consequence, accepted:** `postgres` and `valkey` have no priced instance
count, so pins on them are REFUSED (422 naming `override.instances`) rather than
provisioned unbillable. This removes a shipped affordance for the primary
product — two existing tests pinned postgres and now assert the refusal.
Re-enabled by **US-11.9**, which is founder-owned because the rate is a
commercial decision.

Explicitly rejected: reusing postgres's `ha` rate (1900¢) as a proxy for one
replica. Nothing evidences that `ha` prices an additional replica rather than
the HA capability as a whole, so borrowing it would silently change pricing.

## Acceptance criteria

- [x] An override that raises capacity **reprices** through the engine and clears
  the same hard cap a scale-up does — `TestManualOverrideRespectsTheCapAndExpires`
  refuses a 9-instance pin under a cap that does not cover it, then applies it
  once the cap is lifted. Mutation-verified.
- [x] The billed amount and the rendered capacity agree: the row carries the
  pinned rate while the pin is live, and the base rate is restored on expiry.
- [x] A pin the catalog cannot price is **refused**, never provisioned —
  `TestUnpriceablePinIsRefused` (422 naming `override.instances`).
- [x] The 24h expiry is real: `ExpireManualOverrides` clears the pin, bumps
  generation so the cell re-polls, rebuilds desired without it, and restores the
  unpinned price. Mutation-verified in both halves (the sweep never matching,
  and the flag never being set).
- [x] An expired pin is never shipped to the cell even before the sweep runs.
- [x] A pin with NO expiry is not honoured — "unset" must not mean "forever".

## Found by

US-3.7's architecture review (2026-07-26), reproduced live.

## Related

US-3.7 (the same law on the create path) · T11.6 (the hard spend cap) · D10

## Outcome

Two defects, and the second is why they had to land together.

**The filed defect:** `params.Override` was set outside the reprice branch, so a
pin provisioned real capacity with no reprice and no spend-cap check. Verified
live at filing: created at 1900¢, `instances: 9` → 200, still 1900¢, desired doc
carrying 9. Nine provisioned, one billed.

**The defect found implementing it:** D22 requires the pin to auto-expire in 24h,
and the API's own validation message promises the customer it does.
`expires_at` was written and never read — not by the renderer's `instancesOf`,
not by any sweep or query. The only `expires_at` enforcement in the codebase is
for tokens, sessions and environments. So every pin ever created was permanent,
which is what made the billing gap unbounded rather than capped at a day, and
what removed the only argument for not charging for it.

Both are closed. A pin now reprices through the engine, clears the cap, meters
at the pinned rate, and expires — with the sweep bumping generation so the cell
converges back, because filtering on read alone would leave a converged service
rendering its pin forever.

The founder ruling that pinned capacity is metered had a consequence I could not
resolve without a pricing decision: postgres and valkey have no priced instance
count. Refusing those pins (rather than deriving a rate from `ha`) is ratified,
with US-11.9 filed to restore them once a rate exists.

Two test fixtures asserted on states the API cannot produce: an override with no
`expires_at`, and a postgres pin. Both corrected — the same class as US-3.7's
`intent: "transactional"`.

Evidence: `services/api` 22 packages RC=0 (no `-race` — see Q10), `apps/cli` 2
RC=0, `services/cell-agent` 5 RC=0 under `-race`; zero failures, zero skips.
