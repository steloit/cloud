provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
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
  core_machine_type       = "e2-medium"
  core_min_nodes          = 1 # A1.6 floor
  core_max_nodes          = 3
  storage_machine_type    = "n2-standard-8"
  storage_node_count      = 1
  storage_local_ssd_count = 2
  workload_machine_type   = "e2-standard-4"
  workload_max_nodes      = 5
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
  alert_emails       = var.alert_emails
}

module "observability" {
  source     = "../../modules/observability"
  project_id = var.project_id
  cell_id    = var.cell_id
}
