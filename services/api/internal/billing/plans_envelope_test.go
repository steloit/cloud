package billing

import "testing"

// THE FOUNDER-APPROVED VALUES, written as literals.
//
// Every other test in this repo that touches the envelope reads plans.json and
// then asserts against what it read — which proves the pipeline is faithful and
// proves NOTHING about the numbers. Changing `business` from 12 to 99 CPU was a
// green change across both modules until this existed.
//
// So this is the one place the numbers are stated independently of the file, and
// it is deliberately a hardcoded table: it is the ruling, not a derivation. If
// plans.json moves, this fails, and the person moving it has to come here and
// say the founder changed their mind.
//
// Ruled by the founder 2026-08-23, per-ENVIRONMENT (docs/founder-config.md §5).
func TestThePlanEnvelopesAreTheFounderApprovedValues(t *testing.T) {
	approved := map[string]Quota{
		"free":       {CPU: "1", Memory: "2Gi", Storage: "10Gi"},
		"pro":        {CPU: "8", Memory: "16Gi", Storage: "100Gi"},
		"business":   {CPU: "12", Memory: "24Gi", Storage: "200Gi"},
		"enterprise": {CPU: "16", Memory: "32Gi", Storage: "250Gi"},
	}
	tbl, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Plans) != len(approved) {
		t.Fatalf("plans.json has %d plans, the ruling covers %d — a new tier needs a founder "+
			"envelope, not a default", len(tbl.Plans), len(approved))
	}
	for plan, want := range approved {
		got, err := tbl.Envelope(plan)
		if err != nil {
			t.Errorf("%s: %v", plan, err)
			continue
		}
		if got != want {
			t.Errorf("%s envelope is %+v, the founder approved %+v. If this changed on purpose, "+
				"update docs/founder-config.md §5 and this table together — plans.json is not "+
				"self-authorising.", plan, got, want)
		}
	}

	// Deny-by-default for anything else.
	for _, unknown := range []string{"", "starter", "standard", "FREE", "enterprise-plus"} {
		if _, err := tbl.Envelope(unknown); err == nil {
			t.Errorf("Envelope(%q) succeeded — an unknown plan must never yield a silent "+
				"envelope, and orgs.plan's CHECK constraint lists exactly four", unknown)
		}
	}
}
