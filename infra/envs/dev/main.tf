provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

# k8s providers ride the gke-cell outputs (T1.2). IMPORTANT: kubernetes_manifest
# plans against the LIVE cluster API (schema fetch at plan time), so a fresh
# project needs the STAGED apply documented in infra/README.md — a bare full
# apply on empty state fails by design of the provider, not of this config.
# Auth via the caller's gcloud token (zero static keys, D5).
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
  source             = "../../modules/cost-guardrails"
  project_id         = var.project_id
  cell_id            = var.cell_id
  billing_account    = var.billing_account
  monthly_budget_usd = 300
  alert_emails       = var.alert_emails
}

module "cost_guardrails" {
  source             = "../../modules/cost-guardrails"
  project_id         = var.project_id
  cell_id            = var.cell_id
  billing_account    = var.billing_account
  monthly_budget_usd = 300
  alert_emails       = var.alert_emails
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
