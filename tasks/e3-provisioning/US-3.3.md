---
id: US-3.3
title: Accepted service provisions via the reconciler; metering starts at ready
epic: E3
status: stub
phase: MVP
priority: critical
sprint: 4
estimate: 1.5ew
deps: [US-1.3, T3.4]
issue: 57
labels: [Platform, Backend]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files: []
verify: []
owner: agent
---

## Goal

Accepted service provisions via the reconciler; metering starts at ready

## Summary

Status walks provisioning → ready (ADR-024); metering starts at `ready`, never before (D10).

## Acceptance criteria

- [ ] end-to-end: desired row → cell agent → Neon tenant + gVisor endpoint pod → ready; usage events flowing.

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E3.
