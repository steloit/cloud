---
id: US-6.2
title: Per-org monthly usage report from the metering store
epic: E6
status: done
phase: MVP
priority: high
sprint: 10
estimate: 0.5ew
deps: [T6.3]
issue: 82
labels: [Backend, API]
module: M6 Observability
contexts: [api-conventions, billing, events-spine]
files:
  - services/api/internal/identity/usage_http.go
  - services/api/internal/metering/rollup.go
  - services/api/oapi-server.cfg.yaml
  - apps/cli/internal/cli/nouns.go
  - services/api/internal/identity/usage_integration_test.go
verify:
  - cd services/api && go build ./... && go vet ./... && go test ./...
  - cd apps/cli && go test ./...
owner: agent
---

## Goal

Per-org monthly usage report from the metering store

## Summary

No invoicing yet — billing attaches in E11.

## Acceptance criteria

- [x] the report RECONCILES with raw meter events: getUsage recomputes the rollup on
      read, so a backdated open span shows measurable seconds and the report equals the
      stored derivation (integration-asserted against both the API and the quota_usage
      row).
- [x] visible from alpha day one: `GET /orgs/{org}/billing/usage` + `steloit usage
      export`, `billing.view`-gated (developer 403 naming the role, billing role 200).

## Outcome

- getUsage renders the metering rollup as the B2 meter table — recompute-on-read keeps
  it reconciled with the append-only spine (no invoicing; billing attaches in E11).
- CLI `usage export` at parity (human table + --json verbatim); SDK reaches it via the
  regenerated core.
- span seconds carry a prorated dollar figure (Σ cents-seconds / seconds-per-month) as
  the meter, not a charge.
