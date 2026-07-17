---
id: E3
title: E3 · Projects, environments, estimates, Postgres provisioning
epic: E3
status: stub
phase: MVP
priority: critical
sprint: 3
estimate: 9ew
deps: [E2]
issue: 4
labels: [Backend, Platform, Database]
module: M4 Provisioning
contexts: []
owner: founders
---

## Scope

Estimate-before-provision made real: estimate engine (one arithmetic, ADR-025), service CRUD gated on `estimate_id`, Neon tenant/timeline provisioning via the reconciler, secrets, bindings (incl. bind-to-external-host), metering emitters.

**Exit:** `steloit db create` → estimate → `--yes` → ready end-to-end on cell-0; canon arithmetic invariants imported and green. (implementation-plan §5 E3)

> Epic tracking file. Work items live beside it; shared design goes here as the epic starts.
