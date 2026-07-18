---
id: US-2.5
title: Every state change lands in the events ledger (via user/assistant/system)
epic: E2
status: done
phase: MVP
priority: critical
sprint: 3
estimate: 0.5ew
deps: [T2.5]
issue: 46
labels: [Backend, Database]
module: M2 Identity
contexts: [api-conventions, rbac, events-spine]
files: []
verify:
  - cd services/api && go test ./...
owner: agent
---

## Goal

Every state change lands in the events ledger (via user/assistant/system)

## Summary

One pipeline serves both `/events` and `/audit` (GOV-002 primitive 9).

## Acceptance criteria

- [ ] append-only; `idx(org_id, at desc)`; every mutating endpoint writes an event.

## Acceptance criteria

- [x] Append-only at the DB level (trigger raises on UPDATE/DELETE — `TestEventsLedger`);
      `idx(org_id, at desc, id desc)` matches the keyset pagination.
- [x] One pipeline serves both views: `/orgs/{org}/audit` and `/envs/{env}/events` read
      the same `events` table (T2.5); denials audited too (US-2.2).
- [x] Every org-scoped mutating endpoint writes an event with `via`: org
      create/update/deletion-schedule, member add/role-change/remove, invite
      create/revoke/accept/decline/renewal, api_key create (gap found in this
      verification pass and closed here), authz denials.

## Outcome

- Verification found `createApiKey` writing no event — closed (`api_key.created`,
  lifecycle, actor = creating user).
- Standing finding (T2.5/T2.7, carried): user-scoped changes (signup, login, personal
  tokens) have no org and the spine row requires `org_id` — actor-scoped visibility needs
  an owner decision. `via ∈ {assistant}` first appears with E13's apply flow.
