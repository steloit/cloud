---
id: US-3.3b
title: "Deleting an environment does not remove its namespace — there is no env-teardown path at all"
epic: E3
status: done
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

## Outcome

An environment's namespace is now torn down, through a contract that did not
exist: `/desired` carries the environments whose namespace must go, and a new
`POST /reconcile/{cell}/environments/{env}/teardown` is how the cell confirms it.

### The gate is the feature

Everything else here is plumbing. The one decision that matters is **when** an
environment may be advertised, because deleting a namespace deletes everything in
it — including a CNPG cluster that is still terminating, which would destroy the
database before US-3.5's final backup.

`DeleteEnvironment` only requires every service to have **reached** `deleting`.
That is not the same as torn down, and the window between them is exactly the
dangerous one. So the gate lives in `ListEnvironmentTeardownsForCell` and asserts
the stronger condition:

```sql
AND NOT EXISTS (
    SELECT 1 FROM services s
    WHERE s.env_id = e.id
      AND (s.status <> 'deleting' OR s.observed_generation < s.generation)
)
```

`observed_generation >= generation` is what makes it real: the cell converged the
teardown and reported it. It is deliberately **not** row absence — nothing in the
tree deletes a service row, so "gone" is a status plus a converged generation
(US-3.3h made `deleting` + `gone` converge, which is what lands it).

### Two columns, because two planes write them

`deletion_scheduled_at` (T4.7) is the customer asking; `torn_down_at` is the cell
confirming. A single status column could not distinguish "asked for" from "done",
and the two are written by different planes. The row is still never hard-DELETEd
— T4.7's reason stands (deployments reference envs `ON DELETE RESTRICT`, history
is immutable, DP1). This records the teardown, not the end of the record.

### What is deleted, and how it stays right

`tenancy.TeardownObjects` renders the real object set and keeps only the
**cluster-scoped** members. Everything namespaced dies with the namespace, so
deleting it by name would be redundant — and an explicit list is what goes stale
when US-3.3c widens the set.

Scope is read **from the rendered bytes** (does the object declare
`metadata.namespace`?), not from a second table. That is the same fact
`kube.Apply` enforces from the other side, so the two cannot disagree, and a kind
added to `Render` is covered automatically.

### The namespace is SENT, never re-derived

`provisioning.NamespaceForEnv` is exported and the control plane puts the
namespace on the wire. US-3.3a shipped a second, agent-side derivation — a label
set to `TrimPrefix(namespace, "env-")`, which yielded `9f3c1a2b` for the id
`env_9f3c1a2b` and named nothing the control plane knew. There is one derivation
now, and the agent does not own it.

### Review round 2: the gate's premise was false

The reviewer's blocker, and it is the one that mattered: **`gone` did not mean
gone.** `Converge`'s deleting arm issued its deletes and returned `gone` on the
next line, and `kube.Delete` treats any 2xx as success — which Kubernetes returns
the *moment* it accepts a delete, with finalizers and graceful termination still
pending, and a CNPG Cluster has both. So the gate's `observed_generation` half
established "the cell's delete was **accepted**", not "the workload is gone", and
the namespace was removed roughly one tick (~10s) later while pods were still
terminating.

The teardown now **observes the Cluster absent** before reporting `gone` (`""`
from `Observe` is a real 404); anything else is `ErrNotConverged`, so the row
stays outstanding and the next tick re-checks. That is what makes the gate's
predicate mean what it says. The Cluster's name comes from the objects just
deleted rather than being re-derived — observing a name the driver did not use
would 404 and read as absence.

**I also cited a contract that does not exist.** The comment justified the gate
by "US-3.5's final-backup contract"; `US-3.5` is `status: blocked`. That is the
second false citation in this session (US-3.3h cited US-3.11 for something its
ACs do not cover). Both are now corrected to claim only what is established.

### And a one-way latch

`MarkEnvironmentTornDown` guarded only on scheduled-and-not-yet-torn-down, so a
confirmation arriving after the environment stopped being eligible **latched** it.
`torn_down_at` has no reset, so that environment is never advertised again — and
when the service inside it is later deleted, its namespace leaks *forever*, which
is the leak this task exists to close, reintroduced through the back door. The
confirmation now carries the **same `NOT EXISTS` fence as the poll**.

Two guards that failed independently, so both are fixed: `CreateService` also
refuses an environment with `deletion_scheduled_at` set, mirroring
`CreateProject`'s org check. `DeleteEnvironment` enforced "nothing live is in
here" at *schedule* time and nothing preserved it afterwards.

### Three more

- **D10:** `torn_down_at` is a durable state change and the record that a tenant
  boundary was destroyed, and it emitted no spine event — the feed showed a
  teardown that starts (`env.deletion_scheduled`) and never finishes. It records
  `env.torn_down` now, only on the call that actually latched it.
- **The ordering test did not test ordering.** `len(reports) > 0 &&
  len(confirmed) > 0` is true in *either* order; the reviewer moved
  `tearDownEnvironments` above the service loop and the whole package stayed
  green. Both halves append to one sequence log now, and every report must
  precede every teardown.
- `kube.IsClusterScoped` was **dead code** whose comment described a design I
  replaced with YAML-derived scope. Deleted.

### Evidence

Eight mutations RED, baseline asserted green **before and after**:

| control plane | | agent | |
|---|---|---|---|
| the safety gate removed | RED | teardown deletes a different name | RED |
| gate ignores `observed_generation` | RED | teardown deletes nothing | RED |
| the poll ignores the cell | RED | teardown also deletes namespaced objects | RED |
| the confirmation guard removed | RED | | |

Round 2 added five more, each on a green baseline:

| mutation | |
|---|---|
| `gone` reported on ACCEPTANCE (no absence check) | RED |
| the confirmation's fence removed (the latch) | RED |
| the `CreateService` guard removed | RED |
| the `env.torn_down` spine event removed | RED |
| environments torn down BEFORE services converge | RED |

The gate mutations are only catchable against **real Postgres** (the condition is
SQL), so those run there: a `ready` service, then one mid-teardown
(`observed 1 < generation 2`), then converged — advertised only at the last step.

**One mutation SURVIVED and the guard was removed rather than tested.** An
explicit `ValidateNamespace` at the top of `TeardownEnvironment` failed no test
when deleted, because nothing can reach a `Delete` without going through
`TeardownObjects` → `Render`, which validates first. A guard nothing can
distinguish is not a guard; the comment now says where the validation actually
lives.

```
services/api         go test -count=1 -race ./...                      all ok (containers)
services/cell-agent  go build && go vet && go test -count=1 -race ./... ok
make gen-sql && make gen-go && make gen-ts                              re-running all three is a no-op
gofmt -l services/ packages/ apps/                                      clean
node scripts/spec-sync/validate.mjs                                     OK: 245 tasks
```

### Recorded, not fixed here

- **A renderer that cannot tear environments down logs an ERROR and leaves the
  work outstanding.** The alpha `AckRenderer` owns no environment-scoped objects,
  so `EnvironmentTeardowner` is a separate interface (the `BranchingDriver`
  pattern) rather than a method every renderer must stub. Confirming a teardown
  that never happened is the one failure that loses the namespace permanently, so
  the loop refuses to.
- **`DeleteEnvironment` still refuses only on `status <> 'deleting'`.** The
  stronger condition lives in the poll, which is the right place — but that means
  the API accepts a deletion the cell will not act on for a while, and the
  customer sees no signal in between. Surfacing teardown progress is not this
  task.
- The teardown is not driven end-to-end against a live cell, because there is
  none (`steloit-dev` has zero GKE clusters). Every layer is tested — the SQL
  gate against real Postgres, the route, the loop, and the rendered delete — but
  the seam between the agent and a real apiserver is exercised only by the fake.
