locals {
  labels = {
    cell_id    = var.cell_id
    managed_by = "steloit-terraform"
  }
  services = [
    "compute.googleapis.com",
    "container.googleapis.com",
    "storage.googleapis.com",
    "secretmanager.googleapis.com",
    "cloudkms.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "sts.googleapis.com",
    "dns.googleapis.com",
    "artifactregistry.googleapis.com",
    "cloudscheduler.googleapis.com",
    "monitoring.googleapis.com",
    "logging.googleapis.com",
  ]
}

resource "google_project_service" "enabled" {
  for_each           = toset(local.services)
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# State bucket: bootstrap chicken-and-egg — created by the documented one-time
# `-backend=false` apply of this module (infra/README.md), then the env's
# backend points at it. Never auto-created by the env that uses it.
resource "google_storage_bucket" "state" {
  name                        = "${var.project_id}-tfstate"
  project                     = var.project_id
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  labels                      = local.labels

  versioning {
    enabled = true
  }
}

resource "google_storage_bucket" "artifacts" {
  name                        = "${var.project_id}-artifacts"
  project                     = var.project_id
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  labels                      = local.labels
}

# WAL archives + backups (architecture §3: PITR to GCS; control-plane DB uses
# a SEPARATE bucket per INF-001 invariant 10 — hence two).
resource "google_storage_bucket" "wal_customer" {
  name                        = "${var.project_id}-wal-customer"
  project                     = var.project_id
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  labels                      = local.labels
}

resource "google_storage_bucket" "wal_control" {
  name                        = "${var.project_id}-wal-control"
  project                     = var.project_id
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  labels                      = local.labels
}

resource "google_kms_key_ring" "core" {
  name     = "${var.cell_id}-core"
  project  = var.project_id
  location = var.region
}

# Secret Manager envelope key (architecture §14: KMS envelope; D5).
resource "google_kms_crypto_key" "secrets" {
  name            = "secrets-envelope"
  key_ring        = google_kms_key_ring.core.id
  rotation_period = "7776000s" # 90 days
  labels          = local.labels
}

# CI federation: GitHub Actions -> WIF -> plan-only service account.
# Zero static keys (D5); apply stays founder-run (infra/README.md).
resource "google_iam_workload_identity_pool" "ci" {
  project                   = var.project_id
  workload_identity_pool_id = "github-ci"
  display_name              = "GitHub Actions CI"
}

resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.ci.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-oidc"
  display_name                       = "GitHub OIDC"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
  }
  attribute_condition = "assertion.repository == \"${var.github_repo}\""

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account" "ci_plan" {
  project      = var.project_id
  account_id   = "ci-terraform-plan"
  display_name = "CI terraform plan (read-only; apply is founder-run)"
}

resource "google_project_iam_member" "ci_plan_viewer" {
  project = var.project_id
  role    = "roles/viewer"
  member  = "serviceAccount:${google_service_account.ci_plan.email}"
}

resource "google_storage_bucket_iam_member" "ci_plan_state" {
  bucket = google_storage_bucket.state.name
  role   = "roles/storage.objectAdmin" # state lock/read requires object write
  member = "serviceAccount:${google_service_account.ci_plan.email}"
}

resource "google_service_account_iam_member" "ci_plan_wif" {
  service_account_id = google_service_account.ci_plan.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.ci.name}/attribute.repository/${var.github_repo}"
}
