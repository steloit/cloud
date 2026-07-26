output "cluster_name" {
  value = google_container_cluster.cell.name
}

output "cluster_endpoint" {
  value     = google_container_cluster.cell.endpoint
  sensitive = true
}

output "cluster_ca_certificate" {
  value     = google_container_cluster.cell.master_auth[0].cluster_ca_certificate
  sensitive = true
}
