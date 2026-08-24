# Steloit infrastructure (T1.1 — e1-substrate-design.md §1)

Two GCP projects, one shape. **Shape lives in `modules/`, capacity in `envs/`**
(INF-001: "cheap on capacity, never on shape"). Every resource carries the
`cell_id` label (US-1.1).

| Env | Project | Region | Notes |
|---|---|---|---|
| `envs/dev` | steloit-dev (P1) | us-central1 | destroyable, duty-cycled, deletion_protection=false |
| `envs/cell0` | steloit-cell0 | asia-south1 | partner-facing (A1.7), deletion_protection=true |

## Gates

- **P1 (GCP project)** — nothing applies until the project ids exist; both are
  variables, never hardcoded. Local validation needs no credentials:
  `terraform -chdir=infra/envs/dev init -backend=false && terraform -chdir=infra/envs/dev validate`
- **P2 (content domain)** — the customer-content DNS zone (A2.4) is created only
  when `content_domain` is set.

## Bootstrap (one-time per project, founder-run)

1. `terraform -chdir=infra/envs/<env> init -backend=false`
2. `terraform -chdir=infra/envs/<env> apply -target=module.project_base -var project_id=<id>`
   — creates the state bucket (`<id>-tfstate`) among the base resources.
3. Re-init onto the backend:
   `terraform -chdir=infra/envs/<env> init -backend-config="bucket=<id>-tfstate" -backend-config="prefix=<env>" -migrate-state`
4. Full `apply`.

### One apply, no `-target` (T1.4a)

A fresh project is: bootstrap (steps 1-3 above, which exist only to create the
state bucket the backend lives in), then **one full `apply`**. The cluster, the
CNPG operator and its CRDs, the storage classes and the control-plane Cluster +
ScheduledBackup all land in that single pass.

**This used to require a four-step `-target` sequence, and the reason is worth
keeping.** `kubernetes_manifest` resolves its GroupVersionKind against the live
API at **plan** time, to build Terraform's type information from the OpenAPI
schema. On a from-zero apply there is no API yet, and for a custom resource
there is no CRD yet either, so the plan failed before anything was created:

```
Error: API did not recognize GroupVersionKind from manifest (CRD may not be installed)
```

`depends_on` cannot fix it — validation runs before dependency resolution — and
it is an acknowledged, still-open limitation
([#1367](https://github.com/hashicorp/terraform-provider-kubernetes/issues/1367),
[#2597](https://github.com/hashicorp/terraform-provider-kubernetes/issues/2597)),
not a transient error. Everything else about creating a cluster and deploying
into it in one apply is fine: HashiCorp's own GKE example does exactly that, and
the Kubernetes/Helm providers simply are not initialised until apply.

So `infra/` uses **`kubectl_manifest`** (`alekc/kubectl`) for every raw manifest.
It applies YAML the way `kubectl apply` does and resolves nothing at plan time.
`TestInfraNeverUsesKubernetesManifest` fails if `kubernetes_manifest` reappears
anywhere under `infra/` — one of them puts the whole cell back on `-target`, and
it would look reasonable in review because the failure only shows on a from-zero
apply nobody runs by hand.

`terraform validate` never contacts a cluster, so CI validation is unaffected.

**Destroy** still has an ordering constraint, and it is real: the Kubernetes,
Helm **and kubectl** providers are all configured from `module.gke_cell`
outputs — kubectl owns four of the five in-cluster resources — so destroy the
in-cluster resources first (`destroy -target=module.cnpg`) and the cluster after.
Terraform cannot express "tear this down before the thing that configures my
provider", so this one is documented rather than solved.

## CI

GitHub Actions federates via WIF (`project-base`: pool `github-ci`, provider
`github-oidc`, plan-only service account `ci-terraform-plan`). **CI plans;
the founder applies.** Zero static keys anywhere (D5) — the repo must stay
`google_service_account_key`-free (T1.1 AC, grep-enforced).

## Image trust chain (T1.3)

`image.yml` (path-gated on `services/**`) builds with BuildKit, pushes to the
`steloit` Artifact Registry repo, and **signs keylessly**: the GitHub OIDC
job identity federates through WIF (`ci-image-push` SA, writer on the one
repo), cosign records the signature against the Fulcio cert for that
identity, and the cluster's `ClusterImagePolicy`
(`infra/k8s/policy/cluster-image-policy.yaml` — authored; its apply + the
sigstore policy-controller install + namespace labeling are a tracked follow-up
that must land with the first real dev apply, see spec-change-proposals) admits only
images whose subject matches this repo's `image.yml`. No cosign keys exist
anywhere — keyless or nothing (D5). Unsigned-rejection evidence lands with
the T1.2 apply (cluster-gated AC).
