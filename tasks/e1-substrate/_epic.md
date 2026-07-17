---
id: E1
title: E1 · Platform substrate — Cell-0
epic: E1
status: stub
phase: MVP
priority: critical
sprint: 1
estimate: 8ew
deps: [E0]
issue: 2
labels: [Platform, Infrastructure]
module: M1 Substrate
contexts: []
owner: founders
---

## Scope

The data plane's skeleton, shaped per D6/D7 from day one, sized per 'cheap on capacity' (INF-001). Zonal GKE + gVisor + namespaces + Neon fleet N=1 + reconciler agent + metering skeleton.

**Exit:** spike ADR recorded; `terraform apply` from empty → working cell; first tenant DB created/branched/destroyed by hand; metering events from a test pod. (implementation-plan §5 E1)

> Epic tracking file. Work items live beside it; shared design goes here as the epic starts.
