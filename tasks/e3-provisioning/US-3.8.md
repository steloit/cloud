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
  - contexts/provisioning.md
  - infra/k8s/control-plane/cnpg-cluster.yaml
  - services/api/cmd/api/main.go
  - services/api/db/queries/services.sql
  - services/api/internal/estimates/engine.go
  - services/api/internal/estimates/engine_test.go
  - services/api/internal/identity/services_integration_test.go
  - services/api/internal/identity/store/**
  - services/api/internal/metering/metering.go
  - services/api/internal/platform/db/db.go
  - services/api/internal/platform/db/db_integration_test.go
  - services/api/internal/platform/db/migrations/**
  - services/api/internal/provisioning/services.go
  - services/api/internal/provisioning/services_http.go
  - services/api/internal/provisioning/services_test.go
  - services/api/internal/reconcile/wiring_integration_test.go
  - tasks/e11-billing/US-11.9.md
  - tasks/e10-observability/US-10.7.md
  - tasks/e3-provisioning/US-3.10.md
  - tasks/e3-provisioning/US-3.8.md
  - tasks/eops/O13.md
  - tasks/eops/O14.md
  - tasks/eops/O15.md
  - tasks/eops/O16.md
  - tasks/eops/O17.md
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
- [x] The 24h expiry is real: `RunOverrideExpiry` clears the pin, bumps
  generation so the cell re-polls, rebuilds desired without it, and restores the
  unpinned price. Mutation-verified in both halves (the sweep never matching,
  and the flag never being set).
- [x] An expired pin is never shipped to the cell even before the sweep runs —
  `TestTheDesiredDocNeverCarriesADeadPin`. This was previously ticked on the
  strength of the guard existing: QA showed the guard could be deleted with the
  suite green, and the cell's renderer never consults `expires_at`, so that one
  line is all that stands between a dead pin and provisioned capacity.
- [x] A pin with NO expiry is not honoured — `TestOverrideLiveness`, a
  table over all five liveness branches. Also previously ticked without a test:
  nothing executed `overrideInstances` at all.
- [x] The sweep clears EVERY shape of dead pin — absent, garbage, regex-passing
  but cast-invalid, and Postgres-parseable-but-not-RFC3339 — and one malformed
  row does not abort the batch
  (`TestTheSweepClearsEveryDeadPinShapeAndSurvivesAMalformedOne`).
- [x] US-3.6's invariant survives this new code path: a reprice before `ready`
  emits no span (`TestARepriceBeforeReadyEmitsNoSpan`). A PATCH now writes the
  price on every edit, so without the `IsBilling` guard it would open a span on
  a service that never ran.

## Found by

US-3.7's architecture review (2026-07-26), reproduced live.

## Related

US-3.7 (the same law on the create path) · T11.6 (the hard spend cap) · D10

## What this task cost, and why

The feature was largely correct after the first pass: the founder ruling was
implemented, and no review round changed the product behaviour that was asked
for. Three architecture rounds and two QA rounds were spent almost entirely on
**restoring evidence that the invariants still held after each refactor** —
guards that existed and worked until a boundary moved under them, and
acceptance criteria that had been ticked because the code was there rather than
because anything proved it.

Two criteria in this file were marked done with no test executing the function
they rested on. That is the failure worth remembering: a green gate plus present
code read as proof, and it was not.

The final QA round found the same failure one level deeper, in the fixes for it.
The test written to close the desired-doc guard **re-implemented that guard in
its own body**, so deleting the production line left it green. And the sweep
predicate's cast guard (`pg_input_is_valid(…) AND (…)::timestamptz`) was dead
code: deleting it changed nothing, because an earlier OR arm was already
`OR pg_input_is_valid(…) = false` — the same guard written twice, with OR's
left-to-right short-circuit shielding the cast. Both were "evidence" that
described the property without depending on it. The diagnostic that catches this
class is one question: *if I delete the line this test is named after, does the
test fail?* Answering it requires running the deletion, which is why mutation
testing found what three reviews of the code did not. Both are now closed by
construction — the doc test drives `UpdateService` and reads the stored column,
and the predicate is one ordered `CASE`, the only SQL construct whose evaluation
order Postgres guarantees.

Two of QA's survivors were also instructive in opposite directions. One is an
**equivalent mutant** — deleting the `ExpiresAt == ""` check falls through to a
parse failure with identical behaviour, so no test can distinguish it and
claiming coverage would be false. The other looked equivalent and was not:
dropping the sweep's regex arm seemed subsumed by `pg_input_is_valid`, but a
future-dated space-separated timestamp is parseable by Postgres and rejected by
Go's RFC3339, so the API would refuse to honour a pin the sweep would never
clear. Two implementations of one predicate, and the guard keeping them in
agreement looked redundant until checked.

A third survivor was recorded rather than closed, and that was WRONG. I claimed
the sweep's `<=` vs `<` at the exact expiry instant was unkillable because
`now()` moves on before any assertion runs. That holds for the sweep, which runs
its SELECT in its own transaction — but not for a test that controls the
transaction, where `now()` is fixed for its whole duration. QA disagreed and was
right; `TestAPinExpiringAtExactlyNowIsSweptNotStranded` now kills it from both
sides of the window. The lesson is not "check harder" — it is that "no test can
distinguish these" is a claim about tests, and I had only checked the tests that
already existed. Similarly, two checks in
`overrideInstances` are equivalent mutants kept for legibility — with a note
that the equivalence is conditional on the parser staying strict, since mutation
testing will not re-flag something already filed as equivalent.

## Recorded decisions and consequences

**Every PATCH now reprices from the current catalog.** Previously a scaling-only
edit preserved the stored price. This is what makes un-pinning correct — the
price follows the effective configuration, whatever the edit touched — and it is
consistent with US-3.7's direction that the configuration is the contract and
the price is derived from it. Recorded as a decision rather than left as a side
effect of the restructure.

**The control DB floor is now PostgreSQL 16**, for `pg_input_is_valid` in the
expiry predicate. Stated in the query itself; on 15 the sweep would fail every
tick with only a log line as the symptom.

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

Evidence: `services/api` 23 packages, 0 failures, 0 skips (serial, `-p 1 -timeout 30m`); cell-agent 5; CLI 2. No `-race` — see O14, which owns a pre-existing fixture race that fails under it.
