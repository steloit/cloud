output "cluster_name" {
  value = google_container_cluster.cell.name
}

output "cluster_endpoint" {
  value     = google_container_cluster.cell.endpoint
  sensitive = true
}

# SPLAT + one(), and deliberately NO try().
#
# A real cluster always returns exactly one master_auth block, so `[0]` was
# correct in production — but a computed block list is EMPTY until the API
# answers, which made the module impossible to evaluate under any harness short
# of a real apply. Terraform mocks cannot populate a computed nested block list
# (override_resource and mock_resource defaults were both measured inert), so
# this line is genuinely what made the enforcement assertion possible.
#
# `master_auth[*]` over an empty list yields an empty list, so one() returns null
# with no try() at all. An earlier revision used try(one(...), null), which
# swallowed EVERY error in the expression — including one()'s own "must be a list
# with zero or one elements" — and handed a silent null to two provider blocks.
#
# A null does NOT "surface as an empty kubeconfig field", as that revision
# claimed: both envs feed this straight into base64decode(), which errors with
# `argument must not be null`. It fails loudly, which is right — the claim was
# simply wrong, and measured so.
output "cluster_ca_certificate" {
  value     = one(google_container_cluster.cell.master_auth[*].cluster_ca_certificate)
  sensitive = true
}

# Surfaced so an ENV-level test can assert what that env actually plans. The
# module's own test can only prove the module's literal; the deployed artifact is
# the env, and the two are different representations of the same property.
output "datapath_provider" {
  value = google_container_cluster.cell.datapath_provider
}

output "deletion_protection" {
  value = google_container_cluster.cell.deletion_protection
}

# The OTHER half of the enforcement invariant, surfaced for the same reason
# datapath_provider is: ADVANCED_DATAPATH and network_policy are mutually
# exclusive, and the module test can only prove the module's own literal. Review
# measured the gap — adding a `legacy_network_policy` variable and setting it in
# dev left the module test, BOTH env tests, fmt and both validates green, while
# the real apply is rejected with "Enabling NetworkPolicy for clusters with
# DatapathProvider=ADVANCED_DATAPATH is not allowed". A plan-level test has no
# backstop after the plan for that combination, so both halves must be asserted
# where the deployed artifact is composed.
output "network_policy" {
  value = google_container_cluster.cell.network_policy
}

# VPC-native is create-time only; surfaced so each env asserts what IT plans.
output "ip_allocation_policy" {
  value = google_container_cluster.cell.ip_allocation_policy
}
