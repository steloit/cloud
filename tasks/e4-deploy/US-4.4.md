---
id: US-4.4
title: Deploy markers land on the events spine
epic: E4
status: done
phase: MVP
priority: medium
sprint: 5
estimate: 0.25ew
deps: [T2.5]
issue: 71
labels: [Backend]
module: M5 Deploy
contexts: [provisioning, events-spine]
files: []
verify:
  - cd services/api && go test ./...
owner: agent
---

## Goal

Deploy markers land on the events spine

## Summary

F4: every chart of the affected env can show them (QA scenario 1's #142/#143 replay depends on this).

## Acceptance criteria

- [ ] deploy event with number + sha emitted per deployment.

## Acceptance criteria

- [x] Every deployment record lands a `deploy`-kind event on the spine carrying
      `number` + `sha` (+ service name) — `TestDeployments` asserts the marker row;
      rollbacks add `deploy.rolled_back`. Any chart of the env can render markers
      from `GET /envs/{env}/events?kind=deploy`.

## Outcome

Carried by T4.3 (markers emitted at record creation) on T2.5's spine. The console's
chart-marker rendering consumes the same rows (E8).
