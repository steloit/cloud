# ADR-0010 — Billing runs through Stripe behind a provider abstraction

**Status:** Accepted (founder-ratified 2026-07-19)
**Deciders:** Founder
**Relates to:** T11.4 (payment provider integration), T11.1/T11.2 (pricing tables,
subscription state machine), ADR-025 (money = integer cents), ADR-040 (execution
models are replaceable; the platform is not defined by any one)

## Context

V1 billing needs a payment provider for subscriptions, invoices, and dunning.
The provider must not become a platform dependency — its concepts (Stripe
customers, subscriptions, webhooks) must stay behind a boundary so the billing
domain model is the platform's own.

## Decision

- **Stripe is the initial billing provider.**
- **A provider abstraction** (`BillingProvider`) fronts it — customer/
  subscription/invoice/payment-method operations and webhook ingestion are
  expressed in platform terms, with Stripe as one adapter. Additional providers
  can be added without changing the billing domain.
- **Stripe is an implementation, not a platform dependency.** The subscription
  state machine (T11.2), pricing/quota tables (T11.1), and the money model
  (integer cents, ADR-025) are the platform's own and provider-agnostic; the
  adapter maps them to Stripe.
- Webhook ingress follows the established pattern (HMAC-verified, idempotent by
  delivery id, off `/v1`); provider events reconcile the platform's own
  subscription state, they do not *become* it.

## Consequences

- The provider seam is `BillingProvider`; Stripe lives behind it in one adapter.
- The subscription lifecycle (trial → anchor → dunning → cancelled/reactivated,
  the Borealis canon states) is modeled in the platform, reconciled against —
  not driven by — Stripe.
- Money stays integer cents end-to-end; the adapter converts at the boundary.
- Secrets: Stripe keys + webhook secret are envelope-encrypted platform secrets.
