locals {
  labels = {
    cell_id    = var.cell_id
    managed_by = "steloit-terraform"
  }
}

# Dedicated node service account (review finding: default compute SA is
# project-Editor-adjacent — a node compromise on the pool running customer code
# must not yield broad project credentials). Least-privilege: logs, metrics,
# and image pulls only (D5 posture at the node layer).
resource "google_service_account" "node" {
  project      = var.project_id
  account_id   = "gke-node-${var.cell_id}"
  display_name = "GKE node SA (${var.cell_id}) — least-privilege: logging/monitoring/AR-read"
}

resource "google_project_iam_member" "node_roles" {
  for_each = toset([
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/monitoring.viewer",
    "roles/stackdriver.resourceMetadata.writer",
    "roles/artifactregistry.reader",
  ])
  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.node.email}"
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

  # NETWORKPOLICY ENFORCEMENT. Without this the API server ACCEPTS AND STORES
  # every NetworkPolicy and nothing drops a packet: D7's tenant boundary would be
  # a set of objects that look like isolation and are not. US-3.3a rendered that
  # boundary, measured every manifest correct, and withdrew it on discovering
  # there was nothing to enforce it.
  #
  # ADVANCED_DATAPATH is GKE Dataplane V2 (eBPF/Cilium). It "comes with
  # Kubernetes network policy enforcement built-in", so `network_policy` must NOT
  # also be set — the API rejects that combination outright:
  #   "Enabling NetworkPolicy for clusters with DatapathProvider=ADVANCED_DATAPATH
  #    is not allowed."
  # (cloud.google.com/kubernetes-engine/docs/how-to/dataplane-v2). The legacy
  # alternative is Calico via `network_policy`, which we do not use.
  #
  # THIS IS A CREATE-TIME DECISION (ADR-0015). Google documents no migration path
  # for an existing Standard cluster; changing it later rebuilds the cluster and
  # its node pools.
  #
  # The premise is that no cell exists yet. Corroborated but DATED: this module
  # has never been applied (no env has completed the README bootstrap, so no
  # state exists), and infra/spike/results/teardown.log's orphan sweep shows an
  # empty cluster listing as of 2026-07-19. It is not a present-tense verified
  # fact — an earlier revision of this comment asserted one.
  #
  # If a cell DOES exist, the two envs differ sharply: cell0 sets
  # deletion_protection = true, so a forced replacement aborts at apply, loudly.
  # dev sets it to FALSE, so a routine apply would destroy and recreate the
  # cluster and every node pool with no barrier but someone reading the plan.
  # Read the first plan for "must be replaced" before applying to dev.
  #
  # Known Dataplane V2 caveats we are accepting, from Google's own list: NetworkPolicy
  # `endPort` (port RANGES) is silently not enforced on affected versions — read a
  # policy back and check the field survived before relying on a range; hairpin
  # connections can drop; hostPort conflicts with the NodePort range. None of these
  # affect the D7 policy set, which uses single ports and no hostPort.
  datapath_provider = "ADVANCED_DATAPATH"
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

    service_account = google_service_account.node.email
    oauth_scopes    = ["https://www.googleapis.com/auth/cloud-platform"]
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

    service_account = google_service_account.node.email
    oauth_scopes    = ["https://www.googleapis.com/auth/cloud-platform"]
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

    service_account = google_service_account.node.email
    oauth_scopes    = ["https://www.googleapis.com/auth/cloud-platform"]
  }
}
