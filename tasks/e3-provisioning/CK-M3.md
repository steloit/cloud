---
id: CK-M3
title: Estimate-gated provisioning end-to-end
epic: E3
status: blocked
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

## Blocked

Blocked on P1 via US-3.3: estimate-gated provisioning end-to-end needs a real cell. Everything control-plane-side of the checkpoint is green.
