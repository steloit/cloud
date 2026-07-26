---
id: E9
title: "E9 · Data layer: Valkey (optional) + external Bindings (storage, AI)"
epic: E9
status: stub
phase: V1
priority: high
sprint: 8
estimate: 4ew
deps: [E3]
issue: 10
labels: [Platform, Backend, Database]
module: M4 Provisioning
contexts: []
owner: founders
---

## Scope

Rescoped by ADR-0004/A5 — build only differentiated products; commodity becomes a Binding or a Postgres capability:
- **Valkey** (E9-1): optional cache service — provision-on-add, idle-suspend, quotas (A5.1). Never default.
- **External-provider Bindings** (E9-2 mechanism, E9-3 Storage + AI): the Binding primitive extended to external targets (A5.5). Storage Binding (connect S3/GCS/R2 — never proxy bytes/egress, A5.3) + AI Binding (govern an LLM provider — no proxy/routing/hard-caps, A5.4). Replaces managed object storage and any AI gateway.
- **Queue** (E9-4): deferred to V2 as a **Postgres capability** (pgmq inside the customer DB, consumed by a worker). No separate service, no NATS; risk R3 and the A3.1 apparatus retired (A5.2).
- Canon parity (E9-6); milestone CK-M6.

Postgres is the pioneer (21-playbooks); Valkey instantiates the anatomy; Bindings ride the existing Binding machinery (F3/T3.6).

> Epic tracking file. Work items live beside it; shared design goes here as the epic starts.
