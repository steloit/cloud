package billing

import (
	"testing"

	"github.com/steloit/cloud/services/api/internal/canon"
)

func TestLoadAndInvariants(t *testing.T) {
	tbl, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// the four tiers exist
	for _, name := range []string{"free", "pro", "business", "enterprise"} {
		if _, ok := tbl.Plan(name); !ok {
			t.Fatalf("plan %q missing", name)
		}
	}
	// ratified fees (ADR-041: Pro $29 canonical; Business $99)
	if fee, ok := tbl.PlanFeeCents("free"); !ok || fee != 0 {
		t.Fatalf("free fee = %d,%v (want 0)", fee, ok)
	}
	if fee, ok := tbl.PlanFeeCents("pro"); !ok || fee != 2900 {
		t.Fatalf("pro fee = %d,%v (want 2900)", fee, ok)
	}
	if fee, ok := tbl.PlanFeeCents("business"); !ok || fee != 9900 {
		t.Fatalf("business fee = %d,%v (want 9900)", fee, ok)
	}
	// enterprise is custom-priced — no hard-coded number
	if _, ok := tbl.PlanFeeCents("enterprise"); ok {
		t.Fatal("enterprise must be custom-priced (no fee number)")
	}
	// B5 project allowances
	if got := tbl.ProjectLimit("free"); got != 1 {
		t.Fatalf("free project limit = %d (want 1)", got)
	}
	if got := tbl.ProjectLimit("pro"); got != 3 {
		t.Fatalf("pro project limit = %d (want 3)", got)
	}
	if got := tbl.ProjectLimit("business"); got != -1 {
		t.Fatalf("business project limit = %d (want -1/unlimited)", got)
	}
	// deny-by-default: an unknown plan gets no allowance and no fee
	if tbl.ProjectLimit("nope") != 0 {
		t.Fatal("unknown plan granted a project allowance")
	}
	if _, ok := tbl.PlanFeeCents("nope"); ok {
		t.Fatal("unknown plan returned a fee")
	}
}

// The billing table is the ONE source: its Business fee must equal the canon
// plan_fee_cents (Business $99), so estimate/quota/invoice can't diverge from
// the demo world the console renders.
func TestBillingReconcilesWithCanon(t *testing.T) {
	tbl, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	w, err := canon.Load()
	if err != nil {
		t.Fatal(err)
	}
	fee, _ := tbl.PlanFeeCents("business")
	if int64(fee) != w.Billing.PlanFeeCents {
		t.Fatalf("business fee %d ≠ canon plan_fee_cents %d", fee, w.Billing.PlanFeeCents)
	}
	// and resources + this plan fee == the canon org total (the $482 invariant)
	if w.Billing.ResourcesCents+int64(fee) != w.Billing.ForecastCents {
		t.Fatalf("resources %d + business fee %d ≠ org total %d",
			w.Billing.ResourcesCents, fee, w.Billing.ForecastCents)
	}
}

// Plans gate capabilities, never safety (billing pack / F9). The never-gated
// list is law — assert the safety features are all on it.
func TestSafetyNeverGated(t *testing.T) {
	tbl, _ := Load()
	for _, cap := range []string{"tls", "backups", "mfa", "policies", "alerts", "dunning_protections", "self_deletion"} {
		if !tbl.IsNeverGated(cap) {
			t.Errorf("safety capability %q must be on the never-gated list", cap)
		}
	}
	if tbl.IsNeverGated("ai") {
		t.Error("ai is plan-gated (ai.use is in the matrix) — not on the never-gated safety list")
	}
}
