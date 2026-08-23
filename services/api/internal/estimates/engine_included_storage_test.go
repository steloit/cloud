package estimates

import (
	"encoding/json"
	"testing"
)

// FOUNDER DECISION 2026-08-23: an unset postgres storage_gb means the size's
// included_gb, NOT zero. "The customer-facing pricing definition takes
// precedence."
//
// Resolved at the gate so the contract is explicit: the shape a customer is
// priced on — and the one persisted and shipped to the cell — says 50 rather
// than implying it. The cell-agent is a separate module and must not carry a
// copy of the catalog.
func TestUnsetStorageResolvesToTheSizesIncludedGB(t *testing.T) {
	for size, want := range catalogIncluded(t) {
		got, _, err := resolve(ShapeInput{Product: "postgres", Shape: map[string]any{"size": size}})
		if err != nil {
			t.Fatalf("size %q: %v", size, err)
		}
		if got["storage_gb"] != want {
			t.Errorf("size %q unset storage_gb resolved to %v, want its included_gb %d",
				size, got["storage_gb"], want)
		}
	}

	// An explicit value is NOT overwritten — the customer's declared shape is the
	// contract, and a larger purchase must survive.
	got, _, err := resolve(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "standard", "storage_gb": 78}})
	if err != nil {
		t.Fatal(err)
	}
	if got["storage_gb"] != 78 {
		t.Fatalf("an explicit storage_gb was overwritten: got %v, want 78", got["storage_gb"])
	}

	// A DECLARED value below the included amount resolves UP to it. An earlier
	// version of this test asserted the opposite — that an explicit 0 is
	// preserved — and that was wrong twice over: it left the declared shape
	// saying 0 while the volume was 50, the very declared-vs-provisioned split
	// this task closes; and it broke "the same configuration spelled with its
	// defaults written out must not be refused", which the estimate gate is
	// built on (TestEstimateGateRefusesAPriceCollidingShape spells
	// `{size: standard, ha: false, storage_gb: 0}` and must still match an
	// estimate for `{size: standard}`).
	//
	// You cannot receive less than what the size includes, so 0, 30 and 50 on a
	// standard are ONE contract.
	for _, below := range []int{0, 1, 30, 49} {
		got, _, err = resolve(ShapeInput{Product: "postgres",
			Shape: map[string]any{"size": "standard", "storage_gb": below}})
		if err != nil {
			t.Fatal(err)
		}
		if got["storage_gb"] != 50 {
			t.Fatalf("storage_gb %d on a standard resolved to %v, want the included 50 — "+
				"a customer cannot receive less than the size includes", below, got["storage_gb"])
		}
	}
	// dev includes 0, so 0 stays 0 there — the rule is the catalog's, not a constant.
	got, _, err = resolve(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev", "storage_gb": 0}})
	if err != nil {
		t.Fatal(err)
	}
	if got["storage_gb"] != 0 {
		t.Fatalf("dev includes 0 GB but storage_gb resolved to %v", got["storage_gb"])
	}
}

// The property the estimate gate is built on: the same contract spelled with its
// defaults written out must canonicalise identically. This is what
// TestEstimateGateRefusesAPriceCollidingShape exercises end-to-end, and it is
// what a naive "default only when unset" broke.
func TestSpellingTheDefaultsOutIsTheSameContract(t *testing.T) {
	for _, spelled := range []map[string]any{
		{"size": "standard"},
		{"size": "standard", "storage_gb": 0},
		{"size": "standard", "storage_gb": 50},
		{"size": "standard", "ha": false, "storage_gb": 0},
	} {
		a, err := Canonical(ShapeInput{Product: "postgres", Name: "db", Shape: map[string]any{"size": "standard"}})
		if err != nil {
			t.Fatal(err)
		}
		b, err := Canonical(ShapeInput{Product: "postgres", Name: "db", Shape: spelled})
		if err != nil {
			t.Fatal(err)
		}
		if a != b {
			t.Fatalf("%v is a different contract from {size: standard}:\n  %s\n  %s", spelled, a, b)
		}
	}
	// And a real purchase above the included amount is still distinct.
	a, _ := Canonical(ShapeInput{Product: "postgres", Name: "db", Shape: map[string]any{"size": "standard"}})
	c, err := Canonical(ShapeInput{Product: "postgres", Name: "db",
		Shape: map[string]any{"size": "standard", "storage_gb": 78}})
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("50 GB and 78 GB share an identity")
	}
}

// THE PRICE MUST NOT MOVE. Price charges `storage_gb - included_gb` only when
// positive, so resolving an unset value to included_gb yields exactly zero extra.
// If this ever changes, every existing standard/performance customer is re-priced
// by a defaulting rule, which is a billing incident, not a rendering one.
func TestResolvingIncludedStorageDoesNotChangeThePrice(t *testing.T) {
	for size, included := range catalogIncluded(t) {
		unset, err := Price(ShapeInput{Product: "postgres", Name: "db", Shape: map[string]any{"size": size}})
		if err != nil {
			t.Fatalf("size %q: %v", size, err)
		}
		explicit, err := Price(ShapeInput{Product: "postgres", Name: "db",
			Shape: map[string]any{"size": size, "storage_gb": included}})
		if err != nil {
			t.Fatalf("size %q: %v", size, err)
		}
		if unset.MonthlyCents.Int64() != explicit.MonthlyCents.Int64() {
			t.Errorf("size %q: unset costs %d but storage_gb=%d costs %d — the default moved the price",
				size, unset.MonthlyCents.Int64(), included, explicit.MonthlyCents.Int64())
		}
		// And the base price is still exactly the catalog's base: no extra storage
		// was charged for what is included.
		if want := catalogBase(t)[size]; unset.MonthlyCents.Int64() != want {
			t.Errorf("size %q priced %d, want the catalog base %d — included storage was charged for",
				size, unset.MonthlyCents.Int64(), want)
		}
	}

	// Paying for MORE than the included amount must still cost more, or the
	// assertions above would be satisfied by a Price that ignores storage.
	base, err := Price(ShapeInput{Product: "postgres", Name: "db", Shape: map[string]any{"size": "standard"}})
	if err != nil {
		t.Fatal(err)
	}
	more, err := Price(ShapeInput{Product: "postgres", Name: "db",
		Shape: map[string]any{"size": "standard", "storage_gb": 78}})
	if err != nil {
		t.Fatal(err)
	}
	if more.MonthlyCents.Int64() <= base.MonthlyCents.Int64() {
		t.Fatalf("78 GB (%d) did not cost more than the included 50 (%d)",
			more.MonthlyCents.Int64(), base.MonthlyCents.Int64())
	}
}

// The identity must treat "unset" and "explicitly the included amount" as the
// SAME contract, or a customer who spells out what they were given cannot
// provision the estimate they were quoted.
func TestUnsetAndExplicitIncludedAreTheSameContract(t *testing.T) {
	a, err := Canonical(ShapeInput{Product: "postgres", Name: "db", Shape: map[string]any{"size": "standard"}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Canonical(ShapeInput{Product: "postgres", Name: "db",
		Shape: map[string]any{"size": "standard", "storage_gb": 50}})
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("the same contract has two identities:\n  unset:    %s\n  explicit: %s", a, b)
	}
	// And a genuinely different purchase must still differ.
	c, err := Canonical(ShapeInput{Product: "postgres", Name: "db",
		Shape: map[string]any{"size": "standard", "storage_gb": 78}})
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("50 GB and 78 GB share an identity")
	}
}

func catalogIncluded(t *testing.T) map[string]int {
	t.Helper()
	var doc struct {
		Postgres struct {
			Sizes map[string]struct {
				IncludedGB int `json:"included_gb"`
			} `json:"sizes"`
		} `json:"postgres"`
	}
	if err := json.Unmarshal(pricingJSON, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Postgres.Sizes) == 0 {
		t.Fatal("no postgres sizes in the catalog — this test would prove nothing")
	}
	out := map[string]int{}
	for k, v := range doc.Postgres.Sizes {
		out[k] = v.IncludedGB
	}
	return out
}

func catalogBase(t *testing.T) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for k, v := range table.Postgres.Sizes {
		out[k] = v.BaseCents
	}
	return out
}

// THE WIRE. The resolved shape is marshalled into services.shape and into the
// desired doc, and reaches the cell-agent as JSON — so the int this package
// resolves arrives there as a float64. The driver's reader must accept that, and
// this half pins what it will actually be given.
//
// Verified from the other side by cnpg's TestThePricedStorageGBSizesTheVolume,
// which drives float64 explicitly. Two representations of one value, both
// checked: reading only the Go int here would have proved nothing about the
// bytes the agent sees.
func TestTheResolvedStorageSurvivesTheWireAsANumber(t *testing.T) {
	for size, included := range catalogIncluded(t) {
		resolved, _, err := resolve(ShapeInput{Product: "postgres", Shape: map[string]any{"size": size}})
		if err != nil {
			t.Fatal(err)
		}
		enc, err := json.Marshal(resolved)
		if err != nil {
			t.Fatal(err)
		}
		var overTheWire map[string]any
		if err := json.Unmarshal(enc, &overTheWire); err != nil {
			t.Fatal(err)
		}
		got, ok := overTheWire["storage_gb"].(float64)
		if !ok {
			t.Fatalf("size %q: storage_gb crosses the wire as %T, not a JSON number — the "+
				"driver reads it as float64", size, overTheWire["storage_gb"])
		}
		if int(got) != included {
			t.Errorf("size %q: the wire carries %v GB, want the included %d", size, got, included)
		}
	}
}
