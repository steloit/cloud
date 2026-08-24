# ADR-0017 — Raw Kubernetes manifests use `alekc/kubectl`, not `kubernetes_manifest`

**Status:** Accepted (T1.4a, 2026-08-24) · Toolchain delta under ADR-0001

## Decision

Every raw manifest in `infra/` is applied with **`kubectl_manifest`** from
**`alekc/kubectl` `~> 2.4`**. `hashicorp/kubernetes`'s `kubernetes_manifest` is
not used, and a test fails if it reappears.

The `kubernetes` provider itself STAYS — `kubernetes_namespace` and other
schema'd resources are fine and preferred where they exist. This is about the
one resource type that needs the API at plan time.

## Why

`kubernetes_manifest` resolves its GroupVersionKind against the live cluster at
**plan** time, because it builds Terraform's type information from the API's
OpenAPI schema. Two consequences on a from-zero apply, both fatal:

1. the cluster does not exist yet — this apply creates it;
2. for a custom resource the CRD does not exist yet either, since the Helm
   release that installs it is in the same apply.

```
Error: API did not recognize GroupVersionKind from manifest (CRD may not be installed)
```

`depends_on` does not help: validation runs before dependency resolution. This
is acknowledged and still open upstream
([#1367](https://github.com/hashicorp/terraform-provider-kubernetes/issues/1367),
[#2597](https://github.com/hashicorp/terraform-provider-kubernetes/issues/2597)).

The cost was a documented four-step `-target` bootstrap — precisely the
"exceptional circumstances only" use Terraform warns about, promoted to the
normal way to stand up a cell.

**Nothing else about the one-pass apply was broken.** Creating a cluster and
deploying into it in a single apply is supported: the Kubernetes and Helm
providers are simply not initialised until apply time, and HashiCorp's own GKE
example does exactly this. Only `kubernetes_manifest` reaches out during plan.

## Alternatives considered

| option | why not |
|---|---|
| **Two roots / two-stage apply** (`envs/dev-operator` + `envs/dev`) | honest about ordering, but splits one cell across two states and two applies, and the task's own AC is a single pass. It also does not remove the constraint — it institutionalises it. |
| **Move the control-plane CNPG cluster to the cell-agent** — the task called this "arguably the most architecturally consistent with D9" | **It is a bootstrap cycle, and that is checkable.** The agent gets desired state by polling the control-plane API (`agent/client.go`), and the API refuses to start without its database (`cmd/api/main.go` runs `db.Migrate` then `db.Connect` before serving). The control-plane database therefore cannot be provisioned by the thing that needs it to exist. Rejected on evidence, not taste. |
| **`gavinbunney/kubectl`** | the provider every tutorial names, and effectively unmaintained; the community has moved to the `alekc` fork, which also supports a state move from it. |
| **CNPG's `cluster` Helm chart instead of raw YAML** | would work, but `infra/k8s/control-plane/cnpg-cluster.yaml` is ground truth that `parity_test.go` asserts the agent's renderer against. Replacing it with chart values moves that truth into a third-party chart's templating and breaks the binding. |
| **`null_resource` + `local-exec kubectl apply`** | a workaround with no state, no drift detection and no destroy. |

## The provider must load LAZILY, or the fix does not work

`alekc/kubectl` **configures at plan time** and errors when `host`/
`cluster_ca_certificate` are still unknown — which is exactly the from-zero case
this ADR exists to fix. The first version of this change therefore replaced one
plan-time failure with another, and review caught it. Measured, with the provider
block copied verbatim and its host taken from a not-yet-applied resource:

```
without lazy_load:  Plan: 1 to add, then
  Error: invalid provider configuration: default cluster has no server defined
with lazy_load:     Plan: 2 to add, clean
```

`hashicorp/kubernetes` and `hashicorp/helm` tolerate an unconfigured client at
plan; this one does not. So `lazy_load = true` is not a tuning knob — it is half
the fix. Terraform re-configures providers during the apply walk, where the
values are known, so nothing is swallowed.

`apply_retry_count = 3` for the same reason: 2.4.1's default is a SINGLE attempt,
and the CNPG `Cluster` is applied moments after the operator's chart returns. A
transient CRD-registration or admission-webhook window would fail the whole apply
with no retry — the difference between a one-pass apply that works and one that
is lucky.

**Neither property can be seen by anything else.** `terraform validate` never
configures providers, and the env `terraform test` suites mock this provider
wholesale, which is what makes them pass. `TestTheKubectlProviderLoadsLazily` is
the only instrument left, and it is a text guard that says so.

## Consequences

- One more provider to keep current. It is pinned `~> 2.4` and locked for
  `linux_amd64` and `darwin_arm64` in all three lock files.
- **`~> 2.4`, not `~> 3.0`.** The project's README says `3.0`; the registry
  publishes 3.0.0 only as `beta2`/`beta3`, so that constraint resolves to
  nothing. Verified against `registry.terraform.io`, not the README — the
  version that ships is 2.4.1.
- `kubectl_manifest` does not type-check manifests at plan. That is the point,
  and the trade is real: a malformed manifest now fails at apply rather than
  plan. The YAML under `infra/k8s/` is parsed in CI by the "k8s manifests parse"
  step, which is where that check belongs anyway — it covers the files whether
  or not Terraform is involved.
- **Destroy blocks less than it did.** `kubectl_manifest`'s `wait` defaults to
  false, so deleting the CNPG `Cluster` is fire-and-forget where
  `kubernetes_manifest` blocked until the object was gone. This is believed
  mitigated — `depends_on` puts `kubernetes_namespace.control_plane` after the
  Cluster on destroy, and the kubernetes provider does block on namespace
  termination, which cannot finish while the Cluster's finalizers and PVCs are
  outstanding. That is an INFERENCE, not a measurement: it needs a live cluster
  to confirm, and `wait = true` on the Cluster would make it a guarantee. Stated
  rather than assumed.
- The CNPG chart annotates its CRDs `helm.sh/resource-policy: keep`, so a
  `helm uninstall` does not cascade-delete CRDs or the Clusters built on them.
  The usual reason to split CRD installation out of the chart does not apply.

## Not verified here

**A real from-zero `apply` has not been run**, because it requires creating a GKE
cluster in `steloit-dev` and that is a spend decision, not an implementation one.
There are currently zero clusters in the project (verified). What IS verified:
`terraform validate` passes for both envs and both modules with the new provider,
the three lock files resolve on linux and darwin, and no `kubernetes_manifest`
remains. T1.4a's AC 2 ("verified by actually destroying and re-applying") is
explicitly still open and named as such in the task.
