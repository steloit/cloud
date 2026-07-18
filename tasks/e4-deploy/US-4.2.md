---
id: US-4.2
title: PR opens → preview environment with a branched database; close → teardown
epic: E4
status: stub
phase: MVP
priority: critical
sprint: 6
estimate: 1.5ew
deps: [US-4.1, T4.4]
issue: 69
labels: [Platform, Backend]
module: M5 Deploy
contexts: [provisioning, events-spine]
files: []
verify: []
owner: agent
---

## Goal

PR opens → preview environment with a branched database; close → teardown

## Summary

The certainty demo (mechanism layer). Preview env `kind=preview`, `expires_at` enforced by a background job.

## Acceptance criteria

- [ ] bot comment carries the canonical demo line: `db: production data (masked · policy) · $0.07/day · capped` (microcopy ruling 2026-07-18, wedge review).

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E4.
