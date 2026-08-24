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
