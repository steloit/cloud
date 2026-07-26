---
id: CK-M3
title: Estimate-gated provisioning end-to-end
epic: E3
status: in-progress
phase: MVP
priority: critical
sprint: 4
estimate: 0ew
deps: [US-3.2, US-3.3]
issue: 187
labels: [milestone-checkpoint]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files: []
verify: []
owner: agent
---

## Goal

Estimate-gated provisioning end-to-end

## Summary

**Exit criteria:**
- [ ] `steloit db create` → estimate → approved → `ready`, metered
- [ ] Canon arithmetic invariants green against the estimate engine

## Unblocked (2026-07-26)

The blocker text was stale: P1 landed 2026-07-19 and US-3.3 stood up a real
GKE+CNPG cell, proved the loop end-to-end, and tore it down. Both deps are done.
