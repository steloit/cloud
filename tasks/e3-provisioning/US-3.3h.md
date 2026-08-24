---
id: US-3.3h
title: "The agent can report a status illegal from the service's current one, and the writeback 409s forever"
epic: E3
status: done
phase: MVP
priority: high
sprint: 4
issue: 0
labels: [Platform, Backend]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/api/internal/reconcile/**
  - services/api/internal/provisioning/services.go
  - services/api/internal/provisioning/services_test.go
  - services/cell-agent/internal/render/**
  - tasks/e3-provisioning/US-3.3h.md
verify:
  - "a service in every from-state, observed in every phase, reaches a status the machine accepts"
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go test -count=1 -race ./internal/reconcile/ ./internal/provisioning/"
owner: agent
---

## The defect

`statusFromPhase` reads only the CNPG phase, so it answers identically whatever
state the row is in — while the writeback asks *"is this edge legal from
`svc.Status`"*. ADR-024 allows `ready → {degraded, suspended, deleting}`, so a
cluster that breaks while READY makes the agent report `failed`, `Transition`
rejects it, `observed_generation` never advances, the row stays outstanding, and
it is retried forever with nothing visible to the customer. `failed → ready` is
the same defect on the recovery path.

Reachable through the ordinary flow: `UpdateServiceShape` bumps the generation
for any status except `deleting`, and `ListDesiredForCell` has no status filter,
so a PATCH on a READY service hands the agent a ready row.

## Where the fix goes — NOT in the agent

US-3.3a round 12 put a `statusFor(from, want)` and a copy of the transition table
in the cell-agent, with a repo-root JSON artifact to keep the two copies honest.
**That was reverted**, for three measured reasons:

1. **It collapsed the transient guard.** `statusFor("ready", "provisioning")`
   finds no legal edge and returns `from` — and `ready` IS terminal, so
   Converge's `!terminal(status)` guard never fired. Measured: a READY service
   observed in `Upgrading cluster` / `Switchover in progress` / `Failing over`
   reported `"ready", nil` instead of `ErrNotConverged`. The agent declares a
   generation converged in the middle of an apply, `MarkObserved` advances, the
   row leaves the outstanding set, and if the upgrade then fails nothing ever
   observes it. That is strictly worse than the 409 loop it replaced.
2. **The premise for the JSON artifact was false.** "Separate go.mod files, so
   neither module can import the other" — `apps/cli/go.mod` already imports
   `github.com/steloit/cloud/packages/contracts/go` across exactly that boundary
   with a `replace`, and `docs/architecture.md` says the cell-agent does too.
3. **A data-plane copy of a control-plane state machine is a plane leak**
   (ADR-0001 D9/A2.5: the cell owns ACTUAL, the control plane owns DESIRED and
   status policy) — and it left THREE copies pinned by two different loaders,
   one of which was bypassable by a decoy `testdata/` in the package directory.

So: the agent reports what it OBSERVES (`statusFromPhase`, unchanged, with the
transient guard intact), and `reconcile.Writeback` maps the observation onto a
legal edge, because it is the only place that has both `from` and the machine.

## Design

Next to `transitions` in `provisioning`:

```go
// Observation is what the machine decided to do with a cell's report. It is a
// TYPE, not a (string, bool) pair, because ADR-0014 binds here: `to, _ :=
// ObservedStatus(...)` compiles, and dropping that bool silently re-introduces
// the advance-observed-while-unsettled bug round 12 was reverted for. Writeback
// must be unable to advance observation without having consulted it.
type Observation struct{ /* unexported */ }

func ObservedStatus(from, observed string) Observation

func (o Observation) Edge() (to string, ok bool) // ok=false: no transition
func (o Observation) Converged() bool            // false: do NOT advance observed_generation
```

The unsafe form must not compile — that is ADR-0014's rule, ratified by the
founder 2026-08-23, and it is the whole reason this task exists rather than a
patch to the reverted design.

| from | observed | → | why |
|---|---|---|---|
| any | same | same, settled | the cell observed the state we are in |
| any | a legal edge | observed, settled | ordinary case |
| `ready` | `failed` | `degraded`, settled | legal AND semantically right |
| `failed` | `ready` | `provisioning`, **unsettled** | the documented retry path; the next tick lands `ready` |
| `suspended` | `ready` | `suspended`, settled | **never auto-resume** — see below |
| anything else | — | `from`, settled | no legal edge: report no change |
| any | `""` | no edge, settled | **a live input at this layer**: `reconcile/http.go` normalizes the wire's `gone` to `""`. Today `Writeback`'s existing `rep.Status != ""` guard keeps the mapping off the teardown path, but AC 2's cross product is over phases and pins nothing here. |

Reached through `reconcile.Transitioner` (extended by one method) so `reconcile`
still does not import `provisioning`.

Traced before proposing: on the unsettled `failed`+healthy hop, `Transition`
(failed → provisioning) is legal, `MarkObserved` is skipped, so
`observed_generation < generation` holds, `ListDesiredForCell` still returns the
row and the next tick lands `ready`. The row is not stranded, and returning an
error is the same shape as the existing failed-`Transition` path the code already
documents.

## Acceptance criteria

1. [n/a — belongs to the AGENT, and is already correct there] A transient phase
   never produces a terminal status. The revert restored `statusFromPhase` +
   the `!terminal(status)` guard, which handles this at the only layer that sees
   a phase. This task moved the FROM-awareness to the control plane, which never
   sees a phase at all — so there is nothing here to get wrong.
2. Every (from-state × phase) pair reaches a status `Transition` accepts, or
   `ErrNotConverged`, or a real error — asserted as a cross product.
3. `ready` + a terminal-bad phase lands `degraded` — asserted by DESTINATION.
   A legality-only sweep cannot see this: it skips any answer equal to `from`,
   so "no change" (a broken database reported healthy forever) passes it.
4. `failed` + healthy converges to `ready` across two ticks, and
   `observed_generation` does not advance on the first.
5. **A suspended service is never reported `ready`.** `suspended → ready` is a
   legal edge, so the agent would silently un-suspend on the next converge and
   restart the metering span. Nothing drives a service to `suspended` today,
   which is why this is an AC and not an incident.
6. `statusFor`-style logic does NOT reappear in the cell-agent, and no copy of
   `transitions` exists outside `services/api`.
7. `observed == ""` is handled explicitly, not by accident of a caller's guard.
8. Mutation-verified on a GREEN-baseline harness, including: each table row
   individually, the transient collapse, and — since ADR-0014 says the unsafe
   form must not compile — a check that ignoring the convergence signal is a
   COMPILE error rather than a surviving mutation.

## Read first

- `services/api/internal/reconcile/reconcile.go` (`Writeback`, and the comment
  on the deliberate Transition-then-MarkObserved order)
- `services/api/internal/provisioning/services.go` (`transitions`, `CanTransition`)
- `services/cell-agent/internal/render/cnpg_renderer.go` (`statusFromPhase`, and
  the BLOCKER comment on never reporting a transient)
- US-3.3a's round-12 revert (this task's reason for existing)

## Outcome

`ObservedStatus(from, observed) Observation` lives next to `transitions` in
`provisioning`, and `reconcile.Writeback` maps every report through it. The agent
is untouched: it reports what it OBSERVES, the control plane decides what that
means, and the data plane holds no copy of the machine (ADR-0001 D9/A2.5).

**Round 3 replaced four hand-written flags with one derived rule.** Both
reviewers, independently, found the same three defects, and each was a hop where
a hand-written `converged:` literal was wrong or missing. That is a class, not
three bugs, so the flag is no longer written per hop:

> a generation is reconciled only when the row comes to rest on a **settled**
> status (not `provisioning`, not `degraded`) that the cell **actually reported**.

Both halves are load-bearing, and together they subsume every special case the
function used to carry — including the two hops round 2 had already fixed by
hand. There is no per-hop flag left to get wrong.

### What that fixed

- **A cell could delete or suspend any service in its cells.** `statusVocab`
  admits both (it mirrors the customer-facing enum) and `CanTransition` accepts
  both from `ready`, so one POST with the reconciler token — a single shared
  secret scoped by a cell list — moved a service to the terminal `deleting`.
  `SetServiceStatus` does not bump the generation, so no `deleting:true` desired
  doc is produced and no teardown runs; `deleting` has no outgoing edge; and
  `DeleteService` then answers "deletion already in progress" forever, metering
  span closed, workload still running. Refused in **two** representations now:
  `ReportableByCell` gates the mapping, and the HTTP handler 422s the report
  before it reaches the store. Settling it would have been the quieter attack —
  advancing `observed_generation` drops a row out of `ListDesiredForCell` for
  good — so a refused report also stays outstanding.
- **`ready` + `degraded` rested on a billing state in one hop**, while round 2
  had correctly made the two-hop `ready` + `failed` path stay outstanding. Same
  guard, same harm: `degraded` bills, `degraded → failed` is the only edge that
  emits a metering `close`, and a row parked there unwatched bills indefinitely.
- **A transient finished a generation mid-apply.** `ready` + `provisioning` and
  `provisioning` + `provisioning` both settled, which meant the control plane was
  relying on the agent's own `terminal()` guard — a data-plane dependency this
  task's premise says it must not have.
- **An unplaceable report settled at a stale status, silently.**
  `provisioning` + `degraded` has no legal edge; it used to answer "no change,
  converged", advancing observation and dropping the row out of the outstanding
  set with no error and nothing visible — the mirror image of the 409 loop this
  task exists to remove. `ErrNotConverged` now names both sides, which is the
  only trace such a report leaves.

### Round 4: the guard had a second face, and it failed open

QA's second pass found four blocking gaps. The worst is **the same defect round 3
fixed, in the other representation.**

Round 3 stopped a cell *asking* for a delete — `deleting`/`suspended` in the
**observed** position, refused in the mapping and 422'd at the route. It did
nothing about the **from** position, and 13 of the 16 held-row pairs were pinned
by nothing. `DeleteService` bumps the generation and only *then* transitions to
`deleting`, so a deleting row is outstanding **by construction** — that is what
redelivers the `deleting:true` desired doc. One plain `ready` report from a cell
that had torn nothing down finished the generation: the row left
`ListDesiredForCell`, the desired doc was never redelivered, the cluster kept
running, and `DeleteService` answered "deletion already in progress" forever.
Measured through the real route: **HTTP 200, `observed_generation` 6→7.**

A held row now converges only on `gone` — evidence the hold was *applied*. Step 2
stops a cell asking for a delete; step 1 stops it **abandoning** one.

**The sweeps were jointly vacuous.** Every convergence invariant was guarded by
`if !o.Converged() { continue }`, so all of them could only punish converging too
*eagerly*. Measured: replacing the whole body of `ObservedStatus` with
`return Observation{}` — a mapping that does nothing for all 72 pairs — left
**all five sweeps green**. Under-convergence is the *original* defect of this
task, so the sweep was blind to half the harm it claimed. The positive half is
now its own sweep; the inert mapping fails 26 tests.

**The concurrency test did not race — and my first fix made it worse.** Adding
the fixture barrier forced every caller to read the same status, so all computed
the same destination and the fake's "already there" arm rejected the duplicates:
detection of a deleted FROM-guard went from 3/20 to **0/10**. The window exists
only when callers compute *different* destinations. Staged deterministically now
(A reads `failed` and is held; B advances to `provisioning`; C finishes to
`ready`; A is released still holding its stale read) — **10/10**. Both shapes are
kept, for different properties.

**Totality was a count, not coverage.** `len(everyPair()) == 8*8` stayed green
with the whole `suspended` row, the whole `suspended` column and the empty-`from`
row replaced by junk — the same defect as the `if checked < 30` floor round 3
deleted. It asserts *membership* of the vocabulary now.

Also: the ADR-024 vocabulary was retyped in **four** places with nothing tying
them, so a status added to the machine would have been swept by none.
`provisioning.StatusVocabulary` is the one definition and the rest derive from
it. `settledStatuses`' `suspended`/`deleting` entries were dead and are gone.

### A defect the invariant found that no reviewer did

Writing the harm as a sweep (*a converged row must not rest on a state still
being watched*) immediately failed on a path nobody had flagged: **`gone` on a
live service**. The handler was normalising the wire's `gone` into `""`, which
destroyed the difference between *"the thing you asked me to run does not exist"*
and *"I applied this generation and have nothing to say about its status"*. Both
converged. So a workload that vanished while desired still wanted it alive
advanced `observed_generation`, left `ListDesiredForCell` permanently, kept
showing the customer `ready`, and kept billing a `ready` span — and the agent,
which only ever sees outstanding rows, would never re-create it.

The existing test called an agent reporting `gone` for a live service "a bug" and
asserted the status was not mutated. It was right about both; it never asked what
happened to the row afterwards. `gone` is passed through unnormalised now and the
machine answers the two meanings separately.

### Evidence

**12 mutations RED**, on a baseline asserted green **before and after** — in a
`cp -R` sandbox, scaffolded with `docs/dev/` because a module-only copy is red on
arrival (this bit again: the first sweep reported a red baseline until
`money-range-audit.md` was copied in).

| mutation | |
|---|---|
| the mapping accepts a lifecycle report | RED |
| the HTTP handler accepts one | RED |
| `degraded` marked settled | RED |
| `provisioning` marked settled | RED |
| drop the "cell actually reported it" half | RED |
| `gone` converges | RED |
| drop the `suspended` arm (auto-resume) | RED |
| drop the `deleting` arm | RED |
| `Converged()` hardcoded true | RED |
| `Writeback` ignores `Converged()` | RED |
| `Writeback` uses the RAW report | RED |
| `ErrNotConverged` falls through to 500 | RED |

Two things the sweep could not have told me, both found by running the tests
rather than reading them:

- **My own integration assertion was answered by something else.** It checked
  `observed_generation != 1` after the degraded hop — but the previous hop had
  already left it at 1, so it could not distinguish "did not advance" from
  "already there". The scenario was also unreachable as written: a `ready` row
  with `observed == generation` is not outstanding, so no agent would ever poll
  it. Fixed by bumping the generation first, which is the reachable path the task
  describes (a PATCH on a READY service).
- **The unit fake was more permissive than the store.** `fakeTrans` applied an
  edge without the `WHERE status = $2` FROM-guard `SetServiceStatus` enforces, so
  a concurrency test could observe a row walking backwards — a property of the
  fake, not the system. The fake enforces the guard now, and the concurrency test
  asserts the invariant (observation never advances on an unsettled row) instead
  of one interleaving.

The billing claim is measured against real Postgres for the first time: the
two-hop path emits exactly `[open, close]`, the `close` arrives only on
`degraded → failed`, and both edges land on the spine. It is RED when the
degraded hop is made to converge.

```
services/api        go test -count=1 -race ./...   all ok (containers)
services/cell-agent go build ./... && go vet ./... && go test -count=1 -race ./...   ok
gofmt -l services/                                  clean
node scripts/spec-sync/validate.mjs                 OK: 242 tasks
```

### Recorded, not fixed here

- The route's OpenAPI description defines 409 as *"a report whose generation is
  not the one desired holds right now is rejected **rather than applied**"*. The
  unconverged 409 is the opposite — the edge WAS applied, only the generation did
  not advance. The 409 response itself is declared, so this is a description gap,
  not contract drift; `openapi.yaml` is outside this task's `files:` globs.
- `last_reconciled_at` is not stamped on an unconverged hop (`MarkObserved` is
  its only writer). Nothing reads the column today; it matters to whatever
  staleness alerting consumes it.
- A permanently degraded cluster now stays outstanding indefinitely — visible to
  the customer as `degraded`, which the original defect was not. **US-3.11 does
  NOT cover this** (its ACs are about the *agent* distinguishing render errors);
  citing it here was wrong, and the real gap is filed as **US-3.13**.
- The route's OpenAPI **request enum** still advertises `suspended` and
  `deleting`, which the API now refuses, and a heartbeat clause that no longer
  holds. Real contract drift, outside this task's `files:` globs — **US-3.12**.
- `ObservedStatus` is on the `Transitioner` interface as a method that ignores
  its receiver. `Transition` earns the interface; this member does not, and a
  package-level call would be equivalent. Left as-is to keep one owner for "when
  do we consult the machine".
