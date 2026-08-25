provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

# k8s providers ride the gke-cell outputs (T1.2). ONE PASS, no -target (T1.4a,
# ADR-0017): every raw manifest uses kubectl_manifest, which resolves nothing at
# plan time, so the cluster, the CRDs and the objects that need them all land in
# a single apply. Auth via the caller's gcloud token (zero static keys, D5).
data "google_client_config" "current" {}

provider "kubernetes" {
  host                   = "https://${module.gke_cell.cluster_endpoint}"
  token                  = data.google_client_config.current.access_token
  cluster_ca_certificate = base64decode(module.gke_cell.cluster_ca_certificate)
}

provider "helm" {
  kubernetes {
    host                   = "https://${module.gke_cell.cluster_endpoint}"
    token                  = data.google_client_config.current.access_token
    cluster_ca_certificate = base64decode(module.gke_cell.cluster_ca_certificate)
  }
}

# The kubectl provider carries the SAME credentials as the other two — one
# cluster, one identity. `load_config_file = false` is load-bearing: without it
# the provider falls back to whatever ~/.kube/config happens to point at, which
# on a developer machine is a different cluster and on a runner is nothing.
provider "kubectl" {
  host                   = "https://${module.gke_cell.cluster_endpoint}"
  token                  = data.google_client_config.current.access_token
  cluster_ca_certificate = base64decode(module.gke_cell.cluster_ca_certificate)
  load_config_file       = false

  # lazy_load IS THE ONE-PASS APPLY. Without it this provider CONFIGURES at plan
  # time and fails when host/CA are still unknown — which is the from-zero case,
  # the whole point of T1.4a. Measured: `Plan: 1 to add` then
  #   Error: invalid provider configuration: default cluster has no server defined
  # and with it, a clean plan. hashicorp/kubernetes and helm tolerate an
  # unconfigured client at plan; this one does not, so removing this line
  # reintroduces the -target bootstrap in a different disguise.
  lazy_load = true

  # A single attempt is the 2.4.1 default. The CNPG Cluster is applied moments
  # after the operator's Helm release returns, so a transient CRD-registration
  # or admission-webhook window would fail the whole apply with no retry — the
  # difference between a one-pass apply that works and one that is lucky.
  apply_retry_count = 3
}


module "project_base" {
  source     = "../../modules/project-base"
  project_id = var.project_id
  region     = var.region
  cell_id    = var.cell_id
}

module "network" {
  source         = "../../modules/network"
  project_id     = var.project_id
  region         = var.region
  cell_id        = var.cell_id
  subnet_cidr    = "10.10.0.0/20"
  content_domain = var.content_domain
}

module "gke_cell" {
  source              = "../../modules/gke-cell"
  project_id          = var.project_id
  zone                = var.zone
  cell_id             = var.cell_id
  network_id          = module.network.network_id
  subnet_id           = module.network.subnet_id
  deletion_protection = false

  # capacity (this file is the ONLY place numbers live)
  core_machine_type     = "e2-small"
  core_min_nodes        = 1 # A1.6 floor
  core_max_nodes        = 2
  storage_machine_type  = "n2-standard-4"
  storage_node_count    = 1
  storage_driver        = "pd" # canonical (ADR-0007/A6); "zfs" is the Cell-1 knob
  workload_machine_type = "e2-standard-4"
  workload_max_nodes    = 3
}

module "cnpg" {
  source                     = "../../modules/cnpg"
  project_id                 = var.project_id
  cell_id                    = var.cell_id
  control_plane              = true # the control-plane DB lives in dev (invariant 10 bucket below)
  wal_control_bucket         = module.project_base.wal_control_bucket
  control_plane_storage_size = "10Gi" # capacity lives HERE, not in the module
}

module "cost_guardrails" {
  source               = "../../modules/cost-guardrails"
  project_id           = var.project_id
  cell_id              = var.cell_id
  billing_account      = var.billing_account
  monthly_budget_units = var.monthly_budget_units
  alert_emails         = var.alert_emails
}

# ADR-0015's first reason for Dataplane V2 is denied-connection logging; this is
# what installs it. Without it that rationale rests on a file nothing applies.
module "datapath_policy" {
  source  = "../../modules/datapath-policy"
  cell_id = var.cell_id
}

module "observability" {
  source     = "../../modules/observability"
  project_id = var.project_id
  cell_id    = var.cell_id
}

# Dev only: duty-cycling (A1.6 — the destroyable founder env sleeps).
module "duty_cycle" {
  source     = "../../modules/duty-cycle"
  project_id = var.project_id
  cell_id    = var.cell_id
}
