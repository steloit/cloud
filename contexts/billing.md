---
id: billing
owns: [services/api/src/billing/**, docs/product/05-features/feature-specs.md#F9]
see: [api-conventions, canon-testing, events-spine]
---

# Billing — the one arithmetic

Authority: F9 in `docs/product/05-features/feature-specs.md` + the constitution's canon-only
numbers. The product's soul: **the shown estimate IS the invoice line, byte-for-byte** (ADR-025,
integer cents end-to-end). One pricing/quota table is consumed by all three: estimate engine,
quota evaluator, invoice generator. If they can diverge, the design is wrong.

## The rules that are nearly all backend

- **Hybrid model:** subscription (Free $0 / Pro $29 / Business $99 / Enterprise) buys platform
  capabilities; pay-as-you-go covers infrastructure; overage = beyond included quotas. ADR-041: Jobs/basic-Search/Vector are included Database capabilities within plan allowances (included, not unlimited); Pro $29 canonical.
- **Product-first presentation (ADR-039):** estimates and invoices label by outcome via the `intent`
  tag (stamped by the Composer at creation) and account by cost components — product line = Σ component
  lines byte-for-byte at every expansion level, bottoming out at published unit price × metered quantity
  or a stated plan allowance; nothing else exists. Usage ranges are honest and bounded by the cap.
- **Plans gate capabilities, never safety.** Never gated at any tier: TLS, backups, MFA, policies,
  alerts, dunning protections, self-deletion. (Table-driven test.)
- **Soft quotas keep working and bill** (egress $0.09/GB · seats $7 · builds $0.01/min · events
  $1.20/M · AI requests $2/1k); soft overage proceeds only with `confirm=true` + shown price.
  **Hard quotas fail loudly**: 429 + Retry-After / 402 with remediation. **Warn at 80%** with
  banner + bell + email showing the math — and no upsell when overage is cheaper (QA scenario 2).
- **Plan changes:** upgrades immediate + prorated; downgrades at the billing anchor, blocked with
  ALL `reasons[]` when over limits (QA scenario 4).
- **Dunning:** day 0 fail → retries → day 7 provisioning paused → day 21 suspend (state kept) →
  day 90 only-then deletion with final notice. Payment clears everything instantly. Never early.
- **Cancel ≠ delete:** services keep running and metering after plan cancellation.
- **The cap is real (F9 flagship, 2026-07-18):** an org's hard spend bound is *enforced* — at the cap,
  provisioning pauses and overage stops accruing (nothing deleted, state kept, instant resume);
  an estimate whose acceptance would cross the cap is refused with the math. Never alerts-only.
- **External Bindings extend certainty honestly (ADR-0004/A5):** Storage/AI Bindings show the *provider's*
  price at bind (estimate-at-bind), not a Steloit markup — we don't own that economics. AI Binding surfaces
  provider token usage in the billing view and offers **soft** spend control (suspend the binding at a
  threshold) — hard real-time in-line caps require sitting in the request path (the gateway commodity, not built).
- Metering flows from day one (D10, see `provisioning`); invoices only *price* what was metered.

## Testing

Time-warped clock for anything dunning/anchor/proration — never real waits. Canon fixtures are
the regression numbers ($208-family, 87/100GB → ~$1.62). See `canon-testing`.

## Mistake bank

- A second pricing constant anywhere (one table, three consumers).
- Re-running the pricing engine at accept/invoice time (the persisted estimate line wins).
- Floats in money math, anywhere, including tests (integer cents only).
- Gating a safety feature behind a plan check (the never-gated list is law).
- Deleting anything before day 90, or pausing anything before day 7 (the timeline is exact).
