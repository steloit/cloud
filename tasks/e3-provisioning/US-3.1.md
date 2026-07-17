---
id: US-3.1
title: Project + environment creation; environment sets the region
epic: E3
status: stub
phase: MVP
priority: high
sprint: 3
estimate: 0.5ew
deps: [T3.2]
issue: 55
labels: [Backend, API]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files: []
verify: []
owner: agent
---

## Goal

Project + environment creation; environment sets the region

## Summary

ADR-004: services inherit the env region; overrides are explicit priced exceptions. Alpha regions: us-central1 (founders), asia-south1 once partner-touchable (A1.7).

## Acceptance criteria

- [ ] region flows env → service; `UNIQUE(org_id,name)` on projects.

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E3.
