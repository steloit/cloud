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
- A spine-scan projection whose "already handled" evidence is **the row it would have written**.
  `ListPendingWebhookEvents` gets away with it because its deliverability filter is IN SQL
  (`e.kind = ANY(w.events)`) and `webhook_deliveries` records a terminal row for the rest — so
  undeliverable events never occupy the window. A projection that decides worthiness in **Go**
  leaves no row for the events it skips, which then permanently satisfy `WHERE n.id IS NULL` and
  fill every later `LIMIT` batch: after one batch of silence the projection starves forever.
  Advance a durable cursor for every *scanned* event, worthy or not, in the same transaction as
  the fan-out (US-10.3 enrichment review).
- Pairing a per-EVENT skip predicate with a per-(user,event) `ON CONFLICT DO NOTHING` and calling
  the result idempotent. They are mutually exclusive — if the scan excludes handled events the
  conflict never fires — and a non-transactional per-member fan-out that crashes halfway leaves
  the event looking handled while most members were never notified (US-10.3 enrichment review).
- Bounding a spine scan by `e.at >= <watermark>`. `events.at` defaults to `now()`, which in
  Postgres is **transaction start** time — so a write tx that opens before a scan tick and commits
  after it lands *below* the watermark and is skipped **permanently and silently**. Timestamps
  order events; they never prove one was handled. Use a terminal ledger row keyed by `event_id`
  (US-10.3 enrichment review; the webhook path's `e.at >= w.created_at` is safe only because it
  bounds a *starting point*, never a resume point).
- Assuming a spine consumer exists because its TABLE does. `email_deliveries` looks like an outbox,
  but the mailer dispatcher scans *events by action* against a static template registry and nothing
  reads that table's `pending` rows — enqueuing into it delivers nothing. Read the consumer's scan
  predicate before treating any table as a queue (US-10.3 enrichment review, third recurrence of
  this error class in one task: prior art must be read end-to-end, not pattern-matched).
