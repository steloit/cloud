---
id: US-3.17
title: "The HA standby has no independent failure domain: one storage node, soft anti-affinity"
epic: E3
status: ready
phase: MVP
priority: high
sprint: 3
estimate: 0.5ew
deps: [US-3.16]
issue: 0
labels: [Infrastructure, Reliability]
module: M3 Provision
contexts: [provisioning]
files:
  - services/cell-agent/internal/driver/cnpg/templates/**
  - infra/envs/**
  - docs/adr/**
  - tasks/e3-provisioning/US-3.17.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -count=1 ./..."
  - "terraform fmt -check -recursive infra"
owner: agent
---

## The defect

US-3.16 made `ha: true` render two instances. Two instances on **one node** is
not high availability.

Three facts compose:

- `infra/envs/{dev,cell0}/main.tf` set `storage_node_count = 1`, a fixed count
  (`gke-cell/main.tf`: "stateful — never autoscaled").
- `cluster.yaml.tmpl` pins `affinity.nodeSelector: pool: db-storage`.
- CNPG's `podAntiAffinityType` defaults to **`preferred`** — a soft, weight-100
  term, not a constraint.

So both instances schedule onto the sole `db-storage` node, with **no signal**:
no Pending pod, the phase reaches "Cluster in healthy state", and the renderer
reports `ready`. Pod-level failover works; **node loss takes primary and standby
together**, which is the failure HA is bought for.

Also, CNPG creates two PodDisruptionBudgets by default, so draining that one node
is then blocked by the primary's PDB.

## Why it is filed rather than fixed in US-3.16

US-3.16 closed "the $19 buys nothing". This is "the $19 buys less than it looks
like" — a different claim, needing an infrastructure change (node count, and
therefore cell cost) rather than a renderer change. Bundling them would have put
a capacity decision inside a billing fix.

## The options, with their costs

1. **`podAntiAffinityType: required` + `storage_node_count >= 2`.** Honest HA.
   Costs a second `n2-standard-8` per cell that sells HA (~$390/mo on-demand at
   `cell0`'s shape) whether or not anyone buys HA.
2. **`required` only.** An HA service simply will not schedule on a one-node
   pool — loud instead of silently degraded. Cheap, and arguably correct: it
   refuses to pretend. But it makes HA unsellable until (1) lands.
3. **Keep `preferred`, and say so.** Record that HA today is pod-level only, and
   that node-level HA arrives with a multi-node storage pool. Costs nothing and
   lies to nobody, but the frame says "high availability" without qualification.

**Recommended: (2) now, (1) when a cell first sells HA.** A cluster that refuses
to schedule is a page; a standby sharing a node with its primary is a false sense
of safety that surfaces only during the incident it was bought for.

## Acceptance criteria

1. The chosen option is recorded — an ADR if it changes cell capacity, otherwise
   a comment in `cluster.yaml.tmpl` in the manner of the existing decision
   comments there.
2. If anti-affinity becomes `required`: a test asserts the rendered Cluster sets
   it, and a mutation removing it goes red.
3. If node count changes: `terraform plan` evidence, and the cost delta stated.
4. Whatever is chosen, an HA service on a one-node pool must not report `ready`
   while silently co-resident. Either it does not schedule, or the limitation is
   documented where a customer-facing promise can be checked against it.

## Notes

Found by the US-3.16 review. Related and lower: replication is asynchronous
(`minSyncReplicas`/`maxSyncReplicas` unset), so a failover can lose transactions
up to replication lag. The frame promises failover, not zero loss, so nothing is
mis-sold — but "HA" reads as durability and the RPO story (ADR-0007's 300s WAL
bound) should be stated beside it.
