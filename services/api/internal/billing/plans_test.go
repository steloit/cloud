package billing

import (
	"slices"
	"sort"
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
	// F9 seat allowances (Free 3 / Pro 5 / Business 20; enterprise 1000)
	for plan, want := range map[string]int{"free": 3, "pro": 5, "business": 20, "enterprise": 1000} {
		if got := tbl.IncludedSeats(plan); got != want {
			t.Errorf("%s seats = %d (want %d)", plan, got, want)
		}
	}
	if tbl.IncludedSeats("nope") != 0 {
		t.Fatal("unknown plan granted seats")
	}
	// F9 soft-overage schedule (exact prices, integer cents)
	if o := tbl.Overage; o.EgressCentsPerGB != 9 || o.SeatCents != 700 || o.BuildCentsPerMin != 1 ||
		o.EventCentsPerMillion != 120 || o.AICentsPer1k != 200 {
		t.Fatalf("overage schedule drifted: %+v", o)
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

// The loader fails loudly on bad data — a typo must never silently ship a $0
// meter, an unlimited allowance, or a missing plan.
func TestLoadRejectsBadData(t *testing.T) {
	good := `{"plans":{"free":{"fee_cents":0,"project_limit":1,"included_seats":3},
		"pro":{"fee_cents":2900,"project_limit":3,"included_seats":5},
		"business":{"fee_cents":9900,"project_limit":-1,"included_seats":20},
		"enterprise":{"fee_cents":null,"project_limit":-1,"included_seats":1000}},
		"overage":{"egress_cents_per_gb":9,"seat_cents":700,"build_cents_per_min":1,"event_cents_per_million":120,"ai_cents_per_1k":200}}`
	if _, err := parse([]byte(good)); err != nil {
		t.Fatalf("valid table rejected: %v", err)
	}
	bad := map[string]string{
		"missing plan":     `{"plans":{"free":{"fee_cents":0,"project_limit":1,"included_seats":3}},"overage":{"egress_cents_per_gb":9,"seat_cents":700,"build_cents_per_min":1,"event_cents_per_million":120,"ai_cents_per_1k":200}}`,
		"zero overage":     `{"plans":{"free":{"fee_cents":0,"project_limit":1,"included_seats":3},"pro":{"fee_cents":2900,"project_limit":3,"included_seats":5},"business":{"fee_cents":9900,"project_limit":-1,"included_seats":20},"enterprise":{"fee_cents":null,"project_limit":-1,"included_seats":1000}},"overage":{"egress_cents_per_gb":9,"seat_cents":0,"build_cents_per_min":1,"event_cents_per_million":120,"ai_cents_per_1k":200}}`,
		"negative limit":   `{"plans":{"free":{"fee_cents":0,"project_limit":-5,"included_seats":3},"pro":{"fee_cents":2900,"project_limit":3,"included_seats":5},"business":{"fee_cents":9900,"project_limit":-1,"included_seats":20},"enterprise":{"fee_cents":null,"project_limit":-1,"included_seats":1000}},"overage":{"egress_cents_per_gb":9,"seat_cents":700,"build_cents_per_min":1,"event_cents_per_million":120,"ai_cents_per_1k":200}}`,
		"negative fee":     `{"plans":{"free":{"fee_cents":-1,"project_limit":1,"included_seats":3},"pro":{"fee_cents":2900,"project_limit":3,"included_seats":5},"business":{"fee_cents":9900,"project_limit":-1,"included_seats":20},"enterprise":{"fee_cents":null,"project_limit":-1,"included_seats":1000}},"overage":{"egress_cents_per_gb":9,"seat_cents":700,"build_cents_per_min":1,"event_cents_per_million":120,"ai_cents_per_1k":200}}`,
	}
	for name, js := range bad {
		if _, err := parse([]byte(js)); err == nil {
			t.Errorf("%s: bad table accepted", name)
		}
	}
}

// Plans gate capabilities, never safety (billing pack / F9; B6: "Safety is never
// gated on any tier"). The never-gated list is LAW — this table-driven invariant
// pins it in BOTH directions so it cannot silently drift: the plans.json list
// must equal the canonical safety set exactly (nothing dropped, nothing extra),
// and capability features must NOT be on it. (US-11.1)
func TestSafetyNeverGated(t *testing.T) {
	tbl, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// The canonical safety set — the seven things a paying-or-not customer always
	// gets: TLS, backups, MFA, policies, alerts, dunning protections, and the
	// ability to leave (self-deletion). Sourced from the billing pack / F9 / B6.
	canonical := []string{"alerts", "backups", "dunning_protections", "mfa", "policies", "self_deletion", "tls"}

	got := append([]string(nil), tbl.NeverGated.Capabilities...)
	sort.Strings(got)
	if !slices.Equal(got, canonical) {
		t.Fatalf("never-gated list DRIFTED from the canonical safety set: canonical=%v plans.json=%v (a dropped entry silently plan-gates safety; an extra entry silently frees a capability)", canonical, got)
	}
	// every safety capability reports never-gated…
	for _, cap := range canonical {
		if !tbl.IsNeverGated(cap) {
			t.Errorf("safety capability %q must be never-gated", cap)
		}
	}
	// …and capability features (sold per tier) are gated, not on the list.
	for _, cap := range []string{"ai", "sso", "audit_export", "byoc", "priority_support"} {
		if tbl.IsNeverGated(cap) {
			t.Errorf("%q is a plan-gated capability — it must NOT be on the never-gated safety list", cap)
		}
	}
}

// TestGateCapabilityNeverGatesSafety — the US-11.1 enforcement path (US-11.2):
// the capability gate consults IsNeverGated BEFORE gating, so no safety
// capability is ever plan-gated at the call site (not just off the list).
func TestGateCapabilityNeverGatesSafety(t *testing.T) {
	tbl, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// every safety capability is permitted on the FREE plan without it being
	// "had" — the gate must never fire for safety.
	for _, cap := range []string{"tls", "backups", "mfa", "policies", "alerts", "dunning_protections", "self_deletion"} {
		if _, gated := tbl.GateCapability("free", cap, false); gated {
			t.Errorf("safety capability %q was plan-gated at the call site — US-11.1 invariant broken", cap)
		}
	}
	// a real capability the plan lacks IS gated (the gate still works for
	// genuine capabilities).
	if rp, gated := tbl.GateCapability("free", "sso", false); !gated || rp != "free" {
		t.Fatalf("a non-safety capability the plan lacks must gate: gated=%v requiredPlan=%q", gated, rp)
	}
	// …but not if the plan already has it.
	if _, gated := tbl.GateCapability("business", "sso", true); gated {
		t.Fatal("a capability the plan HAS must not be gated")
	}
}
