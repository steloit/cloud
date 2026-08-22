---
id: US-3.6
title: Failed provisioning never bills and never strands state
epic: E3
status: done
phase: MVP
priority: critical
sprint: 4
estimate: 0.5ew
deps: [S7, US-3.3]
issue: 60
labels: [Backend, Billing]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files:
  - services/api/cmd/api/main.go
  - services/api/internal/httpapi/chain.go
  - services/api/internal/httpapi/chain_test.go
  - services/api/internal/identity/identity_integration_test.go
  - services/api/internal/identity/store/models.go
  - services/api/internal/platform/idempotency/**
  - services/api/internal/platform/db/migrations/*_idempotency.*.sql
  - services/api/db/queries/idempotency.sql
  - services/api/internal/identity/store/idempotency.sql.go
  - services/api/internal/identity/http.go
  - services/api/internal/provisioning/services.go
  - services/api/internal/reconcile/**
  - services/cell-agent/internal/render/**
  - tasks/e3-provisioning/US-3.6.md
  - tasks/e3-provisioning/US-3.6a.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go build ./... && go vet ./... && go test ./..."
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go build ./... && go vet ./... && go test ./..."
owner: agent
---

## Goal

A provisioning attempt that fails **never bills** and **never strands state**, and
the client can safely retry (S7 idempotency).

## Three independent guarantees

**1 · Never bills.** Metering opens only on the `ready` edge (D10;
`metering.BillingEdge`). A service that goes `provisioning → failed` must have
ZERO `usage_events`. Largely already true structurally — this task PINS it,
including the failure path and the retry path (a failed→provisioning→ready retry
must open exactly ONE span, not two).

**2 · Never strands state.** A failed provisioning leaves the cell clean: the
reconciler's converge is level-triggered and idempotent, so a retry re-applies
rather than duplicating, and a service abandoned in `failed` that is then deleted
must tear down whatever partial objects exist (US-3.3's teardown deletes every
rendered object, so absence is convergent). Pin: partial apply → failed → retry →
ready leaves exactly one cluster; failed → delete leaves none.

**3 · Retry-safe (S7).** The ratified S7 ruling is UNIMPLEMENTED: there is no
idempotency table and no middleware. Implement it —
- `Idempotency-Key` header (≤255) on mutating POSTs;
- dedupe on **(principal, endpoint, key)** for **24h**;
- a replay returns the ORIGINAL response with `Idempotent-Replay: true`;
- same key + **different body** → **409** problem+json with remediation.
Wire it on `createService` and `createEstimate` first (the estimate-gated
provisioning path this task exists to protect), the shape being reusable for
`signup`/`createWebhook`.

## Why it matters

Without S7, a client that times out mid-`createService` and retries can burn a
second estimate and create a second service — the estimate-before-provision law
(F2) is one-shot, so the retry either double-provisions or wastes the estimate.
"Failed provisioning never bills" is not credible while a retry can double-bill.

## Acceptance criteria

- [x] A service that reaches `failed` has **zero** usage events; a
  failed→retry→ready sequence opens **exactly one** span (real Postgres) —
  `TestFailedProvisioningNeverBills`.
- [x] `Idempotency-Key` replay returns the original response body + status with
  `Idempotent-Replay: true`, and does NOT burn a second estimate or create a
  second service — `TestReplayedCreateServiceDoesNotBurnASecondEstimate` and
  `TestIdempotentEstimateReplaysInsteadOfBurningASecond` (end-to-end through the
  served chain, real Postgres) · `TestReplayReturnsOriginalBytesAndRunsHandlerOnce`.
  Both fail if the layer is unmounted (verified); without dedupe the retry gets
  `409 estimate ... already used`, which is the exact failure this prevents.
- [x] Same key + different body → 409 problem+json carrying `remediation` —
  `TestIdempotentReuseWithDifferentBodyIsRefusedEndToEnd`.
- [x] Keys are scoped to **(principal, endpoint, key)** —
  `TestIdempotencyKeyIsScopedToPrincipalAndEndpoint`; the endpoint is the
  CONCRETE path, so one key on a different `{env}` is not a replay
  (`TestKeyIsScopedToTheConcretePathNotTheRoutePattern`).
- [x] Entries expire after 24h and the key is OWNABLE again —
  `TestIdempotencyKeyIsReusableAfterTheReplayWindow`; a COMPLETED key keeps replaying for the full
  window (`TestIdempotencyCompletedKeyStillReplaysAfterTheAbandonmentWindow`)
  while an ABANDONED claim is re-claimable after 5 minutes
  (`TestIdempotencyAbandonedClaimBecomesReclaimable`).
- [x] A concurrent double-submit with the same key creates exactly one owner —
  `TestIdempotencyConcurrentDoubleSubmitHasOneWinner` (12 goroutines, real
  Postgres); mutation-verified (removing the atomic guard yields 12 owners).
- [x] A late owner cannot release or overwrite the CURRENT owner's claim —
  `TestIdempotencyLateLoserCannotReleaseTheCurrentOwnersClaim` (mutation-verified
  against the `claim_token` fence).
- [x] A panic, a client disconnect, and a concurrent HTTP double-submit each
  resolve to owner-or-replay, never a stranded key or a second execution —
  `TestAPanicReleasesTheClaimSoTheRetryCanRun`,
  `TestCompleteSurvivesAClientDisconnect`,
  `TestConcurrentHTTPDoubleSubmitExecutesOnce` (all mutation-verified).
- [x] Keys are scoped to the authenticated principal at the HTTP layer
  (`user:` / `orgkey:` / `anon:`) — `TestKeyIsScopedToTheAuthenticatedPrincipal`.
- [x] A ready service that later fails CLOSES its span — an unclosed span bills a
  service that is not running (`TestFailureAfterReadyClosesTheSpan`).
- [x] Teardown after failure leaves no stranded objects —
  `TestDeletingAFailedServiceLeavesNothingBehind` (mutation-verified) and
  `TestFailedProvisioningRetryLeavesExactlyOneCluster`.

## Spec conflict (recorded, not resolved — US-3.6a)

`openapi.yaml` declares `idempotencyKey` on FOUR operations and the S7 text says
"every mutating POST". Only `createEstimate` and `createService` are enforced:
`signup` and `createWebhook` return CREDENTIAL material in their 2xx bodies (a
session cookie; the reveal-once webhook signing secret), and S7's "replay the
ORIGINAL response" would mean storing that credential in plaintext for 24h.
A key on those routes PASSES THROUGH unenforced: refusing it would turn a
spec-declared optional header into a hard failure (breaking signup for any
client that sets the header globally), which is a live regression traded for a
warning. The divergence belongs in the contract, which is an owner-level change.
Filed as **US-3.6a**; `deferredRoutes` documents the exclusion inline and the
drift test requires every declared operation to be enforced or explicitly
deferred.

## Out of scope

Idempotency on every mutating POST (the shape lands here; the credential-bearing
routes need a design decision — US-3.6a). The cell's own crash-recovery beyond what
level-triggered converge already guarantees.

## Read first

S7's Outcome (the ratified ruling) · `services/api/internal/metering/metering.go`
(BillingEdge) · `internal/provisioning/services.go` (Transition, CreateService) ·
`docs/product/08-api/openapi.yaml` `components/parameters/idempotencyKey`

## Outcome

S7 idempotency ships as HTTP middleware OUTSIDE the generated strict server: the
ruling is about bytes ("replay the ORIGINAL response"), and a strict middleware
only ever sees a decoded request and a typed response object — it could
reconstruct an equivalent response, never replay the original one. `response` is
`bytea`, not `jsonb`, for the same reason: jsonb re-serializes on read, and the
e2e test caught a replay coming back with reordered keys.

Enforced on `createEstimate` and `createService` only. `signup` and
`createWebhook` DECLARE the parameter in openapi.yaml but return credential
material in their 2xx bodies (a live session cookie; the reveal-once webhook
signing secret), so replaying them means storing a credential for 24h. The
header passes through there unenforced — refusing would turn a spec-declared
optional header into a hard signup failure. Spec conflict recorded, not
resolved: **US-3.6a**.

Review found ten defects that reading alone would have shipped. The two worst:
`Complete` ran on the request context, which `net/http` cancels the instant a
client disconnects — the exact scenario S7 exists for, producing a second
service and a second billing span; and the recorder would have written the
plaintext webhook secret into the database while the same handler
envelope-encrypts it. Also fixed: a panic unwound past `Complete` and stranded
the claim; neither fenced write was tied to its claim, so a stale owner could
delete a successor's; nothing swept the table; and the `ResponseWriter` wrapper
silently stripped `Flusher`/`Hijacker` — then, once fixed, advertised `Hijacker`
over writers that lack it, which is worse than stripping (a handler's fallback
branch becomes a runtime error). Hence two wrapper types.

The endpoint key oscillated between two half-correct fixes before landing:
decoding the whole path COLLAPSES `a%2Fb` and `a%2F%2E%2Fb` into one bucket
(cross-resource replay); the raw escaped path SPLITS `env_1` from `env%5F1`
(a retry that re-encodes double-provisions). It now normalizes per segment as
ServeMux does, and both directions are pinned so it cannot flip back.

Five of the tests written for this task passed while proving nothing — an
assertion true by construction, a `0 == 0` comparison, a single-principal
scoping check, an ordering that made last-write-wins indistinguishable from a
fence, and a decorative billing assertion. All were found by mutation testing,
not by reading. Every guard here now has a mutation that kills it.

Evidence: `services/api` 22 packages RC=0, `services/cell-agent` 5 packages RC=0
under `-race`, zero failures, zero skips. Postgres-backed tests run against real
Postgres via testcontainers.
