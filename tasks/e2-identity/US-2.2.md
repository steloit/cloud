---
id: US-2.2
title: Two-layer authorization on every mutating request
epic: E2
status: stub
phase: MVP
priority: critical
sprint: 3
estimate: 1ew
deps: [T2.3, T2.4]
issue: 43
labels: [Backend, Security]
module: M2 Identity
contexts: [api-conventions, rbac, events-spine]
files: []
verify: []
owner: agent
---

## Goal

Two-layer authorization on every mutating request

## Summary

`allow = matrix[role][perm]==Y AND policies.evaluate(actor, perm, {org,project,env})==permit`. Matrix denials name the missing role; policy denials name the policy; both audited.

## Acceptance criteria

- [ ] that exact sentence is the acceptance test (11-permissions contract).

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E2.
