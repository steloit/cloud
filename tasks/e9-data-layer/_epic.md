---
id: E9
title: E9 · Data layer completion — Valkey, Object storage, Queue
epic: E9
status: stub
phase: V1
priority: high
sprint: 8
estimate: 8ew
deps: [E3]
issue: 10
labels: [Platform, Backend, Database]
module: M4 Provisioning
contexts: []
owner: founders
---

## Scope

Per GOV-002 v0.5: Valkey (per-project-env pods, D5), object storage (proxied GCS, D4; Steloit content-domain URLs, A1.4/A2.4), queue (A3.1 WAL-signal design review FIRST — scale-to-zero must survive; NATS is last resort). Postgres is the pioneer; these instantiate the same anatomy. (implementation-plan §5 E9)

> Epic tracking file. Work items live beside it; shared design goes here as the epic starts.
