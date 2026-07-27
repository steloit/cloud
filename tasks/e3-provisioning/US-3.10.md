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

A property test, not a table — the acceptance is the biconditional:

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

## Fix direction

Prefer making SQL agree with Go rather than the reverse: Go's rule is the one
the API enforces and the one the customer's pin was accepted under. Tightening
the regex to require the colon and reject hour 24 is the smaller change; the
alternative — relaxing Go — would start honouring pins the API never promised.

Whichever way it goes, the two rules should stop being two rules. Consider
whether the liveness decision can live in one place that both sides call.

## Found by

US-3.8's final QA sweep (2026-07-27), as a FUZZ item that arrived with its own
counterexamples.
