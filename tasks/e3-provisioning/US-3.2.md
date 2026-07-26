---
id: US-3.2
title: Estimate before provision — impossible to skip at the API layer
epic: E3
status: done
phase: MVP
priority: critical
sprint: 4
estimate: 1ew
deps: [T3.1]
issue: 56
labels: [Backend, Billing]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files: []
verify:
  - cd services/api && go test ./...
owner: agent
---

## Goal

Estimate before provision — impossible to skip at the API layer

## Summary

`POST /estimates` → `createService` requires `estimate_id`; estimate line grammar is byte-identical to the eventual invoice line (one arithmetic, ADR-025).

## Acceptance criteria

- [ ] service creation without an accepted estimate is a 4xx, not a UI rule.

## Acceptance criteria

- [x] Impossible to skip at the API layer — `TestServices` (T3.3): create without
      `estimate_id` → 422 naming the field; bogus/expired/reused/cross-env estimates →
      409 with remediation; a mismatched shape is refused WITHOUT burning the estimate;
      the accepted estimate is one-shot.
- [x] One arithmetic: the service's `monthly_estimate_cents` IS the accepted estimate
      line's cents (engine-priced both times, canon-pinned); line grammar shared with
      invoices by construction (`estimates.Line` is the single type).

## Outcome

Carried by T3.1 (engine + one-shot env-fenced Accept) and T3.3 (the gate in
createService). The estimate IS the contract; nothing provisions without one.
