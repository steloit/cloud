---
id: US-2.4
title: Last owner cannot leave or be demoted
epic: E2
status: stub
phase: MVP
priority: medium
sprint: 3
estimate: 0.25ew
deps: [T2.7]
issue: 45
labels: [Backend, Database]
module: M2 Identity
contexts: [api-conventions, rbac, events-spine]
files: []
verify: []
owner: agent
---

## Goal

Last owner cannot leave or be demoted

## Summary

**AC:** DB trigger enforces ≥1 owner; API returns 409 with `reasons[]`.

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E2.
