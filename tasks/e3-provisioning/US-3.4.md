---
id: US-3.4
title: Bind an external app to the database (credentials minted, rotated on unbind)
epic: E3
status: blocked
phase: MVP
priority: high
sprint: 4
estimate: 0.75ew
deps: [T3.6]
issue: 58
labels: [Backend, Security]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files: []
verify: []
owner: agent
---

## Goal

Bind an external app to the database (credentials minted, rotated on unbind)

## Summary

GOV-002 v0 first-class mode: Steloit-as-data-layer.

## Acceptance criteria

- [ ] `<TARGET>_URL` injected/displayed; bindings cost $0; read-only scope enforced by the datastore itself.

## Blocked

Blocked on P1: bind/mint/rotate mechanics and $0 + <TARGET>_URL landed in T3.6; "read-only scope enforced by the datastore itself" requires the CNPG driver to materialize roles/grants (T3.4).
