---
id: US-5.3
title: Destructive commands state blast radius; --confirm <exact-name>; no --force
epic: E5
status: done
phase: MVP
priority: high
sprint: 5
estimate: 0.25ew
deps: [T5.3]
issue: 83
labels: [CLI, Security]
module: M10 Clients
contexts: [api-conventions]
files: []
verify:
  - cd apps/cli && go test ./...
owner: agent
---

## Goal

Destructive commands state blast radius; --confirm <exact-name>; no --force

## Summary

**AC:** data-destructive commands print the recovery path; every state change audited with actor = token's user.

## Acceptance criteria

- [x] Destructive commands state blast radius and the recovery path ("final backup
      restorable 30 d"), require `--confirm <exact-name>`, and no `--force` exists
      (T5.3 tests: unconfirmed and wrong-name refusals, exact-name proceeds).
- [x] Every state change lands in the audit log with actor = the token's user — the
      API side is US-2.2/T2.5's contract (denials audited too); the CLI adds no
      unaudited path by construction (it only calls contract operations).

## Outcome

Carried by T5.3's destroy grammar + the server-side audit spine.
