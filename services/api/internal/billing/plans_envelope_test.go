package billing

import (
	"os"
	"regexp"
	"testing"
)

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

// THE BACKFILL MIGRATION IS A SECOND COPY OF THE ENVELOPE, AND SQL CANNOT READ
// plans.json. The duplication is unavoidable (a migration is plain SQL, and it
// must be immutable once shipped); what is avoidable is it being SILENT. This
// parses the migration's VALUES list and compares it to the plan table, so a
// wrong digit in the migration is a red test rather than a fleet provisioned to
// the wrong ceiling on the one run that can never be repeated.
//
// It asserts a snapshot, deliberately: it proves the list was right ON THE DAY
// IT WAS WRITTEN. If a future founder ruling changes an envelope, this test goes
// red and the correct response is NOT to edit the shipped migration (already
// applied everywhere it matters) but to add a new one — the failure is the
// prompt to do that, so it is doing its job either way.
func TestTheBackfillMigrationMatchesThePlanTable(t *testing.T) {
	const path = "../platform/db/migrations/20260823140000_service_quota_backfill.up.sql"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the backfill migration is missing — an existing service with no envelope "+
			"cannot converge at all (tenancy.Render refuses it): %v", err)
	}
	sql := string(raw)

	// ('plan', 'cpu', 'memory', 'storage') — anchored on four quoted fields so a
	// row that loses one does not silently match with a shifted meaning.
	row := regexp.MustCompile(`\('([a-z]+)',\s*'([^']+)',\s*'([^']+)',\s*'([^']+)'\)`)
	found := map[string][3]string{}
	for _, m := range row.FindAllStringSubmatch(sql, -1) {
		found[m[1]] = [3]string{m[2], m[3], m[4]}
	}
	if len(found) == 0 {
		t.Fatal("parsed no VALUES rows out of the migration — if its shape changed, this test " +
			"is now asserting nothing, which is worse than not existing")
	}

	table, err := Load()
	if err != nil {
		t.Fatalf("plans.json did not load: %v", err)
	}
	for _, plan := range []string{"free", "pro", "business", "enterprise"} {
		want, err := table.Envelope(plan)
		if err != nil {
			t.Fatalf("plan %q has no envelope in plans.json: %v", plan, err)
		}
		got, ok := found[plan]
		if !ok {
			t.Errorf("plan %q is absent from the backfill — every org on it keeps a service that "+
				"cannot converge", plan)
			continue
		}
		if got != [3]string{want.CPU, want.Memory, want.Storage} {
			t.Errorf("plan %q: the migration would write %v but the plan table grants %v/%v/%v",
				plan, got, want.CPU, want.Memory, want.Storage)
		}
	}
	for plan := range found {
		if _, err := table.Envelope(plan); err != nil {
			t.Errorf("the migration writes an envelope for %q, which is not a plan "+
				"(orgs.plan CHECKs four values) — those rows would be silently skipped", plan)
		}
	}
}
