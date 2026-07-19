# ADR-0009 — Email is sent through Resend behind a provider interface, asynchronously via the outbox

**Status:** Accepted (founder-ratified 2026-07-19)
**Deciders:** Founder
**Relates to:** T7.2 (recovery/reset emails), T7.5 (invite emails), T10.4 (email
service integration), the events/outbox spine (GOV-002), ADR-0006 (governed-resource contract)

## Context

Multiple V1 flows need transactional email — password reset, MFA recovery,
member invites, quota/dunning notifications. The provider choice must not leak
into application code, sending must not block request paths, and templates must
survive a provider change.

## Decision

- **Resend is the primary email provider.**
- **A provider interface** (`EmailProvider`: `Send(ctx, Message) (id, error)`)
  fronts it, so SES / Postmark / SMTP can be added later without touching
  application code. Resend is an *implementation*, never a platform dependency.
- **Sending is asynchronous through the events/outbox pipeline** — a flow emits
  an email intent to the spine; a worker drains the outbox and calls the
  provider. No handler blocks on the provider; delivery is retryable and
  auditable on the spine (send attempted / delivered / bounced are events).
- **Templates are versioned and provider-agnostic** — rendered to a neutral
  `{subject, html, text}` payload *before* the provider boundary, so the
  provider never owns template state. A template id + version is stamped on each
  send for audit and reproducibility.

## Consequences

- The provider seam is `EmailProvider`; Resend lives behind it in one adapter.
- Email delivery is a governed capability (ADR-0006): it routes through the
  outbox and emits lifecycle events; it does not implement its own audit path.
- Template versioning is the contract for T12.4-style template capture later.
- Secrets: the Resend API key is an envelope-encrypted platform secret, never in
  code or plaintext config.
