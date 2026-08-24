# THE ENV IS WHAT DEPLOYS. The module test asserts the module's literal; this
# asserts the value THIS ENVIRONMENT actually plans.
#
# Those are two representations of one property, and only the first was covered.
# Demonstrated: make the module take `datapath_provider = var.datapath_provider`
# with the right default and have this env pass "LEGACY_DATAPATH" — the module
# test passes, `terraform fmt -check` is clean, `terraform validate` succeeds,
# and the whole `infra` job is green while this cell deploys with NO
# NetworkPolicy enforcement. That is the US-3.3a defect restored, past the test
# written to prevent it.
#
# `terraform validate` cannot check a value. This can.

mock_provider "google" {
  # The mock invents random ids for computed attributes, and something then
  # rejects one. The cause is NOT what an earlier revision of this comment said
  # ("google_service_account.name against a regexp whose local part must be 6-30
  # chars"): measured, the error is raised on `service_account_id` of
  # google_service_account_iam_member — in module.cnpg (cnpg_control_wif) and
  # module.project_base (ci_image_wif, ci_plan_wif) — because an 8-char mock id
  # like "59csy0yn" is not a `projects/.../serviceAccounts/...` path. Length is
  # not the rule. gke-cell's own SA is not implicated at all, which is exactly
  # why the MODULE test needs no defaults and these env tests do.
  #
  # Note the block is TYPE-WIDE: it supplies one fake identity to every
  # google_service_account in the env. A PRODUCIBLE default keeps the plan whole;
  # nothing here is asserted.
  mock_resource "google_service_account" {
    defaults = {
      name  = "projects/steloit-test/serviceAccounts/cell-mock-sa@steloit-test.iam.gserviceaccount.com"
      email = "cell-mock-sa@steloit-test.iam.gserviceaccount.com"
    }
  }
}
mock_provider "google-beta" {}
mock_provider "kubernetes" {}
mock_provider "helm" {}

# kubectl is mocked for the same reason kubernetes and helm are: this suite
# asserts the PLANNED cell, and an unmocked provider tries to reach a real API
# (measured: it dialled a cluster IP and timed out). ADR-0017 moved every raw
# manifest onto it, so it is now part of the plan the same way helm is.
mock_provider "kubectl" {}

variables {
  project_id = "steloit-test"
  # Required with no default so a real plan refuses until the amount is chosen
  # (O2's fail-closed guard — which this test just re-confirmed by failing on it).
  # A test value, emphatically NOT a budget decision: billing_account defaults to
  # "" here, so the guardrail module renders nothing either way.
  monthly_budget_units = 1
}

run "this_cell_enforces_network_policy" {
  command = apply

  assert {
    condition     = module.gke_cell.datapath_provider == "ADVANCED_DATAPATH"
    error_message = "the dev cell does not enable GKE Dataplane V2, so NOTHING ENFORCES NetworkPolicy: the API server would accept and store D7's default-deny policies and never drop a packet. Create-time only — changing it later rebuilds the cluster and its node pools."
  }

  # An env-supplied safety value with no env-level test was the same class of
  # hole: the module can hardcode it away and no gate notices.
  assert {
    condition     = module.gke_cell.deletion_protection == false
    error_message = "the dev cell's deletion_protection is not false — this env's chosen setting is not reaching the cluster."
  }

  # THE OTHER HALF OF THE SAME INVARIANT. ADVANCED_DATAPATH and network_policy are
  # mutually exclusive, and only the first half was ported to this layer. Review
  # measured it: giving the module a `legacy_network_policy` variable and setting
  # it in this env left the module test, BOTH env tests, fmt and both validates
  # green, while the real apply is rejected with "Enabling NetworkPolicy for
  # clusters with DatapathProvider=ADVANCED_DATAPATH is not allowed". A mocked
  # plan has no backstop for that combination, so it has to be asserted here.
  assert {
    condition     = length(module.gke_cell.network_policy) == 0
    error_message = "this cell sets network_policy alongside ADVANCED_DATAPATH — the two are mutually exclusive and GKE rejects the combination at apply, so the cell cannot be created at all."
  }

  # VPC-native, which is create-time only in the same way the datapath is.
  assert {
    condition     = length(module.gke_cell.ip_allocation_policy) == 1
    error_message = "this cell does not request VPC-native networking; a routes-based cluster cannot be converted after creation."
  }

  # A1.6's floor is NOT asserted here on purpose: `core_min_nodes` is a literal in
  # this env's main.tf, not a variable, and the module now carries a
  # `validation { condition = var.core_min_nodes >= 1 }` block — which is
  # evaluated at PLAN time against whatever each env actually passes, so it holds
  # for every present and future env instead of the ones someone remembered to
  # write a test for. Review measured `core_min_nodes = 0` as green everywhere
  # before that validation existed.
}
