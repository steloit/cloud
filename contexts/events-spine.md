---
id: events-spine
owns: [services/api/src/events/**]
see: [api-conventions, rbac]
---

# The events spine

GOV-002 primitive 9: **one append-only pipeline** feeds everything that "happened" — audit,
`/events`, SSE, the bell, notifications, deploy markers, billing dunning emails. There is exactly
one events table; `/orgs/{org}/audit` and `/envs/{env}/events` are views over it.

## Contract

- Row: `id (evt_), org_id, actor, via ∈ {user, assistant, system}, action, subject, at, detail`.
  Append-only at the DB level; `idx(org_id, at desc)`.
- **Every state change writes an event** — grants, denials (yes, denials), lifecycle edges,
  applied AI proposals (`applied as <user>, via assistant`), billing transitions.
- SSE: `x-streamable` reads accept `Accept: text/event-stream`; reconnect must support cursor
  resume. The spine drives CLI `-f`, console toasts/bell/status pills (SSE-primary, poll-fallback
  2s→10s per `docs/product/10-state-management/state.md`).
- Deploy markers: each deployment emits an event with number + sha so ANY chart of the affected
  env can render it (QA scenario 1's #142/#143 replay depends on this).
- Notifications & emails are **routing over the spine**, never separate truths: one event → at
  most one email → one action (23-emails). Quiet hours affect routing only — recording and firing
  are untouched; security/paging classes are never gated.

## Mistake bank

- A second "activity" or "notifications" table that duplicates spine facts (one pipeline, views over it).
- Mutating or deleting event rows (append-only; corrections are new events).
- Emails or bell counts computed from anything but the spine (banner/email number mismatch = defect).
- SSE without cursor resume (reconnects silently drop events).
- Forgetting `via` provenance — assistant-applied changes must be distinguishable forever.
- A webhook that BACKFILLS events predating its creation — a scan-based fan-out (`events ⋈
  webhooks`) with no `e.at >= w.created_at` guard floods a brand-new webhook with the org's entire
  history. A webhook receives events from creation onward, never a backfill (T10.3).
- Letting a routing pref gag a RECORDING — email-off suppresses the *email route*; the bell row
  and the spine event still happen (prefs route, they never gag the audit; glossary law) (T10.3).
