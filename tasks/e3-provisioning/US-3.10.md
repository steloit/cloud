---
id: US-3.10
title: "The Go and SQL pin-liveness rules disagree, and a disagreeing pin is stranded forever"
epic: E3
status: ready
phase: MVP
priority: medium
sprint: 4
estimate: 0.25ew
deps: []
issue: 0
labels: [Backend]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/api/db/queries/services.sql
  - services/api/internal/identity/store/services.sql.go
  - services/api/internal/provisioning/services.go
  - services/api/internal/identity/services_integration_test.go
  - docs/product/08-api/openapi.yaml
  - tasks/e3-provisioning/US-3.10.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go build ./... && go vet ./... && go test ./..."
owner: agent
---

## The defect

Pin liveness is decided TWICE, by two different parsers, and the SQL comment in
`ListExpiredOverrides` states outright that "the two liveness implementations
must agree". They do not.

- **Go** — `overrideInstances` uses `time.Parse(time.RFC3339, …)`.
- **SQL** — `ListExpiredOverrides` uses a regex plus `pg_input_is_valid(…,
  'timestamptz')`.

When SQL keeps a pin that Go refuses to honour, the row is **stranded**: the API
will not apply it, and the sweep will not clear it. It sits non-NULL forever,
invisible to both halves of the feature.

Three reproducers, verified against a real `postgres:16-alpine` and Go 1.26
(US-3.8 QA sweep, 2026-07-27). All three are "SQL keeps, Go refuses":

| `expires_at` | SQL | Go | why |
|---|---|---|---|
| `2027-08-01T00:00:00+0700` | kept | dead | the regex's `[+-]\d{2}:?\d{2}` makes the colon optional; RFC3339 does not |
| `2027-08-01T24:00:00Z` | kept | dead | Postgres rolls hour 24 to the next day; Go rejects it |
| `2027-08-01T00:00:00-0000` | kept | dead | same optional-colon path, negative zero offset |

## Why it is not a blocker today

**Not reachable through the API.** `services_http.go` stamps `expires_at` itself
on every pin (`time.Now().Add(24h).UTC().Format(time.RFC3339)`); no client
string reaches the column.

It IS reachable by the same route the sweep's own test fixture blesses — direct
writes, which that test's comment describes as "shapes the API would never
write, which is exactly why the sweep has to cope with them". The `spaceform`
case already covered there is precisely such an input. By the file's own
standard these three belong with it. A migration, a support script, or a future
endpoint that accepts a caller-supplied expiry all make it live.

## What to build

Under Option B, a property test, not a table:

    for any expires_at string s at a fixed instant T:
        sqlKeeps(s, T)  ⟺  goLive(s, T)

Drive `ListExpiredOverrides` through `store.New(tx)` inside ONE transaction so
`now()` is fixed and comparable to a `time.Time` read from that same transaction
(the technique `TestAPinExpiringAtExactlyNowIsSweptNotStranded` already uses),
and compare against `overrideInstances` on the same string.

Corpus: valid RFC3339 seeds, plus mutations over the offset form (`Z`, `z`,
`+hh:mm`, `+hhmm`, `-0000`), hour/minute/second out of range, fractional-second
widths 0–9, leap seconds, year `0000` and `99999`, space separator, lowercase
`t`, and empty/absent.

**It must fail on the three strings above before the fix**, so the ticket
reproduces itself.

## Fix direction — DELETION FIRST, and read this before tightening anything

**Option A (preferred): delete the SQL liveness predicate entirely.**

`ListExpiredOverrides` becomes `WHERE override IS NOT NULL AND status <>
'deleting'` — already index-bounded by `services_override_idx`, on the same
premise that index rests on (pins are rare and short-lived by construction) —
and the sweep filters liveness in Go with `overrideInstances`, which already
owns that decision. There is then ONE implementation and nothing to disagree.

What this retires, all together: the regex arm, the `pg_input_is_valid` arm, the
malformed-row batch-abort hazard, **the PostgreSQL 16 floor and `db.Connect`'s
version probe with its two tests**, the CNPG `imageName` pin, the
exactly-at-expiry SQL boundary test — and this task itself.

**Do not count the batch-resilience argument on the "keep" side.** It is
circular: the malformed-row abort exists ONLY because the SQL casts
`expires_at` to `timestamptz`. Under the Go filter there is no cast in the
query, so there is no batch to make resilient. The resilience is a COST of the
SQL predicate, not a benefit of it. (US-3.8 architecture review, correcting the
implementer's own stated reason for deferring.)

**Weigh honestly, though:** dropping the PG16 floor is easy and raising one
later is not. That predicate cost a database version requirement *and* produced
three of US-3.8's recorded mistakes — which makes the case stronger than "retire
the disagreement", but the floor is still a deliberate decision to give up, not
a benefit to bank.

**Deletion MOVES risk rather than removing it.** Three things need tests
afterwards, none of them string-parsing agreement (US-3.8 QA review):

1. **A bad row must not stop the sweep.** This is the strongest argument FOR
   deletion — today one malformed `expires_at` can abort the whole statement,
   silently, for every customer; a Go parse error is per-row. But
   `expireOverride`'s loop could still `return` where it should `continue`, so
   `TestTheSweepClearsEveryDeadPinShapeAndSurvivesAMalformedOne` needs
   RETARGETING, not deleting.
2. **A live pin must cost no per-row work.** `WHERE override IS NOT NULL` lists
   every pinned service, not every expired one, and `expireOverride` does three
   round trips before the clear. If the liveness filter does not run BEFORE
   those, the sweep does N× the work every five minutes — the "permanent
   busy-work with no symptom" `TestTheSweepLeavesADeletingServiceAlone` exists
   to prevent.
3. **Whose clock decides.** Genuinely new, and neither design has a test for it.
   Today the sweep defers to Postgres's `now()`. Afterwards the Go process's
   `time.Now()` is authoritative — better in that there is one clock, but
   skewed API replicas can then disagree about liveness where they previously
   all deferred to the database.

**Option B (only if A is rejected): tighten the SQL regex** to require the
offset colon and reject hour 24, and add the property test below. This makes the
two-implementation split PERMANENT. It is the smaller diff and the worse
outcome, and it is recorded second deliberately — an earlier version of this
task named it first, which would have had an implementer close off Option A
cheaply and correctly without ever seeing it.

## The property test (Option B only)

WITHDRAWN under Option A: a biconditional with one side deleted asserts that a
thing agrees with itself, which can only pass. Insurance against an impossible
disagreement is worse than no test, because it reports coverage.

Under Option B, the acceptance is the biconditional:

## Also in scope: the contract does not advertise the bounds

`openapi.yaml`'s `override.instances` is `{type: integer}` with no `minimum` and
no `maximum`, while the implementation now refuses `< 1` at the handler and
refuses counts too large to price exactly in the engine. Generated clients and
the console therefore cannot know the bound and will discover it as a 422.

Adding `minimum: 1` (and a maximum consistent with the engine's bound) is an
owner-level change to the design authority, which is why US-3.8 recorded it
rather than editing the spec — but it belongs with this task, since both are
"the two liveness/validity rules must agree" in different guises.

## Found by

US-3.8's final QA sweep (2026-07-27), as a FUZZ item that arrived with its own
counterexamples.
