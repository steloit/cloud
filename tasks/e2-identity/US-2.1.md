---
id: US-2.1
title: Sign up, create an org (name → permanent slug, home region)
epic: E2
status: done
phase: MVP
priority: high
sprint: 2
estimate: 0.5ew
deps: [T2.1]
issue: 42
labels: [Backend, API]
module: M2 Identity
contexts: [api-conventions, rbac, events-spine]
files: []
verify:
  - cd services/api && go test ./...
owner: agent
---

## Goal

Sign up, create an org (name → permanent slug, home region)

## Summary

**AC:** slug `[a-z0-9-]{3,32}` immutable; A5 microcopy verbatim; empty world has a way forward.

## Acceptance criteria

- [x] Slug `[a-z0-9-]{3,32}` derived at create, unique, immutable — `TestOrgGovernance`
      (T2.7): "Gov Co" → `gov-co`, rename keeps the slug, collision → 409; no code path
      writes slug after create.
- [x] Signup → session → org → owner membership, all on real Postgres
      (`TestAuthLifecycle` + `TestOrgGovernance`); `home_region` accepted with the
      contract default (`aws/ap-south-1`) and PATCHable (a prefill, never a lock).
- [x] A5 microcopy + empty-world CTA live in the built console (canon-mocked); wiring the
      console to this API is E8's scope, tracked there — not silently absorbed here.

## Outcome

- API side fully carried by T2.1 (signup/session) and T2.7 (org create with slug
  derivation, subscription row, owner membership, events). No new code — this story
  verifies and records the evidence trail. Console wiring lands with E8.
