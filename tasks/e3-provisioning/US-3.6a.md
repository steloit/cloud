---
id: US-3.6a
title: "Idempotency on credential-bearing responses: signup and createWebhook cannot replay as they stand"
epic: E3
status: done
phase: MVP
priority: high
sprint: 4
deps: [US-3.6]
issue: 0
labels: [Backend, Security]
module: M4 Provisioning
contexts: [api-conventions, provisioning]
files:
  - docs/adr/0013-idempotent-replay-of-credential-bearing-responses.md
  - services/api/cmd/api/main.go
  - services/api/internal/identity/identity_integration_test.go
  - services/api/internal/identity/session/session.go
  - services/api/internal/identity/store/**
  - services/api/internal/reconcile/idempotency_integration_test.go
  - services/api/internal/platform/idempotency/**
  - services/api/db/queries/idempotency.sql
  - services/api/internal/platform/db/migrations/*_idempotency*.sql
  - docs/product/08-api/openapi.yaml
  - tasks/e3-provisioning/US-3.6a.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go build ./... && go vet ./... && go test ./..."
owner: agent
---

## The conflict (found implementing US-3.6 — do not resolve silently)

`openapi.yaml` declares `idempotencyKey` on FOUR operations, and the ratified S7
text (openapi.yaml:23) goes further: "every mutating POST accepts
Idempotency-Key". US-3.6 shipped enforcement on **two** of them
(`createEstimate`, `createService`). The other two cannot be replayed as they
stand, because S7's promise — *return the ORIGINAL response* — means **storing
that response for 24h**:

- **`createWebhook`** returns the reveal-once signing secret in its 201 body
  (`WebhookCreated.secret`). The same handler envelope-encrypts that secret
  before persisting it (`notify.MintSecret` → `Ciphertext`/`WrappedDEK`/`KEKID`).
  Recording the response verbatim would write the **plaintext** secret into
  `idempotency_keys.response`, defeating the reveal-once contract.
- **`signup`** sets the session cookie via the carrier. Replaying it means
  storing a **live session credential**; NOT replaying it returns a 201 with no
  `Set-Cookie`, leaving the client convinced it is signed in while holding no
  session. Both options are wrong, so neither shipped.

Today a key on either route is **refused** (422 with the reason) rather than
silently ignored — a client told nothing would believe its retry was deduped.
That is a safe stopgap, not the answer.

## Why it is not just wiring

The generic recorder is byte-oriented by design (S7 says *original response*),
and byte-fidelity is exactly what conflicts with redaction. Resolving this needs
a **design decision**, one of:

1. **Per-operation replay policy** — an operation declares what may be recorded
   (e.g. record status + a redacted body; replay with the credential field
   omitted and a documented marker). Breaks byte-fidelity deliberately, per
   operation, and the contract must say so.
2. **Credential pointer** — record a reference, and re-mint or re-fetch the
   credential on replay. Preserves the response shape; needs each credential to
   be re-derivable, which reveal-once deliberately makes it not.
3. **Encrypt the recorded response** at rest with the existing envelope
   (`secrets.Vault`), so storing it is no worse than the row it came from.
   Cheapest, and the most consistent with how the webhook secret is already
   handled — but it makes replay a decrypt path, and the session-cookie case
   still needs headers recorded (US-3.6 records status + body only).
4. **Amend the ruling** to "operations that declare `idempotencyKey`", and drop
   the declaration from these two — honest, but gives up retry-safety exactly
   where a double-submit creates a duplicate account or webhook.

Option 3 + recording response headers is the likely answer; it is still an
owner-level call, not an implementation detail.

## Acceptance criteria

- [x] A decision is recorded — **ADR-0013**, founder-ratified, covering Option 3,
  the two consequences decided beyond it (seal-everything, discard-on-unopenable),
  the KEK-rotation blast radius and its runbook obligation, and the session-token
  widening.
- [x] `signup` and `createWebhook` enforce idempotency under that decision; the
  `deferredRoutes` category is deleted and `TestRouteTableMatchesOpenAPI` now
  requires enforced == declared.
- [x] No plaintext credential is written to `idempotency_keys` —
  `TestIdempotencyRecordedSecretIsNeverRecoverableAtRest` scans the raw stored
  row for both the webhook secret and the session token; bypassing the seal
  fails it (mutation-verified).
- [x] A replayed `signup` carries the same `Set-Cookie` the original did, and
  that cookie is a **working** session —
  `TestIdempotentSignupReplaysTheSameSessionCookie` (real Postgres, real chain).
- [x] The `deferredRoutes` table is gone and the drift test still passes.

## Beyond the stated criteria

- The AAD binds each record to `(principal, endpoint, key)`; unbinding it lets a
  copied row decrypt into another key's response (mutation-verified).
- A record under a retired KEK degrades to "expired" and is **discarded** so the
  key is re-claimable, rather than 500-ing or stranding the client.
- The discard is fenced on `claim_token`: unfenced, a late discard wipes a
  successor's valid record and the request executes twice (mutation-verified).
- Replay is byte-exact for empty, non-UTF8 and 256KB bodies, and preserves every
  value of a multi-valued `Set-Cookie` (mutation-verified).

## Findings filed

Two ADRs share the number 0007, so "ADR-0007" does not identify a document —
found while citing prior art for ADR-0013. Filed as **O8**, not fixed here
(renumbering rewrites citations and is not this task's scope).

## Related

US-3.6 (shipped the two enforceable routes; `deferredRoutes` documents the
exclusion inline) · S7 ruling (openapi.yaml:23) ·
`internal/identity/webhooks_http.go` · `internal/secrets` (envelope encryption)

## Outcome

Envelope-encrypting the whole recorded response — headers and body as one sealed
payload — lets S7 replay and reveal-once credential semantics hold at once,
which neither plaintext storage nor header-dropping could do alone. All four
declared routes are now enforced, closing the spec conflict US-3.6 recorded
rather than carrying it.

Two decisions went beyond the ratified text and are recorded in ADR-0013: every
response is sealed rather than only those classified as secret-bearing (a
per-route "does this contain a credential?" judgement is exactly what drifted
into the original plaintext bug), and an unopenable record is discarded rather
than erroring (this is a TTL-bounded cache, not authoritative state).

The accepted cost is that a KEK rotation invalidates the whole cache at once —
lazy re-wrap is impossible when you cannot open the record — so for up to 24h
afterwards requests that would have been deduped execute again. ADR-0013 carries
the runbook obligation (truncate at rotation) and the discard logs at Error.

Review caught a race I introduced: the discard was the only destructive
statement not fenced on `claim_token`, so a late discard could wipe a
successor's freshly-written valid record and cause the double-provision every
other statement is fenced against.

Evidence: `services/api` 22 packages RC=0, `services/cell-agent` 5 packages RC=0
under `-race`, zero failures, zero skips, Postgres-backed tests against real
Postgres.
