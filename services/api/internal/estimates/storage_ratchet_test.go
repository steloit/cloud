package estimates

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// The migration that normalises stored storage_gb up to included_gb must not
// move any price. That is the whole reason it is a migration rather than a
// founder decision, and it is asserted by driving the real pricing engine over
// every catalog size — not by restating the three numbers in the SQL comment.
func TestTheStorageRatchetMigrationIsPriceNeutral(t *testing.T) {
	for size, included := range catalogIncluded(t) {
		before, err := Price(ShapeInput{Product: "postgres", Name: "db",
			Shape: map[string]any{"size": size, "storage_gb": 0}})
		if err != nil {
			t.Fatalf("size %q: %v", size, err)
		}
		after, err := Price(ShapeInput{Product: "postgres", Name: "db",
			Shape: map[string]any{"size": size, "storage_gb": included}})
		if err != nil {
			t.Fatalf("size %q: %v", size, err)
		}
		if before.MonthlyCents.Int64() != after.MonthlyCents.Int64() {
			t.Errorf("size %q: raising a stored 0 to the included %d moves the price "+
				"%d → %d. The migration would re-price live customers, which is a founder "+
				"decision, not a data fix.",
				size, included, before.MonthlyCents.Int64(), after.MonthlyCents.Int64())
		}
	}
}

// The migration embeds the included amounts because a migration is a historical
// record — it must keep applying what it applied on the day it ran, whatever the
// catalog says later. This asserts they matched WHEN IT WAS WRITTEN. If the
// catalog changes, the answer is a new migration, not an edit to that file, and
// this test is the thing that says so.
func TestTheStorageRatchetMigrationMatchedTheCatalogWhenWritten(t *testing.T) {
	migration := filepath.Join("..", "platform", "db", "migrations",
		"20260823120000_postgres_storage_ratchet.up.sql")
	raw, err := os.ReadFile(migration)
	if err != nil {
		t.Fatalf("read %s: %v", migration, err)
	}

	// Parse the VALUES list the UPDATE joins against.
	re := regexp.MustCompile(`\('([a-z-]+)',\s*(\d+)\)`)
	found := map[string]int{}
	for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			t.Fatal(err)
		}
		found[m[1]] = n
	}
	if len(found) == 0 {
		t.Fatal("parsed no (size, included_gb) pairs out of the migration — this test would prove nothing")
	}

	catalog := catalogIncluded(t)
	for size, want := range catalog {
		got, ok := found[size]
		if !ok {
			t.Errorf("the catalog has size %q and the migration does not normalise it", size)
			continue
		}
		if got != want {
			t.Errorf("migration normalises %q to %d, catalog says %d — the migration no longer "+
				"describes what it applied; write a NEW migration rather than editing that one",
				size, got, want)
		}
	}
	for size := range found {
		if _, ok := catalog[size]; !ok {
			t.Errorf("the migration normalises %q, which is not a catalog size", size)
		}
	}
}

// included_gb is a DERIVED decomposition, and pricing.json says so: its `$note`
// records "Two decompositions are DERIVED to satisfy canon and carried as S9
// findings: (1) included_gb on standard/performance …".
//
// The derivation it gives covers standard (db-main is $58 with 50 GB, so
// standard must include 50) and dev (db-reports pays $5 for 10 GB, so dev
// includes none). PERFORMANCE IS NAMED IN THE FINDING BUT NOT DERIVED BY IT —
// there is no priced performance fixture in canon, so its 50 is analogy with
// standard. Lowering it re-prices every performance customer by 50c/GB.
//
// This test does not invent an authority. It requires every catalog size to be
// either derivable from canon or listed here with the reason it is not, so
// adding a size forces someone to say where its number came from, and moving an
// analogical one fails loudly instead of silently re-pricing.
func TestEveryIncludedGBHasANamedAuthority(t *testing.T) {
	notDerivedFromCanon := map[string]struct {
		gb  int
		why string
	}{
		"performance": {50, "ANALOGY WITH standard. pricing.json's $note names included_gb on " +
			"standard/performance as one derived S9 finding, but the derivation it states covers " +
			"standard only (db-main $58 @ 50GB); canon prices no performance service. " +
			"docs/founder-config.md §5 records it under the 2026-08-23 ruling, which named standard."},
	}
	derivedFromCanon := map[string]string{
		"dev":      "db-reports pays $5 for 10 GB at $0.50/GB, so dev includes none",
		"standard": "db-main's $58 total with 50 GB requires standard to include 50",
	}

	for size, included := range catalogIncluded(t) {
		if claim, ok := notDerivedFromCanon[size]; ok {
			if claim.gb != included {
				t.Errorf("size %q: pricing.json says included_gb=%d, the recorded claim says %d (%s)",
					size, included, claim.gb, claim.why)
			}
			continue
		}
		if _, ok := derivedFromCanon[size]; !ok {
			t.Errorf("size %q has no named authority for included_gb=%d — derive it from a canon "+
				"fixture, or record it in notDerivedFromCanon with the reason", size, included)
		}
	}
	// The canon-derived numbers are pinned by TestCanonArithmetic / TestEngineRules;
	// this only asserts every size is accounted for by one list or the other.
	for size := range derivedFromCanon {
		if _, ok := catalogIncluded(t)[size]; !ok {
			t.Errorf("%q is claimed as canon-derived but is not a catalog size", size)
		}
	}
}
