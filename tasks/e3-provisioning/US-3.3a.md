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
  - tasks/e3-provisioning/US-3.3b.md   # the half this could not reach
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
- [x] The namespace carries default-deny NetworkPolicy, ResourceQuota, LimitRange
  (D7). Egress/ingress defaults stated, not implied.
- [x] A service in a brand-new environment converges with no hand-created
  namespace; proven without the runbook's preflight.
- [ ] Deleting an environment removes the namespace (and nothing else's).

## Related

US-3.3 (found it) · INF-001 D7 · `infra/modules/cnpg`

## Decision — agent-side, and it is not a preference

**The control plane has NO Kubernetes dependency.** `services/api/go.mod` contains
no `k8s.io/*` at all, because the two-plane split (D6, frozen by ADR-0001) is that
the control plane writes desired state and the cell-agent converges it. Creating
the namespace control-plane-side would mean giving the control plane cluster
credentials and a kube client — an architecture delta, and ADR-0040 says a delta
needs evidence from implementation or a customer. There is none; there is just a
missing namespace.

Agent-side is also **level-triggered for free**: a namespace or policy deleted out
from under us is recreated on the next converge. A create-once call at environment
creation would never notice.

## What was actually missing — checked in all three places

| where | what it does with the namespace |
|---|---|
| control plane | **derives the name only** (`namespaceForEnv`), never creates |
| cell-agent | **consumes** it as `driver.Spec.Namespace`; drivers render *into* it |
| terraform | creates `cnpg-system` and `control-plane` — nothing per-env |

So the task's claim held exactly. The live e2e worked because a runbook ran
`kubectl create ns` in preflight.

## Two things this uncovered

**1. The applier could not express a cluster-scoped resource.** `resourcePath`
hardcoded `/namespaces/<ns>/<plural>/<name>` and rejected an empty namespace — but
a Namespace lives at `/api/v1/namespaces/<name>` and *has* no namespace. Nesting it
is a 404 at apply time on a live cluster: the exact class the explicit `plurals`
map exists to prevent, one level up. `clusterScoped` is now explicit for the same
reason `plurals` is — guessing scope from a kind name is how a manifest silently
applies to the wrong path.

**2. Test fixtures still used the pre-ADR-0012 namespace shape.** Eleven fixtures
in `internal/render` used `acme--prod` — the `proj--env` form ADR-0012 superseded
*because it collided across orgs and shared the very isolation boundary it was
meant to create*. `tenancy.Render` rejects it, which is how they surfaced.
Corrected to `env-9f3c1a2b` (`sanitize(env_id)`, ADR-0012 §36). `internal/kube`'s
eleven uses are untouched and still pass: `resourcePath` legitimately takes any
namespace string.

## D7, and why each object is there

Six manifests, ordered — the Namespace first, because everything after it is
namespaced:

- **`default-deny-all`** — `podSelector: {}` selects every pod; naming a policyType
  with **no rules** is what denies it. Ingress and Egress are *separate* denials:
  a policy with only Ingress leaves egress wide open, and egress is the half that
  reaches the metadata server and other tenants.
- **`allow-dns-egress`** — scoped to kube-dns, not opened to the cluster. Without
  it every pod fails to resolve anything including its own database, and a
  default-deny that breaks the product gets reverted rather than shipped.
- **`allow-same-namespace`** — a service must reach its own database; the
  default-deny forbids even that until this allows it.
- **`env-quota` + `env-limits`** — a **pair**. A quota constraining
  requests/limits *rejects* any pod that omits them, so the LimitRange defaults
  are what make the quota usable rather than a wall.

## Negative evidence — six mutations, each RED

| mutation | |
|---|---|
| drop the default-deny NetworkPolicy | RED |
| default-deny covers Ingress only (egress open) | RED |
| drop the LimitRange (quota then rejects ordinary pods) | RED |
| render tenancy but never APPLY it | RED |
| apply the namespace LAST (ordering broken) | RED |
| route Namespace as namespaced (404 on a live cluster) | RED |

Plus the applier's own path tests, positive **and** negative: a cluster-scoped
kind ignores a supplied namespace rather than nesting, and a *namespaced* kind
with no namespace is refused rather than silently routed cluster-scoped.

## Two tests were made stronger rather than adjusted to pass

`TestDeletingAFailedServiceLeavesNothingBehind` began failing because the env
objects survive a service teardown — which is **correct**: the namespace belongs
to the environment, and other services live in it. Rather than exempt them, the
test now pins **both** directions: no *service* object may remain, and the
*environment* objects **must**. The env set is DERIVED from `tenancy.Render`, so a
policy added there is covered without anyone remembering.

`TestCNPGRendererAppliesRenderedManifests` asserted an exact count of 2 and
indexed positions. It now asserts by **kind** and pins the ordering invariant on
the real applied slice — a count breaks whenever either renderer legitimately
grows an object and then gets "fixed" by bumping the number.

## NOT done — AC 4, deliberately

**"Deleting an environment removes the namespace (and nothing else's)"** is not
implemented. There is no environment-deletion path in the agent to hook: teardown
today is per-service (`svc.Status == "deleting"`), and env deletion would need the
control plane to express it in desired state first. Implementing it here would
mean inventing that contract, which the task does not authorise. **Filed as a
follow-up rather than silently skipped.**
