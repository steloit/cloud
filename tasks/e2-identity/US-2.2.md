---
id: US-2.2
title: Two-layer authorization on every mutating request
epic: E2
status: done
phase: MVP
priority: critical
sprint: 3
estimate: 1ew
deps: [T2.3, T2.4]
issue: 43
labels: [Backend, Security]
module: M2 Identity
contexts: [api-conventions, rbac, events-spine]
files:
  - services/api/internal/identity/authorize.go
  - services/api/internal/identity/us22_integration_test.go
verify:
  - cd services/api && go build ./... && go vet ./...
  - cd services/api && go test ./...
owner: agent
---

## Goal

Two-layer authorization on every mutating request

## Summary

`allow = matrix[role][perm]==Y AND policies.evaluate(actor, perm, {org,project,env})==permit`. Matrix denials name the missing role; policy denials name the policy; both audited.

## Acceptance criteria

- [x] that exact sentence is the acceptance test (11-permissions contract):
      `TestUS22TwoLayerAuthorizationSentence` sweeps every non-delegated matrix cell for
      owner and developer against the live Authorizer with no policies (allow ⇔ matrix Y),
      then attaches `ai_assistant=disabled` and proves the policy layer denies a matrix-Y
      permission naming the policy.

## Outcome

- The verification pass exposed one real gap: **denials were not audited**. Closed:
  `Authorizer.deny` now records every denial on the spine (`authz.denied`; policy denials
  as `policy_trigger`, matrix/membership denials as `membership`; `detail.denied_by`
  carries the E3 explanation; actor falls back to the token id for org-key principals).
- The acceptance test asserts both denial classes land as spine rows AND surface through
  `GET /orgs/{org}/audit?action=authz.denied` — one pipeline, views over it.
- Everything else the story demands was already carried by T2.3 (matrix-as-data ceiling,
  denials naming roles), T2.4 (tighten-only policy layer, property-tested), and T2.7
  (every governance handler routes through `Authorizer.Require`).
