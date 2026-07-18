output "network_id" {
  value = google_compute_network.vpc.id
}

output "subnet_id" {
  value = google_compute_subnetwork.cell.id
}

output "lb_address" {
  value = google_compute_global_address.lb.address
}
