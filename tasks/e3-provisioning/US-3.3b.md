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

US-3.3a made the cell-agent create an environment's namespace and its D7
isolation objects on every converge. **Nothing removes them.**

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

A deleted environment leaves behind: the namespace, three NetworkPolicies, a
ResourceQuota and a LimitRange. Not customer data — the CNPG cluster and its PVCs
are torn down by the per-service path — but the namespace is not free:

- it holds a `ResourceQuota` that counts against nothing, and
- it is a live tenant boundary for an environment that no longer exists, which is
  a confusing thing to find in a cluster during an incident.

## Design constraint inherited from US-3.3a

The delete must be **narrower than the create**. `tenancy.Render` is applied on
every converge for the whole environment; deletion must remove the namespace *of
that environment only*. Deleting by label selector (`steloit.dev/environment-id`)
is safer than by name-prefix, because the label is set by the agent and the name
is derived — two representations, and the label is the one the agent owns.

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
