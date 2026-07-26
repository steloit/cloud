---
id: CK-M2
title: Identity + RBAC + audit real
epic: E2
status: done
phase: MVP
priority: critical
sprint: 3
estimate: 0ew
deps: [US-2.2, US-2.3, T2.7]
issue: 186
labels: [milestone-checkpoint]
module: M2 Identity
contexts: [api-conventions, rbac, events-spine]
files: []
verify: []
owner: agent
---

## Goal

Identity + RBAC + audit real

## Summary

**Exit criteria:**
- [ ] QA scenario 6 (token reveal) green
- [ ] QA scenario 8 (invite lifecycle) green
- [ ] RBAC matrix table-driven test over every cell

## Exit evidence (all green in CI on real Postgres)

- [x] QA scenario 6 (token reveal) — `TestUS23TokenRevealOnceAndLiveShrink` (US-2.3):
      plaintext exactly once, prefix-only listing, live shrink on demotion.
- [x] QA scenario 8 (invite lifecycle) — `TestOrgGovernance` (T2.7): 7-day expiry,
      renew-notifies-inviter, wrong-account blocked, already-used sign-in path.
- [x] RBAC matrix table-driven test over every cell — `rbac_test.go` (T2.3) sweeps all
      cells through the public evaluator; `TestPropertyNoPolicyWidensN` (T2.4) adds 220
      randomized policy environments over every N; `TestUS22TwoLayerAuthorizationSentence`
      (US-2.2) re-runs the sweep against live membership with denials audited.

## Outcome

M2 identity is real: argon2id auth, server-side sessions, reveal-once tokens (personal +
org), matrix-as-data RBAC with the tighten-only policy layer, the append-only events
spine with SSE, org/member/invite governance, problem+json everywhere. Open findings
(org-key scope model, org-less user events, org-key prefix, A6 copy for 3 roles) are
recorded on their tasks and do not gate the checkpoint.
