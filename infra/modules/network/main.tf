locals {
  labels = {
    cell_id    = var.cell_id
    managed_by = "steloit-terraform"
  }
}

resource "google_compute_network" "vpc" {
  name                    = "${var.cell_id}-vpc"
  project                 = var.project_id
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "cell" {
  name                     = "${var.cell_id}-subnet"
  project                  = var.project_id
  region                   = var.region
  network                  = google_compute_network.vpc.id
  ip_cidr_range            = var.subnet_cidr
  private_ip_google_access = true
}

resource "google_compute_router" "egress" {
  name    = "${var.cell_id}-router"
  project = var.project_id
  region  = var.region
  network = google_compute_network.vpc.id
}

resource "google_compute_router_nat" "egress" {
  name                               = "${var.cell_id}-nat"
  project                            = var.project_id
  region                             = var.region
  router                             = google_compute_router.egress.name
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

# One external IP for the cell LB (console static bundle + api ingress ride it).
resource "google_compute_global_address" "lb" {
  name    = "${var.cell_id}-lb"
  project = var.project_id
  labels  = local.labels
}

# Customer-content zone (A2.4: separate registrable domain, githubusercontent
# pattern). Gated on P2 — created only once the domain exists.
resource "google_dns_managed_zone" "content" {
  count    = var.content_domain != "" ? 1 : 0
  name     = "${var.cell_id}-content"
  project  = var.project_id
  dns_name = "${var.content_domain}."
  labels   = local.labels
}
