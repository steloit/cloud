output "state_bucket" {
  value = google_storage_bucket.state.name
}

output "artifacts_bucket" {
  value = google_storage_bucket.artifacts.name
}

output "wal_customer_bucket" {
  value = google_storage_bucket.wal_customer.name
}

output "wal_control_bucket" {
  value = google_storage_bucket.wal_control.name
}

output "kms_secrets_key" {
  value = google_kms_crypto_key.secrets.id
}

output "ci_plan_service_account" {
  value = google_service_account.ci_plan.email
}

output "wif_provider" {
  value = google_iam_workload_identity_pool_provider.github.name
}
