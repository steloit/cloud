---
id: US-2.4
title: Last owner cannot leave or be demoted
epic: E2
status: done
phase: MVP
priority: medium
sprint: 3
estimate: 0.25ew
deps: [T2.7]
issue: 45
labels: [Backend, Database]
module: M2 Identity
contexts: [api-conventions, rbac, events-spine]
files: []
verify:
  - cd services/api && go test ./...
owner: agent
---

## Goal

Last owner cannot leave or be demoted

## Summary

**AC:** DB trigger enforces ≥1 owner; API returns 409 with `reasons[]`.

## Acceptance criteria

- [x] DB trigger enforces >=1 owner (`members_last_owner_guard`, migration
      20260718192530) — demote AND remove both blocked; the guard steps aside during org
      CASCADE so deletion still works.
- [x] API returns 409 with `reasons[]` — `TestOrgGovernance` (T2.7) demotes and removes
      the last owner via live HTTP and asserts both 409s; the problem body carries
      `reasons: ["last owner cannot be demoted or removed"]` with F1 named in the
      remediation.

## Outcome

Carried entirely by T2.7 (trigger + error mapping + integration coverage). This story
records the verification; no new code.
