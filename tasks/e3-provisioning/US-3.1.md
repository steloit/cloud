---
id: US-3.1
title: Project + environment creation; environment sets the region
epic: E3
status: done
phase: MVP
priority: high
sprint: 3
estimate: 0.5ew
deps: [T3.2]
issue: 55
labels: [Backend, API]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files: []
verify:
  - cd services/api && go test ./...
owner: agent
---

## Goal

Project creation **auto-creates the `production` environment** (born, not created — ADR-037); environment sets the region

## Summary

ADR-037 contract: `POST /projects` atomically creates the project + one environment named `production` carrying the home region; the create response includes it. No project ever exists without an environment; the user never creates their first one. Implicit ≠ anonymous: the name is real in every API response/audit row from birth.

ADR-004: services inherit the env region; overrides are explicit priced exceptions. Alpha regions: us-central1 (founders), asia-south1 once partner-touchable (A1.7).

## Acceptance criteria

- [x] `POST /orgs/{org}/projects` creates project + `production` in ONE transaction
      (T3.2); the env is real in every response and audit row from birth; env_count 1 in
      the create response.
- [x] Region flows org home → env (override explicit) → service (the env's region is
      what estimates price against); `UNIQUE(org_id,name)` enforced, 409 on collision.

## Outcome

Carried by T3.2 (implicit production in-tx, region inheritance, uniqueness) and verified
in `TestProjectsAndEnvironments`. No new code — evidence recorded.
