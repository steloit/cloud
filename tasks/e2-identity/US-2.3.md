---
id: US-2.3
title: Personal token: plaintext exactly once; role demotion shrinks live tokens
epic: E2
status: done
phase: MVP
priority: high
sprint: 3
estimate: 0.5ew
deps: [T2.2]
issue: 44
labels: [Backend, Security]
module: M2 Identity
contexts: [api-conventions, rbac, events-spine]
files:
  - services/api/internal/identity/us23_integration_test.go
verify:
  - cd services/api && go build ./... && go vet ./...
  - cd services/api && go test ./...
owner: agent
---

## Goal

Personal token: plaintext exactly once; role demotion shrinks live tokens

## Summary

**AC:** QA scenario 6 end-to-end — GET returns prefix+hash only; tokens re-evaluate against *current* roles at use time.

## Acceptance criteria

- [x] QA scenario 6 green (`TestUS23TokenRevealOnceAndLiveShrink`, real Postgres in CI):
      create → plaintext exactly once (`shown_once`, `hash_stored`, DB holds no plaintext);
      list returns prefix + metadata only; an admin's full-scope token authorizes
      `members.invite` (evaluator + live HTTP), the admin is demoted to billing, and the
      SAME live token is denied immediately (`role:billing` named, HTTP 403) while
      `billing.view` keeps working — shrink, not kill.

## Outcome

- No production code needed: T2.2's design (tokens act as their user; roles resolved from
  the membership row at check time, never baked into the credential) makes the shrink
  immediate by construction. This story pins that property with an end-to-end test so a
  future cache can't silently break it.
