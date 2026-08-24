---
id: US-3.3b
title: "Deleting an environment does not remove its namespace — there is no env-teardown path at all"
epic: E3
status: ready
phase: MVP
priority: high
sprint: 4
issue: 0
labels: [Platform, Security]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/api/internal/provisioning/**
  - services/cell-agent/internal/**
  - tasks/e3-provisioning/US-3.3b.md
verify:
  - "deleting an environment removes its namespace and nothing else's"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -race ./..."
owner: agent
---

## Goal

US-3.3a made the cell-agent create an environment's namespace on every converge
(its D7 isolation objects were withdrawn to US-3.3c). **Nothing removes it.**

That was US-3.3a's AC 4 ("Deleting an environment removes the namespace — and
nothing else's"), and it was deliberately not implemented there. This task says
why, so the gap is owned rather than inherited.

## Why it could not be done in US-3.3a

**There is no environment-deletion path to hook.** Teardown in the agent is
per-SERVICE: `Converge` branches on `svc.Status == "deleting"` and tears down that
service's objects. An environment is not a service and never reaches the renderer.

So this needs the control plane to express environment deletion in desired state
first — a contract that does not exist. Inventing it inside US-3.3a would have
been a new control-plane↔agent contract smuggled into a namespace-creation task.

## What is actually leaked today

A deleted environment leaves behind **its namespace**. Not customer data — the
CNPG cluster and its PVCs are torn down by the per-service path — but the
namespace is not free:

- it is a live tenant boundary for an environment that no longer exists, which is
  a confusing thing to find in a cluster during an incident.

As merged, US-3.3a renders the namespace and nothing else: the D7
NetworkPolicies, ResourceQuota and LimitRange were withdrawn to US-3.3c/d/e, so
the leak is **one object per environment, not six**. US-3.3c will widen it again
— write the teardown against `tenancy.Render`'s output rather than a typed-out
list, exactly as `envObjectKeys` in `delete_all_test.go` already does.

**`Delete` could not address a Namespace at all** until round 3: `plurals` had the
kind but `Delete` hardcoded the CNPG apiVersion, so `Delete(ns,"Namespace",ns)`
built `/apis/postgresql.cnpg.io/v1/namespaces/<name>`, got a 404, and returned
`nil` — reporting success while the namespace survived. There is now an
`apiVersions` map and `TestDeleteRoutesEveryKindToItsOwnAPIGroup` pinning
`Namespace` → `/api/v1/namespaces/<name>`.

## Design constraint inherited from US-3.3a

The delete must be **narrower than the create**. `tenancy.Render` is applied on
every converge for the whole environment; deletion must remove the namespace *of
that environment only*.

**Do not delete by `steloit.dev/environment-id`.** US-3.3a shipped that label set
to `TrimPrefix(namespace, "env-")`, which yields `9f3c1a2b` for the id
`env_9f3c1a2b` — it named nothing the control plane knows, and it has been
removed. The agent is not given the environment id: the desired doc carries the
namespace only. Either delete by the namespace NAME (which the control plane
resolved and therefore knows exactly), or add `environment_id` to the desired doc
first and label from that — one derivation, not two.

Deleting the namespace deletes everything in it, so ordering matters in reverse:
the per-service teardown should already have run.

## Acceptance criteria

- [ ] The control plane expresses environment deletion in desired state (or an
  explicit alternative is recorded with its reasoning).
- [ ] The agent removes the namespace for that environment and no other. Proven
  with two environments present, not one.
- [ ] A service teardown still leaves the environment's objects intact — the
  invariant US-3.3a pinned must not regress.
- [ ] Mutation-verified: deleting the wrong environment's namespace, and deleting
  none, both fail a test.

## Found by

US-3.3a (2026-08-23), which implemented the create half and recorded this as the
half it could not reach.
