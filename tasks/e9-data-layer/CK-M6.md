---
id: CK-M6
title: "Data layer complete (Valkey + external Bindings)"
epic: E9
status: stub
phase: V1
priority: high
sprint: 10
estimate: 0ew
deps: [E9-1, E9-3, E9-6]
issue: 190
labels: [milestone-checkpoint]
module: M4 Provisioning
contexts: [provisioning, api-conventions]
files: []
verify: []
owner: agent
---

## Goal

Data layer complete (rescoped by ADR-0004): Valkey optional service + Storage/AI Bindings live.

## Summary

**Exit criteria:**
- [ ] Valkey provisionable as an optional service (provision-on-add, idle-suspend) with estimates
- [ ] Storage Binding and AI Binding live on the external-Binding mechanism (no proxy, no egress borne)
- [ ] Canon parity for the V1 data layer (E9-6)
- [ ] Queue is a documented Postgres capability deferred to V2 (E9-4); no separate queue service exists

> **Stub** — enrich to `ready` before starting. Plan: docs/plan/implementation-plan.md §5 E9.
