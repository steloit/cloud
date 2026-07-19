locals {
  labels = {
    cell_id    = var.cell_id
    managed_by = "steloit-terraform"
  }
}

resource "google_container_cluster" "cell" {
  name     = var.cell_id
  project  = var.project_id
  location = var.zone

  network    = var.network_id
  subnetwork = var.subnet_id

  remove_default_node_pool = true
  initial_node_count       = 1
  deletion_protection      = var.deletion_protection
  resource_labels          = local.labels

  release_channel {
    channel = "REGULAR"
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }
}

# Core pool: api + operators + observability. Floor 1 (A1.6) — never zero.
resource "google_container_node_pool" "core" {
  name     = "core"
  project  = var.project_id
  location = var.zone
  cluster  = google_container_cluster.cell.name

  autoscaling {
    min_node_count = var.core_min_nodes
    max_node_count = var.core_max_nodes
  }

  node_config {
    machine_type = var.core_machine_type
    labels       = merge(local.labels, { pool = "core" })

    workload_metadata_config {
      mode = "GKE_METADATA"
    }

    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }
}

# DB storage pool (ADR-0007 / INF-001 A6, ratified 2026-07-19): the canonical
# dev/alpha driver is "pd" — plain PD-CSI-backed nodes, no local SSD, no node
# bootstrap. "zfs" (OpenEBS ZFS-LocalPV on local SSD) is RETAINED as the Cell-1
# branch-density option behind an explicit measured trigger — same pool shape,
# one variable. Fixed count (stateful — never autoscaled); tainted so only
# CNPG/storage pods land here.
resource "google_container_node_pool" "db_storage" {
  name       = "db-storage"
  project    = var.project_id
  location   = var.zone
  cluster    = google_container_cluster.cell.name
  node_count = var.storage_node_count

  node_config {
    machine_type    = var.storage_machine_type
    local_ssd_count = var.storage_driver == "zfs" ? var.storage_local_ssd_count : 0
    labels          = merge(local.labels, { pool = "db-storage", storage_driver = var.storage_driver })

    taint {
      key    = "storage"
      value  = var.storage_driver
      effect = "NO_SCHEDULE"
    }

    workload_metadata_config {
      mode = "GKE_METADATA"
    }

    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }
}

# Workload pool: customer code + builds, gVisor-sandboxed (D7), scale-to-zero.
resource "google_container_node_pool" "workload" {
  # sandbox_config (gVisor) is google-beta-only in provider 6.x.
  provider = google-beta
  name     = "workload"
  project  = var.project_id
  location = var.zone
  cluster  = google_container_cluster.cell.name

  autoscaling {
    min_node_count = 0
    max_node_count = var.workload_max_nodes
  }

  node_config {
    machine_type = var.workload_machine_type
    image_type   = "COS_CONTAINERD" # required by gVisor sandbox
    labels       = merge(local.labels, { pool = "workload" })

    sandbox_config {
      sandbox_type = "gvisor"
    }

    taint {
      key    = "sandbox"
      value  = "gvisor"
      effect = "NO_SCHEDULE"
    }

    workload_metadata_config {
      mode = "GKE_METADATA"
    }

    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }
}
