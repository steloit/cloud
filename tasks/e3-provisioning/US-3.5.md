---
id: US-3.5
title: Deleting anything takes a final backup; typed-confirm names dependents
epic: E3
status: blocked
phase: MVP
priority: high
sprint: 4
estimate: 0.5ew
deps: [T3.3]
issue: 59
labels: [Backend, Security]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files: []
verify: []
owner: agent
---

## Goal

Deleting anything takes a final backup; typed-confirm names dependents

## Summary

**AC:** QA scenario 10 — delete disabled until exact name typed; 409 names dependents; final backup recorded.

## Blocked

Blocked on P1: the 409-dependents half landed (T3.6, U6-tested); "final backup recorded" requires the driver (T3.4). Typed-confirm is client-side (console/CLI).
