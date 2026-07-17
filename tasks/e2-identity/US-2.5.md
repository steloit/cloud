---
id: US-2.5
title: Every state change lands in the events ledger (via user/assistant/system)
epic: E2
status: stub
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
verify: []
owner: agent
---

## Goal

Every state change lands in the events ledger (via user/assistant/system)

## Summary

One pipeline serves both `/events` and `/audit` (GOV-002 primitive 9).

## Acceptance criteria

- [ ] append-only; `idx(org_id, at desc)`; every mutating endpoint writes an event.

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E2.
