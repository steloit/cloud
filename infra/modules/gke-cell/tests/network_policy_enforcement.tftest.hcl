# D7's tenant boundary needs something that ENFORCES it.
#
# US-3.3a rendered default-deny NetworkPolicies, proved every manifest correct,
# and withdrew them on discovering the cluster had no enforcement at all: the API
# server accepts and stores a NetworkPolicy on a plain GKE Standard cluster and
# nothing drops a packet. Rendered, stored and enforced are three different
# things, and a green test suite covered the first two.
#
# This asserts the third against the PLANNED RESOURCE, not against the text of
# main.tf. `terraform test` with a mocked provider gives a credential-free plan,
# so this runs in CI on every PR alongside fmt and validate.
#
# What it CANNOT prove is that a pod in one environment actually fails to reach a
# pod in another. That needs a live cell and belongs in the e2e runbook; there
# are currently zero clusters in the project. This is the configuration half,
# asserted structurally — the honest boundary of what a plan can tell you.

# The mock returns zero-values for computed attributes, and this module's outputs
# index master_auth[0]. Supplying it keeps the plan whole so the assertions below
# are about the cluster's CONFIGURATION rather than about mock plumbing.
mock_provider "google-beta" {}

mock_provider "google" {
  mock_resource "google_container_cluster" {
    defaults = {
      endpoint = "10.0.0.1"
    }
  }
}

variables {
  project_id            = "steloit-test"
  zone                  = "asia-south1-a"
  cell_id               = "cell0"
  network_id            = "projects/steloit-test/global/networks/cell"
  subnet_id             = "projects/steloit-test/regions/asia-south1/subnetworks/cell"
  deletion_protection   = false
  core_machine_type     = "e2-standard-4"
  core_min_nodes        = 1
  core_max_nodes        = 3
  storage_machine_type  = "n2-standard-8"
  storage_node_count    = 1
  workload_machine_type = "e2-standard-4"
  workload_max_nodes    = 5
}

# `apply` against a MOCKED provider: nothing is created, no credentials are used,
# and computed attributes are populated so the module's outputs resolve. `plan`
# leaves them unknown, which this module's master_auth[0] output cannot survive.
run "the_cell_enforces_network_policy" {
  command = apply

  # master_auth is a computed block list; a mocked provider returns it empty and
  # this module's output indexes [0]. Overridden per-run so the plan resolves.
  override_resource {
    target = google_container_cluster.cell
    values = {
      master_auth = [{ cluster_ca_certificate = "bW9jaw==" }]
    }
  }

  assert {
    condition     = google_container_cluster.cell.datapath_provider == "ADVANCED_DATAPATH"
    error_message = "The cell does not enable GKE Dataplane V2, so NOTHING ENFORCES NetworkPolicy: the API server would accept and store D7's default-deny policies and never drop a packet. This is a create-time setting — Google documents no migration path for an existing Standard cluster, so changing it later means rebuilding the cluster and its node pools."
  }

  # Dataplane V2 has enforcement built in, and the API REJECTS the combination:
  # "Enabling NetworkPolicy for clusters with DatapathProvider=ADVANCED_DATAPATH
  # is not allowed." Setting both would fail at apply, on a cell, after the plan
  # looked fine — so it is refused here instead.
  assert {
    condition     = length(google_container_cluster.cell.network_policy) == 0
    error_message = "network_policy is set alongside ADVANCED_DATAPATH. GKE rejects that combination outright; Dataplane V2 already enforces NetworkPolicy, and the legacy Calico path must not be enabled with it."
  }

  # Workload Identity is what lets a customer's Postgres reach its WAL bucket
  # without a static key. The D7 egress allow-set has to permit the metadata
  # server precisely because this is on; if it were ever turned off, that
  # allowance would be dead weight and the WAL path would break differently.
  assert {
    condition     = length(google_container_cluster.cell.workload_identity_config) > 0
    error_message = "Workload Identity is off, so the D7 egress allowance for the metadata server protects nothing and WAL archiving has no credential path."
  }
}
