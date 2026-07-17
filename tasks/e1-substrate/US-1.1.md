---
id: US-1.1
title: Every resource row carries cell_id; provisioning routes through cell selection
epic: E1
status: stub
phase: MVP
priority: high
sprint: 2
estimate: 0.25ew
deps: []
issue: 39
labels: [Backend, Database]
module: M1 Substrate
contexts: [provisioning]
files: []
verify: []
owner: agent
---

## Goal

Every resource row carries cell_id; provisioning routes through cell selection

## Summary

Invariant 1 — even while the answer is always cell-0.

## Acceptance criteria

- [ ] schema check: no resource table without `cell_id`; provisioner calls a cell-selection function.

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E1.
