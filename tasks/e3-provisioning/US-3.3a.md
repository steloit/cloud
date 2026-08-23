---
id: US-3.3a
title: "Nothing creates the env namespace (nor its D7 default-deny policies)"
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
  - services/cell-agent/internal/**
  - tasks/e3-provisioning/US-3.3a.md
  - tasks/e3-provisioning/US-3.3b.md   # the half this could not reach
  - tasks/e3-provisioning/US-3.3c.md   # the half the security review withdrew
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go build ./... && go vet ./... && go test -race ./..."
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && test -z \"$(gofmt -l .)\""
owner: agent
---

## Goal

An environment's Kubernetes namespace — and the isolation D7 requires — must be
created by the system, not by a human running a runbook.

## Why — found in US-3.3's review

US-3.3 renders into `desired.namespace`, but **nothing creates that namespace**:
not the control plane, not the agent, not terraform (`module.cnpg` creates only
`control_plane`). The live e2e worked because the runbook did
`kubectl create ns` in preflight. On a real cell a genuinely new project/env
would 404 on first apply — and, until US-3.3's ErrNotConverged fix, would have
retried forever silently.

D7 also requires the namespace to carry **default-deny NetworkPolicies, a
ResourceQuota and a LimitRange** — the tenant isolation boundary. None exist.
US-3.3 fixed the namespace *collision* (env-id derived) but not its *creation*
or *policing*, so isolation is nominal until this lands.

## Acceptance criteria

- [x] Creating an environment creates its namespace (control-plane-side at env
  creation, or agent-side as part of converge — decide and record why).
- [ ] The namespace carries default-deny NetworkPolicy, ResourceQuota, LimitRange
  (D7). **Withdrawn to US-3.3c** — implemented, then removed before merge because
  the security review showed each object is inert or harmful. See below.
- [x] A service in a brand-new environment converges with no hand-created
  namespace; proven without the runbook's preflight.
- [ ] Deleting an environment removes the namespace (and nothing else's).
  **Filed as US-3.3b** — no env-teardown path exists to hook.

## Related

US-3.3 (found it) · INF-001 D7 · US-3.3b · US-3.3c · `infra/modules/cnpg`

## Decision — agent-side, and it is not a preference

**The control plane has NO Kubernetes dependency.** `services/api/go.mod` contains
no `k8s.io/*` at all, because the two-plane split (D6, frozen by ADR-0001) is that
the control plane writes desired state and the cell-agent converges it. Creating
the namespace control-plane-side would mean giving the control plane cluster
credentials and a kube client — an architecture delta, and ADR-0040 says a delta
needs evidence from implementation or a customer. There is none; there is just a
missing namespace.

Agent-side is also **level-triggered for free**: a namespace deleted out from
under us is recreated on the next converge. A create-once call at environment
creation would never notice.

## What was actually missing — checked in all three places

| where | what it does with the namespace |
|---|---|
| control plane | **derives the name only** (`namespaceForEnv`), never creates |
| cell-agent | **consumes** it as `driver.Spec.Namespace`; drivers render *into* it |
| terraform | creates `cnpg-system` and `control-plane` — nothing per-env |

So the task's claim held exactly. The live e2e worked because a runbook ran
`kubectl create ns` in preflight.

## D7 was implemented and then WITHDRAWN — this is the substance of the task

The first version of this branch shipped all six D7 objects. The security review
found the isolation boundary it created does not exist, and that two of the four
objects are actively harmful. All four findings are in **US-3.3c**; in short:

| object | enforced today? | why it did not ship |
|---|---|---|
| Namespace + labels | **yes** | shipped — this is the real gap |
| NetworkPolicies ×3 | **no** | `infra/modules/gke-cell` is GKE Standard with no `network_policy` and no `ADVANCED_DATAPATH`. The API server stores them; nothing drops a packet. |
| NetworkPolicies ×3 | — | and when enforcement *is* on, the allow-set fences CNPG off the metadata server (Workload Identity), GCS (WAL archiving) and the apiserver (instance manager) — the first Postgres pod never reaches ready |
| LimitRange | **yes** | `cluster.yaml.tmpl` declares no `resources:`, so `default: 512Mi` becomes the hard cap on every managed Postgres, existing ones included |
| ResourceQuota | **yes** | unsourced, plan-independent envelope; `persistentvolumeclaims: 16` caps an env at ~5 HA clusters with no error surface. Founder-owned. |

Shipping the policies unenforced would have put a green suite and a "D7 done"
behind a boundary that is not there. `tenancy.Render`'s package doc records why
each is absent, and `TestTheD7PolicyObjectsAreDeliberatelyNotRenderedYet` makes
re-adding one impossible without reading it.

## Two things this uncovered

**1. The applier could not express a cluster-scoped resource.** `resourcePath`
hardcoded `/namespaces/<ns>/<plural>/<name>` and rejected an empty namespace — but
a Namespace lives at `/api/v1/namespaces/<name>` and *has* no namespace. Nesting it
is a 404 at apply time on a live cluster: the exact class the explicit `plurals`
map exists to prevent, one level up. `clusterScoped` is now explicit for the same
reason `plurals` is.

**2. Test fixtures still used the pre-ADR-0012 namespace shape.** Eleven fixtures
in `internal/render` used `acme--prod` — the `proj--env` form ADR-0012 superseded
*because it collided across orgs and shared the very isolation boundary it was
meant to create*. `tenancy.Render` rejects it, which is how they surfaced.
Corrected to `env-9f3c1a2b` (`sanitize(env_id)`, ADR-0012 §36). The review
independently diffed all eleven and confirmed none asserted anything *about* the
`proj--env` shape, so no coverage was destroyed.

## A wrong label was removed rather than kept

US-3.3a first set `steloit.dev/environment-id` to
`strings.TrimPrefix(namespace, "env-")`. The real id is `env_9f3c1a2b` and
`k8sNamespace` maps `_`→`-`, so the label read **`9f3c1a2b`** — a value that names
nothing the control plane knows, and US-3.3b's stated design was to delete
namespaces *by that label*. The agent is not given the environment id; the desired
doc carries the namespace only. Absent beats wrong: the namespace NAME already
identifies the environment. A test pins the absence and says what to do instead
(put the id in the desired doc first).

## Apply now refuses a cross-namespace write locally

`Apply` routes by the **caller's** namespace argument and never read
`metadata.namespace`, so a manifest declaring another namespace was written into
the caller's — fail-closed only because the API server answers 400, a tenant
boundary enforced a network hop away on a path nothing pinned. It is refused
before the request is built, in both directions (a namespaced object declaring a
foreign namespace; a cluster-scoped object declaring any).

## Negative evidence — thirteen mutations, each RED

Applied in `cp -R` copies (AGENTS.md), each with an assert-the-mutation-applied
guard, because a mutation that does not apply reads as a green hole.

| mutation | |
|---|---|
| render the namespace but never APPLY it | RED |
| apply the namespace LAST (ordering broken) | RED |
| route Namespace as namespaced (404 on a live cluster) | RED |
| `clusterScoped["Cluster"]`/`["ScheduledBackup"]` = true | RED |
| remove the cluster-scoped branch entirely | RED |
| a namespaced kind with no namespace is accepted | RED |
| the CNPG Cluster rendered into `env-victim` | RED |
| `steloit.dev/environment-id` returns with the truncated value | RED |
| a NetworkPolicy is re-added to `tenancy.Render` | RED |
| namespace validation relaxed back to a prefix check | RED |
| the cell label value left unvalidated (YAML injection) | RED |
| `Render` refuses everything (negatives alone would pass) | RED |
| the Apply mismatch guard, each arm separately + refuse-everything + runs-too-late | RED |

## One test was made stronger. One was weakened, and the review caught it

`TestDeletingAFailedServiceLeavesNothingBehind` began failing because the env
objects survive a service teardown — which is **correct**: the namespace belongs
to the environment, and other services live in it. Rather than exempt them, the
test now pins **both** directions: no *service* object may remain, and the
*environment* objects **must**. The env set is DERIVED from `tenancy.Render`.

`TestCNPGRendererAppliesRenderedManifests` was **weakened**, and the branch
claimed the opposite. Its D8 assertion went from `objs[0]` — the CNPG Cluster
specifically — to a substring search over the *concatenated* applied set. Every
tenancy manifest also contains `namespace: env-9f3c1a2b`, so the assertion was
answered by a different object than the one it names: mutating
`driver.Spec.Namespace` to `env-victim`, rendering one tenant's database into
another tenant's namespace, left the test **GREEN**. It now locates the object
containing `kind: Cluster` and asserts against that one.

## NOT done — AC 2 and AC 4

**AC 2 (D7 policies)** → **US-3.3c**, priority `critical`. Enforcement, the CNPG
allow-set, shape-derived resources and a founder-owned quota envelope are one
change; any one alone is a no-op or an outage.

**AC 4 (env deletion)** → **US-3.3b**. There is no environment-deletion path in
the agent to hook: teardown today is per-service (`svc.Status == "deleting"`), and
env deletion needs the control plane to express it in desired state first.

**As merged, D7 is a namespace and nothing else.** An environment is a name and a
quota-free, policy-free namespace; deleting an environment leaks it.
