---
id: US-5.4
title: Errors render problem+json as three lines (what / why-where / what-next)
epic: E5
status: done
phase: MVP
priority: medium
sprint: 5
estimate: 0.25ew
deps: [T2.6]
issue: 84
labels: [CLI]
module: M10 Clients
contexts: [api-conventions]
files: []
verify:
  - cd apps/cli && go test ./...
owner: agent
---

## Goal

Errors render problem+json as three lines (what / why-where / what-next)

## Summary

**AC:** 402 shows the math; 429 honors Retry-After; every error names a next step.

## Acceptance criteria

- [x] problem+json renders as three lines (what · why/where · what-next); 403s name the
      role/policy via denied_by; 409s list ALL reasons; 402 carries the math the server
      sent; 429 prints Retry-After; every error path shows remediation (output_test.go).
- [x] Exit codes map 401/403→3 · 404→4 · 409→5 · 402→6 · 429→7; garbage bodies degrade
      to 1, never panic.

## Outcome

Carried by T5.5's Problem renderer; auto-retry-on-429 for idempotent reads is deferred
to the SDK ergonomics layer (T5.6) where retries belong.
