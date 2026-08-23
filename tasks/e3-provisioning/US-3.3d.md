---
id: US-3.3d
title: "A managed Postgres declares no resources, so any LimitRange default silently becomes its hard cap"
epic: E3
status: ready
phase: MVP
priority: high
sprint: 4
issue: 0
labels: [Platform]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/cell-agent/internal/driver/cnpg/**
  - services/cell-agent/internal/render/**
  - tasks/e3-provisioning/US-3.3d.md
verify:
  - "for every shape the catalog sells, the rendered Cluster declares resources matching the sold shape"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -race ./..."
owner: agent
---

## Goal

The Cluster we render must declare the resources the customer paid for.

## Why — US-3.3a's security review

`services/cell-agent/internal/driver/cnpg/templates/cluster.yaml.tmpl` contains no
`resources:`, no `memory` and no `cpu` at all. Two consequences, and the second is
the one that bit:

1. **A customer buying a larger shape gets a pod with no resource floor.** The
   estimate resolves `memory_mb` (`services/api/internal/estimates/engine.go:167`
   — default 1024) and prices it; nothing carries it to the pod. Scheduling and
   eviction therefore ignore what was sold.
2. **Any LimitRange in the namespace silently becomes the hard cap.** US-3.3a
   tried to ship `default: {cpu: 500m, memory: 512Mi}` as part of D7. With no
   `resources:` on the Cluster that default is applied at pod admission to every
   managed Postgres — *existing ones included*, since SSA reapplies each converge
   and it bites on the next restart or failover. A "standard" Postgres OOMKilled
   at 512Mi is a product regression arriving through an isolation task.

This is why the LimitRange was withdrawn from US-3.3a rather than shipped. It is
**not** blocked on NetworkPolicy enforcement (US-3.3c) — a LimitRange is enforced
by the API server at admission, needs no CNI, and the fix belongs with the CNPG
driver rather than with an isolation task.

## Acceptance criteria

1. The rendered Cluster declares `resources.requests` and `resources.limits`
   derived from the sold shape, for every product the driver renders.
2. A test asserts, for **each** shape in the catalog, that the rendered values
   match the shape rather than a constant — a single-shape test passes on a
   hardcoded literal.
3. Only then, a LimitRange whose defaults sit *underneath* the declared
   resources, with a test that no catalog shape is capped below what it sold.
4. The golden fixtures in `internal/driver/cnpg/testdata/` are regenerated and
   the diff is read, not just accepted.

## Read first

- `services/cell-agent/internal/driver/cnpg/templates/cluster.yaml.tmpl`
- `services/api/internal/estimates/engine.go` (`memory_mb` resolution)
- commit `7e94f26` — the withdrawn LimitRange, and why its defaults were wrong
