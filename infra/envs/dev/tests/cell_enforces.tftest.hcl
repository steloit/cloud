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
  # The mock invents random ids for computed attributes, and the provider then
  # rejects its own generated `google_service_account.name` against a client-side
  # regexp whose local part must be 6-30 chars. A PRODUCIBLE default keeps the
  # plan whole; nothing here is asserted.
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
}
