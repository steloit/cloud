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

# No overrides are needed here: the module's cluster_ca_certificate output is a
# splat + one(), which yields null over the mock's empty master_auth rather than
# erroring. (An earlier revision carried override_resource scaffolding and a
# comment about "master_auth[0]"; both the indexing and the scaffolding are gone,
# and the comment describing them outlived them by a round.)
mock_provider "google-beta" {}
mock_provider "google" {}

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

# and computed attributes are populated so the module's outputs resolve. `plan`
# leaves them unknown, which this module's master_auth[0] output cannot survive.
# `apply` against MOCKED providers — nothing is created and no credentials are
# used. It is needed because several assertions below reference COMPUTED
# attributes (the node service account's email, and the node pools' nested
# config blocks), which are unknown under `plan` and make the condition
# unevaluatable.
#
# An earlier revision also used `apply`, but justified it with a claim about
# `master_auth[0]` that is no longer true and was never the reason. It also
# carried two blocks of mock scaffolding for master_auth, both verified INERT by
# deletion: `override_resource.values` cannot populate a computed nested block
# list. They are gone rather than left to teach the next reader a false model of
# how this harness works.
run "the_cell_enforces_network_policy" {
  command = apply

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

  # WORKLOAD IDENTITY HAS TWO REPRESENTATIONS. The cluster-level pool is useless
  # if a node pool exposes the raw GCE metadata server — pods then get the NODE
  # service account instead of a workload identity. Deleting GKE_METADATA from
  # all three pools was a green mutation while this assertion passed.
  assert {
    condition = alltrue([for p in [
      google_container_node_pool.core,
      google_container_node_pool.db_storage,
      google_container_node_pool.workload,
      ] : one(one(p.node_config).workload_metadata_config).mode == "GKE_METADATA"
    ])
    error_message = "a node pool exposes the raw GCE metadata server — cluster-level Workload Identity does nothing for pods on it, so WAL archiving has no credential path and the D7 metadata-server allowance protects nothing."
  }

  # The dedicated least-privilege node SA. Without it the pools fall back to the
  # DEFAULT COMPUTE SA, which this module's own comment calls
  # "project-Editor-adjacent" — and a node compromise on the pool running
  # customer code then walks around both gVisor and NetworkPolicy. Deleting it
  # from all three pools was green.
  assert {
    condition = alltrue([for p in [
      google_container_node_pool.core,
      google_container_node_pool.db_storage,
      google_container_node_pool.workload,
      ] : one(p.node_config).service_account == google_service_account.node.email
    ])
    error_message = "a node pool runs as the default compute service account, which is project-Editor-adjacent; a node compromise on the customer-code pool would yield broad project credentials."
  }

  # gVisor on the pool that runs customer code. INF-001 D7: customer code ALWAYS
  # runs under GKE Sandbox.
  #
  # ALL THREE PARTS ARE ASSERTED, because sandbox_config alone is a third of the
  # mechanism and the other two were measured green: removing the `sandbox` taint
  # (nothing then keeps ordinary pods off the sandboxed pool, and nothing keeps
  # customer pods ON it), and removing image_type = "COS_CONTAINERD", which the
  # module's own comment calls "required by gVisor sandbox". An earlier revision
  # of this comment NAMED the taint as a green mutation and then did not close
  # it, which is worse than not having measured it.
  assert {
    condition     = one(one(google_container_node_pool.workload.node_config).sandbox_config).sandbox_type == "gvisor"
    error_message = "the workload pool does not run gVisor — INF-001 D7 requires customer code to run under GKE Sandbox."
  }

  assert {
    condition = contains([
      for t in one(google_container_node_pool.workload.node_config).taint :
      "${t.key}=${t.value}:${t.effect}"
    ], "sandbox=gvisor:NO_SCHEDULE")
    error_message = "the workload pool has no sandbox=gvisor:NO_SCHEDULE taint — gVisor is configured on a pool that anything can land on, so the isolation is advisory."
  }

  assert {
    condition = contains([
      for t in one(google_container_node_pool.db_storage.node_config).taint :
      "${t.key}=${t.effect}"
    ], "storage=NO_SCHEDULE")
    error_message = "the db-storage pool has no storage NO_SCHEDULE taint — general workloads can land on the stateful pool."
  }

  assert {
    condition     = one(google_container_node_pool.workload.node_config).image_type == "COS_CONTAINERD"
    error_message = "the workload pool is not COS_CONTAINERD, which gVisor sandboxing requires."
  }

  # THE OTHER WAY TO ENABLE LEGACY NETWORK POLICY. `network_policy {}` is asserted
  # elsewhere in this file, but the Calico ADDON is a different attribute path
  # entirely and was measured green: addons_config.network_policy_config with
  # disabled = false. GKE does not support it alongside Dataplane V2, so the
  # guard's own premise was covered for one of the two ways to trip it.
  assert {
    condition = alltrue([
      for a in google_container_cluster.cell.addons_config :
      length(a.network_policy_config) == 0 || one(a.network_policy_config).disabled
    ])
    error_message = "the Calico NetworkPolicy addon is enabled alongside ADVANCED_DATAPATH — a second, separate way to trip the combination GKE rejects."
  }

  # A FOURTH NODE POOL THE SECURITY ASSERTIONS CANNOT SEE. Flipping
  # remove_default_node_pool to false was green everywhere: the cluster keeps a
  # default pool running the DEFAULT COMPUTE SERVICE ACCOUNT, with no taint, no
  # image_type, no sandbox_config and no GKE_METADATA. Every assertion in this
  # file iterates the three pools by name, so it is structurally blind to a
  # fourth — and the default-compute-SA outcome is exactly what the node identity
  # assertion above exists to prevent.
  assert {
    condition     = google_container_cluster.cell.remove_default_node_pool
    error_message = "the default node pool is kept; it runs the default compute service account (project-Editor-adjacent) and none of this module's pool assertions can see it."
  }

  # VPC-native is asserted at BOTH layers, like the datapath. It was env-only, and
  # the env layer is the one a rename can drop out of CI.
  assert {
    condition     = length(google_container_cluster.cell.ip_allocation_policy) == 1
    error_message = "the cell does not request VPC-native networking; network mode is create-time only and cannot be converted afterwards."
  }

  # Least privilege is asserted through the SCOPES as well as the identity: the
  # node service account assertion above is green with oauth_scopes = [] on the
  # core and db-storage pools, so it pins who the node is and not what it may do.
  assert {
    condition = alltrue([
      for p in [
        google_container_node_pool.core,
        google_container_node_pool.workload,
        google_container_node_pool.db_storage,
      ] : contains(one(p.node_config).oauth_scopes, "https://www.googleapis.com/auth/cloud-platform")
    ])
    error_message = "a node pool has lost the cloud-platform oauth scope; Workload Identity resolves permissions per-pod, but the node itself still needs this scope to function."
  }

  # A1.6's floor-1 invariant, stated in main.tf and in the variable description
  # and pinned by nothing: min_node_count = 0 was green.
  assert {
    condition     = one(google_container_node_pool.core.autoscaling).min_node_count >= 1
    error_message = "the core pool can scale to zero; A1.6 requires a floor of 1 (api + operators + observability never go away)."
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
