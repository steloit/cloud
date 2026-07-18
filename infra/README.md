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

Apply order thereafter: project_base → network → gke_cell → cnpg/observability.

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
(`infra/k8s/policy/cluster-image-policy.yaml`, applied with T1.2) admits only
images whose subject matches this repo's `image.yml`. No cosign keys exist
anywhere — keyless or nothing (D5). Unsigned-rejection evidence lands with
the T1.2 apply (cluster-gated AC).
