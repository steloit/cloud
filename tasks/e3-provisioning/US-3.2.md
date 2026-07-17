---
id: US-3.2
title: Estimate before provision — impossible to skip at the API layer
epic: E3
status: stub
phase: MVP
priority: critical
sprint: 4
estimate: 1ew
deps: [T3.1]
issue: 56
labels: [Backend, Billing]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files: []
verify: []
owner: agent
---

## Goal

Estimate before provision — impossible to skip at the API layer

## Summary

`POST /estimates` → `createService` requires `estimate_id`; estimate line grammar is byte-identical to the eventual invoice line (one arithmetic, ADR-025).

## Acceptance criteria

- [ ] service creation without an accepted estimate is a 4xx, not a UI rule.

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E3.
