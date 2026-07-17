---
id: US-6.1
title: Per-env metrics and logs queryable (with SSE tail)
epic: E6
status: stub
phase: MVP
priority: high
sprint: 6
estimate: 0.5ew
deps: [T6.1, T6.2]
issue: 92
labels: [Backend, API]
module: M6 Observe
contexts: [events-spine, billing]
files: []
verify: []
owner: agent
---

## Goal

Per-env metrics and logs queryable (with SSE tail)

## Summary

DB + web-service basics: CPU, connections, p95, logs.

## Acceptance criteria

- [ ] `GET /envs/{env}/metrics|logs` per yaml; `x-streamable` honored.

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E6.
