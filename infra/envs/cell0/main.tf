provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

# See infra/README.md staged-apply note: kubernetes_manifest plans against the
# live cluster API — fresh projects apply in stages.
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
  subnet_cidr    = "10.20.0.0/20"
  content_domain = var.content_domain
}

module "gke_cell" {
  source              = "../../modules/gke-cell"
  project_id          = var.project_id
  zone                = var.zone
  cell_id             = var.cell_id
  network_id          = module.network.network_id
  subnet_id           = module.network.subnet_id
  deletion_protection = true

  # capacity (this file is the ONLY place numbers live)
  core_machine_type    = "e2-medium"
  core_min_nodes       = 1 # A1.6 floor
  core_max_nodes       = 3
  storage_machine_type = "n2-standard-8"
  storage_node_count   = 1
  # ADR-0007/A6: pd is canonical; flipping to "zfs" (+ storage_local_ssd_count)
  # is the Cell-1 density decision — requires the measured trigger, never default.
  storage_driver        = "pd"
  workload_machine_type = "e2-standard-4"
  workload_max_nodes    = 5
}

module "cnpg" {
  source     = "../../modules/cnpg"
  project_id = var.project_id
  cell_id    = var.cell_id
}

module "cost_guardrails" {
  source             = "../../modules/cost-guardrails"
  project_id         = var.project_id
  cell_id            = var.cell_id
  billing_account    = var.billing_account
  monthly_budget_usd = 700
  budget_currency    = var.budget_currency
  alert_emails       = var.alert_emails
}

module "observability" {
  source     = "../../modules/observability"
  project_id = var.project_id
  cell_id    = var.cell_id
}
