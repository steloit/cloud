---
id: US-3.6a
title: "Idempotency on credential-bearing responses: signup and createWebhook cannot replay as they stand"
epic: E3
status: in-progress
phase: MVP
priority: high
sprint: 4
deps: [US-3.6]
issue: 0
labels: [Backend, Security]
module: M4 Provisioning
contexts: [api-conventions, provisioning]
files:
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

- [ ] A decision is recorded (ADR or an amendment to the S7 ruling) covering
  what a replay of a credential-bearing response returns.
- [ ] `signup` and `createWebhook` either enforce idempotency under that
  decision, or lose the `idempotencyKey` declaration in `openapi.yaml`.
- [ ] Whatever ships, no plaintext credential is written to `idempotency_keys`.
  Pin it with a test that asserts the stored response for these operations
  contains no secret material.
- [ ] If replay is enabled for `signup`, the replayed response carries the same
  `Set-Cookie` the original did (US-3.6 records status + body only — response
  headers are dropped today, which is why signup could not simply be turned on).
- [ ] The `deferredRoutes` table in `internal/platform/idempotency/middleware.go`
  is emptied or updated, and `TestRouteTableMatchesOpenAPI` still passes.

## Related

US-3.6 (shipped the two enforceable routes; `deferredRoutes` documents the
exclusion inline) · S7 ruling (openapi.yaml:23) ·
`internal/identity/webhooks_http.go` · `internal/secrets` (envelope encryption)
