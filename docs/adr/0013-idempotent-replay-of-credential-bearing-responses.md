# ADR-0013 — Idempotent replay of credential-bearing responses

- **Status:** Accepted — founder-ratified 2026-07-26
- **Context:** US-3.6 (S7 implementation) · US-3.6a (this decision)
- **Builds on:** `services/api/internal/secrets` (T3.5 — `Seal`/`Open`, the `KEK`
  interface, and the `kek_id`-per-row lazy-rotation model) · `docs/architecture.md`
  D5 ("KMS envelope"). No ADR covers the envelope/KEK model itself.
- **Supersedes:** the `deferredRoutes` stopgap shipped in US-3.6

## Problem

The ratified S7 ruling says a mutating POST may carry `Idempotency-Key`, and a
replay **returns the ORIGINAL response**. `openapi.yaml` declares the parameter
on four operations. Two of them return **credential material** in their 2xx
bodies:

- `createWebhook` — the reveal-once signing secret (`WebhookCreated.secret`),
  which the same handler envelope-encrypts before persisting it.
- `signup` — the session cookie, set via the carrier on the 201.

"Return the original response" therefore means **storing that credential for the
24h replay window**. US-3.6 shipped the two safe routes and left these two
declared-but-unenforced, refusing to resolve the conflict silently.

Neither obvious option is acceptable on its own: storing the response in
plaintext defeats reveal-once, and *not* replaying the headers returns a 201
with no `Set-Cookie`, leaving the client convinced it is signed in while holding
no session.

## Decision

**Envelope-encrypt the recorded response — headers and body together — using the
existing `secrets.Seal`/`Open` path.** Both promises then hold at once: the
replay is byte-for-byte equivalent from the client's perspective, and no
credential material is ever persisted in plaintext.

Concretely:

- `idempotency_keys.response` (plaintext `bytea`) is replaced by
  `response_ciphertext`/`_nonce`/`_wrapped_dek`/`_dek_nonce`/`_kek_id`.
- The sealed payload is `{header, body}` as JSON. `body` is `[]byte`, which
  encodes as base64 and decodes byte-exact.
- AAD is `idem:<principal>\x1f<endpoint>\x1f<key>`, so a row copied to another
  key fails authentication instead of decrypting into someone else's response.
- All four declared routes are enforced; the deferred category is deleted.

### Two consequences decided beyond the ratified text

**1 · Every recorded response is sealed, not only credential-bearing ones.**

Classifying per route would mean maintaining a "does this response contain a
credential?" judgement that drifts the moment a handler adds a field — which is
exactly how the plaintext webhook secret entered the design. A uniform seal has
no classification to get wrong. The cost is two AES-GCM operations on four
low-volume POSTs.

The coupling is worth naming: this widens the rotation blast radius below from
two routes to four.

**2 · An unopenable record is discarded, not surfaced as an error.**

The vault errors on a KEK mismatch ("rotate before reading") because it holds
authoritative secrets. This table is a **TTL-bounded replay cache**. Erroring
would turn a legal retry into a 500; returning the status without its body would
be a lie. So an unopenable record is treated as absent — and *discarded*, so the
key is immediately re-claimable, because leaving a record nobody can ever replay
would strand the client until the TTL, which is the stranding US-3.6 exists to
prevent.

The discard is fenced on the `claim_token` of the record actually read. Without
that fence a late discard can wipe a successor's freshly-written valid record
and cause the double-provision every other statement in the module is fenced
against.

## Consequences

**KEK rotation invalidates the entire replay cache.** Lazy re-wrap is impossible
here — you cannot re-seal what you cannot open — so every row becomes unopenable
at once and is discarded. For up to 24h afterwards, requests that would have
been deduped **execute again**, including `POST /v1/estimates`, whose one-shot
semantics are the reason this layer exists.

This is accepted, with two obligations:

- **Rotation runbook:** truncate `idempotency_keys` at rotation. It is a 24h
  cache; truncation is the explicit form of what the code does implicitly, and
  doing it deliberately keeps the window from being discovered in production.
- The discard logs at **Error**, not Warn: each occurrence is a potential
  double-execution.

**The raw session token is now held outside `sessions`.** That table stores only
a sha256 hash. Replaying signup requires the original `Set-Cookie`, so the raw
token lives envelope-encrypted in `idempotency_keys` for the replay window,
recoverable by anyone holding both DB read access and the KEK. This is a real
widening — recorded here rather than left implicit, and noted in the `session`
package doc, whose "raw value never stored" invariant would otherwise be false.

**A replay 24h later re-issues a cookie for a session that may have been logged
out.** The client receives the original 201 and a dead cookie. This is correct
S7 semantics ("return the original response") but is a surprise worth stating.

**Headers are replayed except those describing *this* response** — `Date`,
`Content-Length`, and the hop-by-hop set. That list is a denylist, safe only
while no middleware above this layer sets per-request headers; the constraint is
recorded in `skipOnReplay` for whoever adds one.

## Alternatives rejected

- **Per-operation replay policy** (record a redacted body) — breaks the
  byte-fidelity S7 promises, per operation, and needs the same drifting
  classification as (1) above.
- **Credential pointer / re-mint on replay** — reveal-once deliberately makes
  the credential non-re-derivable.
- **Amend the ruling to "operations that declare `idempotencyKey`"** and drop
  the two declarations — honest, but surrenders retry-safety exactly where a
  double-submit creates a duplicate account or webhook.

## Evidence

`TestIdempotencyRecordedSecretIsNeverRecoverableAtRest` scans the raw stored row
for the secret and the session token (bypassing the seal fails it) ·
`TestIdempotencyRecordCopiedToAnotherKeyDoesNotDecrypt` (unbinding the AAD fails
it) · `TestIdempotencyRecordUnderARetiredKEKIsTreatedAsExpired` ·
`TestIdempotencyDiscardCannotWipeASuccessorsValidRecord` (removing the fence
fails it) · `TestIdempotentSignupReplaysTheSameSessionCookie`, which asserts the
replayed cookie is a *working* session, not merely an equal string.
