---
id: US-2.3
title: Personal token: plaintext exactly once; role demotion shrinks live tokens
epic: E2
status: stub
phase: MVP
priority: high
sprint: 3
estimate: 0.5ew
deps: [T2.2]
issue: 44
labels: [Backend, Security]
module: M2 Identity
contexts: [api-conventions, rbac, events-spine]
files: []
verify: []
owner: agent
---

## Goal

Personal token: plaintext exactly once; role demotion shrinks live tokens

## Summary

**AC:** QA scenario 6 end-to-end — GET returns prefix+hash only; tokens re-evaluate against *current* roles at use time.

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E2.
