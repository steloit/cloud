---
id: US-3.6
title: Failed provisioning never bills and never strands state
epic: E3
status: in-progress
phase: MVP
priority: critical
sprint: 4
estimate: 0.5ew
deps: [S7, US-3.3]
issue: 60
labels: [Backend, Billing]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files: []
verify: []
owner: agent
---

## Goal

Failed provisioning never bills and never strands state

## Summary

Implements the S7 idempotency ruling.

## Acceptance criteria

- [ ] kill provisioning mid-flight → no meter events, clean desired/actual state, retry-safe.

## Blocked

Blocked on P1: "failed provisioning never bills" is proven (T3.7 integration); "never strands state" is the reconciler cleanup path (T3.4+).
