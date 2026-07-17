---
id: US-1.3
title: Reconciler protocol: desired state written, cell agent converges actual
epic: E1
status: stub
phase: MVP
priority: critical
sprint: 2
estimate: 1ew
deps: [T1.1]
issue: 41
labels: [Platform, Backend]
module: M1 Substrate
contexts: [provisioning]
files: []
verify: []
owner: agent
---

## Goal

Reconciler protocol: desired state written, cell agent converges actual

## Summary

D9/A2.5: desired-state tables, agent poll/apply loop, status writeback, drift report. Control-plane outage degrades to 'cannot make changes', never 'apps down'.

## Acceptance criteria

- [ ] kill the control plane; customer test pod keeps running.

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E1.
