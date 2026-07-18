---
id: ai-plane
owns: [services/api/src/assistant/**, docs/product/12-ai/**]
see: [rbac, events-spine, api-conventions]
---

# AI plane — the four laws as architecture

Authority: `docs/product/12-ai/ai-implementation.md` + GOV-002 §7 (ADR-005: the laws are
permanent). They are enforced by *shape*, not by prompt:

1. **Suggest, never act** — every AI capability ends in a proposal object; **no auto-apply path
   exists in the API** (not policy — there is no such endpoint to call). Applying is a normal
   human-session call that re-runs the full two-layer RBAC for the underlying change.
2. **Explainable** — every claim cites evidence ids (`metric|trace|log|query_plan|config|cost|
   schema|event`); no citable evidence → the answer says so.
3. **Read broadly, act within permissions** — retrieval is scoped to the *viewer's* RBAC; context
   envelopes carry **ids only**; a resolver fetches data at prompt-fill time. Insights are
   generated org-wide but evidence is re-checked against viewer scope at read time.
4. **Whole without AI** — the `ai-assistant` org policy (`enabled|opt_in|disabled`) hides every AI
   surface instantly, deletes nothing (threads retained-but-hidden), re-enables instantly.
   `/assistant/*` reads return the 404 empty-equivalent when disabled (per the yaml).

## Mechanics

- Tools (read-only + draft, exactly eight): `get_metrics get_logs get_trace get_config get_events
  get_billing_summary price_estimate draft_proposal`. No mutating tool exists.
- Proposal object renders in W10 order: `evidence[] → reasoning → proposed_change → impact{
  cost_delta, risk, blast_radius}`; statuses `open|applied|dismissed|snoozed`; dismiss requires a
  logged reason; apply audits as `applied as <user>, via assistant` on the events spine.
- Proposal drafting refuses change classes outside the allowlist — never IAM/secrets/network/
  destructive, even as one-click suggestions (describe-only).
- Inference is a bought API behind a swappable interface (D5); AI requests are metered ($2/1k).

## AI Binding ≠ AI assistant (ADR-0004/A5.4)

This pack is the **assistant** (Steloit's own AI, the four laws). Separately, the customer's app governs
its *own* LLM dependency via an **AI Binding** (a Binding to an external provider — see `provisioning`):
allow-policy, credentials-in-Secrets, config injection, **estimate-at-bind** (model $/1k tokens), cost
visibility (provider usage API), lifecycle audit, **soft** spend control. It is control-plane governance —
**no proxy, no routing, no hard in-line caps** (that is the AI-gateway commodity, never built). There is
no AI Gateway product. Don't conflate the two: the assistant proposes; the AI Binding governs a dependency.

## The Composer (ADR-039/040 — describe-to-provision, T13.7)

AI1/describe-to-provision is the **Composer**: it maps an intent to one or more *named resolutions with
stated semantics*, each priced (`price_estimate`), and proposes — never provisions (Law 1: the human
accepts; the accepted estimate is the reconciliation contract between catalog and architecture). It
stamps `intent:` on everything it creates (the billing grouping key). Execution-model migrations it
suggests are proposals like any other: visible, priced, consented.

## Mistake bank

- Any endpoint whose effect is "AI applies a change" (Law 1 is architectural — it must not exist).
- A Composer path that selects or migrates an execution model without a consented estimate (ADR-040).
- Passing resolved data (not ids) in a context envelope (breaks Law 3's re-check point).
- Insights/threads deleted on disable (retain-and-hide; QA scenario 7 checks byte-identity).
- Evidence-free assertions in assistant output (Law 2: cite or say you can't verify).
- Violet used for anything non-AI in UI, or AI surfaces that survive the disable policy.
