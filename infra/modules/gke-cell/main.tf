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
  # for an existing Standard cluster: "GKE Dataplane V2 can only be enabled when
  # creating a new cluster. Existing clusters cannot be upgraded to use GKE
  # Dataplane V2."
  #
  # What Terraform actually plans is WORSE than "rebuilds the cluster and its
  # node pools", which is what an earlier revision of this comment said. Measured
  # with hashicorp/google 6.50.0 against a fabricated state: the cluster shows
  # `must be replaced` / `~ datapath_provider ... # forces replacement`, but a
  # dependent node pool whose `cluster` is a literal name shows `will be updated
  # in-place` — `Plan: 1 to add, 1 to change, 1 to destroy`. The API destroys the
  # pools along with the cluster, so they are gone in fact and un-recreated in
  # state. Recovery is a manual state repair, not a re-apply.
  #
  # The premise is that no cell exists yet. VERIFIED 2026-08-23 against the live
  # project, not inferred: `gcloud container clusters list --project=steloit-dev`
  # returns `[]`, and `gs://steloit-dev-tfstate/dev/default.tfstate` (serial 47)
  # holds exactly ONE resource — module.project_base's state bucket. This module
  # has never been in state. No Cloud SQL, no compute instances either.
  #
  # (An earlier revision asserted this in the present tense while the author
  # could not run the command; it was then softened to dated corroboration. This
  # is the measured version.)
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

  # DNS IS PINNED TO kube-dns, and this is a D7 dependency, not a preference.
  #
  # US-3.3c's allow-dns-egress names the kube-dns and node-local-dns PODS. With
  # Cloud DNS for GKE there are no such pods at all: resolution goes to
  # 169.254.169.254:53 — the metadata server, which the same policy set
  # deliberately DENIES to customer pods (AC 9). Enabling Cloud DNS would
  # therefore leave customer workloads with no DNS, with every unit test green,
  # and the live evidence unable to see it (the verification cell ran kube-dns +
  # NodeLocal DNSCache).
  #
  # Stating it makes the coupling reviewable instead of accidental. Changing it
  # is a D7 change: the policy set has to gain an allowance first.
  dns_config {
    cluster_dns = "PROVIDER_UNSPECIFIED" # kube-dns; NOT CLOUD_DNS — see above
  }

  # VPC-NATIVE, STATED RATHER THAN INHERITED — and this is the same class of
  # decision as the line above: create-time only, silently defaulted, expensive
  # to discover later.
  #
  # Google's Dataplane V2 page does not list VPC-native as a prerequisite, so
  # this is NOT claimed as a requirement. What is documented is that "VPC-native
  # is the default network mode for all new clusters in any available version and
  # created through any surface", with routes-based still selectable. An empty
  # ip_allocation_policy block is the provider's way of asking for VPC-native
  # with GKE-managed secondary ranges — which is what we would get by default
  # today, written down so that a change in the default, or someone copying this
  # module, cannot silently produce a routes-based cell that then cannot be
  # converted.
  #
  # No secondary ranges are pinned here on purpose: infra/modules/network defines
  # none, and choosing pod/service CIDRs is a capacity decision for the first real
  # cell, not a default to bake in now. Raised by review as an adjacent
  # create-time-only setting; recorded rather than left implicit.
  ip_allocation_policy {}
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
