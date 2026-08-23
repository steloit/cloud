output "cluster_name" {
  value = google_container_cluster.cell.name
}

output "cluster_endpoint" {
  value     = google_container_cluster.cell.endpoint
  sensitive = true
}

# `one()` rather than `[0]`. A real cluster always returns exactly one
# master_auth block, so the index was correct in production — but it makes the
# module impossible to evaluate under ANY harness that does not perform a real
# apply, because a computed block list is empty until the API answers. That is a
# testability defect, and it is what stopped `terraform test` from asserting the
# cell enforces NetworkPolicy at all.
#
# one() returns the single element, or null for an empty list, instead of
# erroring. A null here is also a better failure than a plan-time crash: it
# surfaces as an empty kubeconfig field rather than an unrelated index error.
output "cluster_ca_certificate" {
  value     = try(one(google_container_cluster.cell.master_auth).cluster_ca_certificate, null)
  sensitive = true
}
