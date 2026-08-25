provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

# ONE PASS, no -target (T1.4a, ADR-0017): raw manifests use kubectl_manifest,
# which resolves nothing at plan time.
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
  subnet_cidr    = "10.20.0.0/20"
  content_domain = var.content_domain

  # T1.8: WAIT FOR THE APIs. `google_project_service.enabled` had ZERO edges in
  # either direction in BOTH envs, so nothing waited for the APIs project_base
  # enables. From-zero applies worked only because the bootstrap procedure ran a
  # manual `gcloud services enable` first — which infra/README.md's contract does
  # not include, and which is what masked T1.7's missing API.
  #
  # `depends_on` on the whole module, not a threaded value: an `apis_ready` output
  # consumed as a module argument was tried first and MEASURED to create no edge
  # at all, because a variable no resource reads does not order that module's
  # resources. An argument that looks like enforcement and enforces nothing is
  # worse than none.
  #
  # Coarse on purpose — this also orders against buckets and WIF — because API
  # enablement genuinely is a project-wide precondition.
  #
  # NOT A TOTAL GUARANTEE: `google_project_service` returning success does not mean
  # the API is instantly usable; GCP enablement propagation lag is real. This
  # narrows the race, it does not close it.
  depends_on = [module.project_base]
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

  # T1.8: WAIT FOR THE APIs. `google_project_service.enabled` had ZERO edges in
  # either direction in BOTH envs, so nothing waited for the APIs project_base
  # enables. From-zero applies worked only because the bootstrap procedure ran a
  # manual `gcloud services enable` first — which infra/README.md's contract does
  # not include, and which is what masked T1.7's missing API.
  #
  # `depends_on` on the whole module, not a threaded value: an `apis_ready` output
  # consumed as a module argument was tried first and MEASURED to create no edge
  # at all, because a variable no resource reads does not order that module's
  # resources. An argument that looks like enforcement and enforces nothing is
  # worse than none.
  #
  # Coarse on purpose — this also orders against buckets and WIF — because API
  # enablement genuinely is a project-wide precondition.
  #
  # NOT A TOTAL GUARANTEE: `google_project_service` returning success does not mean
  # the API is instantly usable; GCP enablement propagation lag is real. This
  # narrows the race, it does not close it.
  depends_on = [module.project_base]
}

module "cnpg" {
  source     = "../../modules/cnpg"
  project_id = var.project_id
  cell_id    = var.cell_id

  # T1.8: WAIT FOR THE APIs. `google_project_service.enabled` had ZERO edges in
  # either direction in BOTH envs, so nothing waited for the APIs project_base
  # enables. From-zero applies worked only because the bootstrap procedure ran a
  # manual `gcloud services enable` first — which infra/README.md's contract does
  # not include, and which is what masked T1.7's missing API.
  #
  # `depends_on` on the whole module, not a threaded value: an `apis_ready` output
  # consumed as a module argument was tried first and MEASURED to create no edge
  # at all, because a variable no resource reads does not order that module's
  # resources. An argument that looks like enforcement and enforces nothing is
  # worse than none.
  #
  # Coarse on purpose — this also orders against buckets and WIF — because API
  # enablement genuinely is a project-wide precondition.
  #
  # NOT A TOTAL GUARANTEE: `google_project_service` returning success does not mean
  # the API is instantly usable; GCP enablement propagation lag is real. This
  # narrows the race, it does not close it.
  depends_on = [module.project_base]
}

module "cost_guardrails" {
  source               = "../../modules/cost-guardrails"
  project_id           = var.project_id
  cell_id              = var.cell_id
  billing_account      = var.billing_account
  monthly_budget_units = var.monthly_budget_units
  alert_emails         = var.alert_emails

  # T1.8: WAIT FOR THE APIs. `google_project_service.enabled` had ZERO edges in
  # either direction in BOTH envs, so nothing waited for the APIs project_base
  # enables. From-zero applies worked only because the bootstrap procedure ran a
  # manual `gcloud services enable` first — which infra/README.md's contract does
  # not include, and which is what masked T1.7's missing API.
  #
  # `depends_on` on the whole module, not a threaded value: an `apis_ready` output
  # consumed as a module argument was tried first and MEASURED to create no edge
  # at all, because a variable no resource reads does not order that module's
  # resources. An argument that looks like enforcement and enforces nothing is
  # worse than none.
  #
  # Coarse on purpose — this also orders against buckets and WIF — because API
  # enablement genuinely is a project-wide precondition.
  #
  # NOT A TOTAL GUARANTEE: `google_project_service` returning success does not mean
  # the API is instantly usable; GCP enablement propagation lag is real. This
  # narrows the race, it does not close it.
  depends_on = [module.project_base]
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

  # T1.8: WAIT FOR THE APIs. `google_project_service.enabled` had ZERO edges in
  # either direction in BOTH envs, so nothing waited for the APIs project_base
  # enables. From-zero applies worked only because the bootstrap procedure ran a
  # manual `gcloud services enable` first — which infra/README.md's contract does
  # not include, and which is what masked T1.7's missing API.
  #
  # `depends_on` on the whole module, not a threaded value: an `apis_ready` output
  # consumed as a module argument was tried first and MEASURED to create no edge
  # at all, because a variable no resource reads does not order that module's
  # resources. An argument that looks like enforcement and enforces nothing is
  # worse than none.
  #
  # Coarse on purpose — this also orders against buckets and WIF — because API
  # enablement genuinely is a project-wide precondition.
  #
  # NOT A TOTAL GUARANTEE: `google_project_service` returning success does not mean
  # the API is instantly usable; GCP enablement propagation lag is real. This
  # narrows the race, it does not close it.
  depends_on = [module.project_base]
}
