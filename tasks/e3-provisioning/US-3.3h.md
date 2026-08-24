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

**TWO hops are not converged, and the second one was a billing bug I shipped.**
Review caught it: `ready` + `failed` → `degraded` was marked CONVERGED, so
`observed_generation` advanced, the row left the outstanding set, and a cluster
CNPG reports as "unrecoverable and needs manual intervention" would rest at
`degraded` — which BILLS — with nothing ever observing it again. Measured:
`degraded → failed` is the ONLY edge that emits a metering `close`, and
`IsBilling("degraded")` is true. The justification in my own test was
measurably false, too: it said marking the hop unconverged "would keep the row
outstanding forever", when `ObservedStatus("degraded","failed")` is converged and
terminates in two hops — exactly the shape of the path below. The asymmetry was
a defect, not a design.

**The other hop that is not converged.** ADR-024 has `failed → {provisioning,
deleting}` and no `failed → ready`, so a healthy cluster under a failed row moves
to `provisioning` and the NEXT tick lands `ready`. `Observation.Converged()` is
false for exactly that hop, and `Writeback` returns `ErrNotConverged` without
advancing `observed_generation` — so `ListDesiredForCell` keeps returning the row.
This is the same rule Kubernetes states for `status.observedGeneration`: it
advances when a generation is reconciled, not when it was merely looked at.

**An unconverged hop is a 409, not a 500.** `ErrNotConverged` had no arm in
`writeErr`, so it fell to `problem.Internal`: an HTTP 500 the OpenAPI contract
does not declare for that route, an ERROR boundary log, and a remediation telling
the operator to "contact support with the event id" — an id that is never minted.
Every failed-service recovery in the fleet would have emitted a control-plane
5xx. 409 is already in the contract and is already what the agent's own client
doc calls "just another re-poll".

**A type, not a `(string, bool)` pair — and an honest account of what that
buys.** ADR-0014 says the unsafe form must not compile. The type removes the
failure mode that actually happened (a discarded second return value), and the
zero value is inert, so a caller that never consults the machine does nothing
rather than something wrong. It does NOT force a caller to ask: nothing in Go
makes `Converged()` mandatory. That half is held by a test at the call site, and
`Writeback ignoring Converged()` is measured RED. Claiming full compile-time
enforcement here would be false.

**Three arms deleted as equivalent mutants.** `observed == ""`, `from == ""`, and
an unknown `from` were written as separate cases and are all indistinguishable
from the default — measured, by deleting them and finding nothing failed. They
read as three guards while being one; they are named in the default's comment and
pinned by BEHAVIOUR instead.

**Eight mutations RED** on a green baseline asserted before and after: the
ready+failed→degraded arm, failed+ready marked converged, the suspended arm,
the default echoing the observation, `observed == from` producing an edge,
`Writeback` using the raw report, `Writeback` ignoring `Converged()`, and
`Converged()` hardcoded true.

*Method note: my first sweep restored the tree only BEFORE each mutation, so the
"green after" assertion measured the last mutation rather than a clean tree. It
reported `FINAL: 4`. Re-run with a restore before the final check.*

**Where the type lives.** `reconcile` imports `provisioning` for the value type.
The Transitioner interface exists so this package never reimplements the legal
edges, the spine events or the metering rule — not to avoid naming a struct — and
Go has no covariant returns, so a mirrored type here cannot satisfy a method
returning the real one. The alternative, `provisioning` importing `reconcile`,
points the domain at its own caller.
