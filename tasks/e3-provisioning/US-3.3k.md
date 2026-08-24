---
id: US-3.3k
title: "The agent has no deployment artifact, so it has no RBAC — and the namespace write it now needs is cluster-scoped"
epic: E3
status: ready
phase: MVP
priority: high
sprint: 4
estimate: 0.5ew
deps: []
issue: 0
labels: [Platform, Security]
module: M4 Provisioning
contexts: [provisioning]
files:
  - infra/k8s/**
  - infra/modules/**
  - services/cell-agent/internal/kube/**
  - tasks/e3-provisioning/US-3.3k.md
verify:
  - "the agent runs in-cluster under its own ServiceAccount and can create/patch namespaces and the D7 objects, proven on a live cell"
  - "a 403 on the tenancy objects degrades to 'this environment is not created', never 'this cell provisions nothing'"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -count=1 -race ./..."
owner: agent
---

## The gap (US-3.3c AC 4, carried out whole)

US-3.3a made the agent PATCH `/api/v1/namespaces/<name>` on every converge, and
US-3.3c added five NetworkPolicies beside it. Both are privileges the agent has
never been granted, because **there is no deployment artifact at all**: no
ServiceAccount, no ClusterRole, no Deployment anywhere in the repo
(`grep -rn "kind: ClusterRole\|kind: ServiceAccount\|serviceAccountName"` over
`*.yaml|*.tf|*.sh` returns nothing).

US-3.3c did NOT do this. Its live verification applied the rendered objects with
`kubectl` under a human credential, which proves the POLICY and says nothing
about the agent's permission to apply it. Granting a ClusterRole with no
Deployment to attach it to would have been an artifact nothing uses.

## Why it is high priority

`Apply` returns on the FIRST error and the Namespace is first in the batch, so a
403 there **halts convergence for every service on the cell** — including
services in namespaces that already exist and were converging fine. The blast
radius of the missing grant is the whole cell, not the new environment.

## Acceptance criteria

1. The agent has a ServiceAccount, a ClusterRole and a Deployment, applied by
   Terraform (the `datapath-policy` module is the precedent for a small module
   that applies cluster objects).
2. The ClusterRole grants cluster-scoped `create`/`patch` on `namespaces`, and
   namespaced rights on the D7 objects the agent now renders (`resourcequotas`,
   `limitranges`, `networkpolicies`) plus what it already applies (CNPG
   `clusters`, `scheduledbackups`, `volumesnapshots`).
3. **A 403 on the tenancy objects degrades, it does not halt.** Converge must
   report that environment as failed and continue with the rest of the cell.
   Asserted with a fake applier that 403s only the tenancy batch.
4. Proven on a live cell: the agent runs in-cluster under its own SA and
   converges a service end to end. Not `kubectl` under a human credential.
5. Least privilege is asserted, not assumed: the ClusterRole must NOT carry `*`
   verbs or `*` resources, and a test reads the rendered role to say so.

## Read first

- `services/cell-agent/internal/kube/client.go` — `NewInCluster`, `Apply`
- `services/cell-agent/internal/driver/tenancy/tenancy.go` — what needs granting
- `docs/dev/us33c-live-evidence.md` — what US-3.3c did and did not prove
