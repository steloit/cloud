# AI implementation guide

## The four laws (non-negotiable, user-visible)
1. **Suggest, never act** — AI output is a reviewable *proposal*; only a human with the underlying permission applies it. No auto-apply path exists in the API.
2. **Explainable** — every claim cites evidence (metric refs, trace ids, plan output, log lines). Answers without citable evidence say so.
3. **Analyze broadly, act within permissions** — retrieval is scoped to the *viewer's* RBAC; the assistant never sees what the user can't.
4. **The platform is whole without AI** — org policy `ai-assistant` (AI3): `enabled | opt_in | disabled`. Disable hides AI1/AI2/AI4/AI5/AI6–9/W10/AI10–12 instantly, deletes nothing, never touches Create/Observe/Deploy/Settings.

## Entry points → surfaces
Top-nav violet button / ⌘J → drawer (AI4, context-aware chip) ⇄ workspace (AI2: Ask · Insights AI10 · Activity AI11 · Capabilities AI12) · describe-to-provision in Create (AI1) · per-product panels (AI6–8) · insight follow-up chips (AI9) · first-run coachmark (AI5, names the opt-out).

## The proposal object (W10 grammar — render in this order)
`evidence[]` (logwell, citable) → `reasoning` → `proposed_change` (diff/config) → `impact` (cost delta, risk, blast radius) → human-only **Apply** → audit event `applied as <user>, via assistant`. Dismiss requires a reason (logged). Statuses: open|applied|dismissed|snoozed.

## Context requirements per surface
Drawer/Ask: current {org, project, env, page, selected entity} + viewer RBAC. Product panels: that service's metrics/config/events window. Insights: org-wide telemetry scan, but each insight's evidence is re-checked against viewer scope at read time.

## Prompt templates (server-side)
- **Ask**: system = four laws + "cite evidence ids for every claim; if none, say you can't verify" + context envelope (ids only; resolver fetches). 
- **Insight generation**: "given telemetry deltas {…}, produce insight {severity, evidence[], hypothesis}; never include remediation that exceeds product config changes (no IAM/secrets/network/destructive)."
- **Proposal drafting**: "draft change for {insight}; output {change, impact:{cost_delta, risk}}; refuse if change class ∉ allowlist."
- **Describe-to-provision**: "map intent → services[] each with why + shape + est; total estimate; unknowns → questions, never guesses."

## Tool integration (assistant's tools = read-only + draft)
`get_metrics, get_logs, get_trace, get_config, get_events, get_billing_summary, price_estimate, draft_proposal`. No mutating tool exists. Applying a proposal is a normal API call by the human's session.

## Conversation flows
Thread persists in workspace; drawer shares the thread. "Attach insight" pins evidence to a thread (AI9). "Turn into proposal / export runbook / share" are thread actions. Quiet/disable: threads retained (nothing deleted), hidden while disabled.
