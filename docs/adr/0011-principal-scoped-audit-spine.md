# ADR-0011 — Principal-scoped audit spine (PROPOSED, post-MVP)

**Status:** Proposed (founder-directed backlog candidate, 2026-07-19) — do NOT implement pre-MVP
**Deciders:** Founder (deferral); design owner TBD
**Relates to:** GOV-002 (the events spine primitive), ADR-0006 (governed-resource contract),
T7.2 (password reset — the use case that surfaced this), T10.4 (the email outbox)

## Context

The events spine is **organization-scoped**: `events.org_id text NOT NULL REFERENCES orgs(id)`.
Every audit fact belongs to exactly one org, which is correct for org-governed actions
(members, projects, services, policies, deploys).

Account-level security actions have **no org**: password reset, and future login-security,
account recovery, new-device/new-location sign-in notices, and personal security alerts. T7.2
hit this directly — a reset event can't be written to the org spine (the FK fails), and fanning a
personal action into the user's orgs is wrong (a new user has no org; a personal reset isn't org
business). T7.2 resolved it by driving the account email from its own durable fact (the
reset-token row) via `mailer.AccountSource`, reusing the idempotent `email_deliveries` ledger.

That works for *email delivery*, but it means account-level security actions currently have **no
first-class audit trail** — reset request/complete, and future login events, are not on any spine.

## Decision (deferral)

**Keep the organization-scoped events model for the MVP. Do not change the event schema now.**
Dedicated durable facts (password-reset records, and future login/recovery records) remain the
source of truth for account-level workflows. This avoids a ~27-site ripple across the spine
consumers (recorder, hub, SSE streamer, audit reader, mailer) while the platform progresses to
its first production milestone. The current solution is architecturally acceptable.

## Future direction (the actual proposal, to design deliberately later)

Introduce a **generalized principal-scoped event model** — NOT simply `org_id` nullable. A single
spine whose events are scoped to a *principal* of a known kind, so the audit model has no special
cases:

- **Organization** events (today's org-scoped facts)
- **User / account** events (password reset, login security, recovery, security notices)
- **Service-account** events (org-key / workload-identity actions)
- **System** events (platform-internal)

Design considerations for that milestone:
- A `principal_kind` + `principal_id` pair (or a typed scope) replacing the bare `org_id`, so
  every consumer filters by principal, not org — org audit = principal_kind='org'; a user's
  personal security log = principal_kind='user'.
- The audit read surface (`/audit`) and SSE fanout generalize to "events for a principal I can
  see," preserving org-audit semantics as one case.
- The governed-resource contract (ADR-0006) and the email outbox (T10.4) both consume the unified
  spine — the `AccountSource` seam collapses back into the single event scan.
- Migration path from the current org-only table (backfill principal_kind='org').

This is a larger evolution of the audit model; it must be designed deliberately as its own
milestone, not introduced to solve a single use case. **Backlog item: promote this ADR from
Proposed to a design task when account-security features (login notices, personal security log)
are scheduled.**

## Consequences

- No code change now; this file is the durable record so the deferral isn't forgotten.
- New account-level workflows until then follow the T7.2 pattern (own durable fact +
  `mailer.AccountSource`), and accept that they lack a spine audit trail (documented per feature).
