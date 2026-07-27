package estimates

// Canon regression: the fixtures file is IMPORTED, never retyped. Every
// canon service shape must price to its canonical monthly_estimate_cents,
// and their sum must be the ratified $208 (20800) family invariant.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/canon"
)

type canonService struct {
	Name                string         `json:"name"`
	Product             string         `json:"product"`
	Shape               map[string]any `json:"shape"`
	MonthlyEstimateCent int64          `json:"monthly_estimate_cents"`
}

func loadCanonServices(t *testing.T) []canonService {
	t.Helper()
	raw, err := os.ReadFile("../../../../docs/product/19-canon/fixtures.json")
	if err != nil {
		t.Fatalf("canon fixtures: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var svcs []canonService
	for key, v := range doc {
		if len(key) >= 8 && key[:8] == "services" {
			if err := json.Unmarshal(v, &svcs); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(svcs) == 0 {
		t.Fatal("no canon services found")
	}
	return svcs
}

func TestCanonArithmetic(t *testing.T) {
	svcs := loadCanonServices(t)
	var total int64
	for _, s := range svcs {
		line, err := Price(ShapeInput{Product: s.Product, Name: s.Name, Shape: s.Shape})
		if err != nil {
			t.Fatalf("canon %s: %v", s.Name, err)
		}
		if line.MonthlyCents != s.MonthlyEstimateCent {
			t.Fatalf("canon %s prices to %d, canon says %d", s.Name, line.MonthlyCents, s.MonthlyEstimateCent)
		}
		total += line.MonthlyCents
	}
	// The $208 anchor is IMPORTED from the shared canon package (Q2), not
	// retyped — the same number the console/invoice layers assert.
	w, err := canon.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := w.EcommerceProjectCents(); total != want {
		t.Fatalf("the $208 invariant broke: priced Σ=%d, canon project total=%d", total, want)
	}
	if err := w.CheckArithmetic(); err != nil {
		t.Fatalf("canon arithmetic invariant broke: %v", err)
	}
	t.Logf("canon reproduced: %d services, Σ=%d cents (canon-verified)", len(svcs), total)
}

// checkout-stack (canon T3 capture example): api + worker + jobs + cache = $126.
func TestCheckoutStackTemplate(t *testing.T) {
	svcs := loadCanonServices(t)
	want := map[string]bool{"api": true, "worker": true, "jobs": true, "cache": true}
	var in []ShapeInput
	for _, s := range svcs {
		if want[s.Name] {
			in = append(in, ShapeInput{Product: s.Product, Name: s.Name, Shape: s.Shape})
		}
	}
	if len(in) != 4 {
		t.Fatalf("expected 4 checkout-stack services, got %d", len(in))
	}
	_, total, err := PriceAll(in)
	if err != nil {
		t.Fatal(err)
	}
	if total != 12600 {
		t.Fatalf("checkout-stack must be $126 (12600), got %d", total)
	}
}

func TestEngineRules(t *testing.T) {
	// server-defaulted intents (S11)
	l, err := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev"}})
	if err != nil || l.Intent != "database" {
		t.Fatalf("postgres default intent: %+v %v", l, err)
	}
	if l, _ = Price(ShapeInput{Product: "valkey", Shape: map[string]any{}}); l.Intent != "cache" {
		t.Fatalf("valkey default intent: %+v", l)
	}
	if l, _ = Price(ShapeInput{Product: "web", Shape: map[string]any{}}); l.Intent != "app" {
		t.Fatalf("web default intent: %+v", l)
	}
	// explicit intent survives (jobs rides postgres)
	if l, _ = Price(ShapeInput{Product: "postgres", Intent: "jobs", Shape: map[string]any{"size": "dev", "storage_gb": 4}}); l.Intent != "jobs" || l.MonthlyCents != 2100 {
		t.Fatalf("jobs line: %+v", l)
	}
	// HA is +$19 (C2)
	base, _ := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev"}})
	ha, _ := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev", "ha": true}})
	if ha.MonthlyCents-base.MonthlyCents != 1900 {
		t.Fatalf("HA delta: %d", ha.MonthlyCents-base.MonthlyCents)
	}
	// unknown product / size fail loudly with the field named
	if _, err := Price(ShapeInput{Product: "gpu"}); err == nil {
		t.Fatal("gpu must be rejected (removed from the surface)")
	}
	if _, err := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "mega"}}); err == nil {
		t.Fatal("unknown size accepted")
	}
	// Dev pays for every GB; Standard's canon-derived 50 GB allowance holds
	devTen, _ := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev", "storage_gb": 10}})
	if devTen.MonthlyCents != 2400 {
		t.Fatalf("dev+10GB must be $24: %d", devTen.MonthlyCents)
	}
	stdSixty, _ := Price(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "standard", "storage_gb": 60}})
	if stdSixty.MonthlyCents != 5800+10*50 {
		t.Fatalf("standard+60GB: %d", stdSixty.MonthlyCents)
	}
}

// TestEstimateLineGrammar — US-11.6: the estimate side of the unified §74 line
// rule. Every estimate line is a stated allowance ("fixed") or a metered
// projection ("usage_projection") — nothing else — with integer cents, and the
// lines sum to the total. This is the estimate-layer mirror of the invoice
// grammar test so "quoted == billed" holds the SAME grammar at both layers.
func TestEstimateLineGrammar(t *testing.T) {
	svcs := loadCanonServices(t)
	shapes := make([]ShapeInput, 0, len(svcs))
	for _, s := range svcs {
		shapes = append(shapes, ShapeInput{Product: s.Product, Name: s.Name, Shape: s.Shape})
	}
	lines, total, err := PriceAll(shapes)
	if err != nil {
		t.Fatalf("PriceAll: %v", err)
	}
	var sum int64
	for _, l := range lines {
		// the closed §74 basis vocabulary — the estimate's plan-vs-metered split,
		// mapping to the invoice's plan:*/meter:* usage_ref (same rule, two encodings).
		if l.Basis != "fixed" && l.Basis != "usage_projection" {
			t.Fatalf("estimate line %q violates the §74 grammar: basis=%q (want fixed|usage_projection)", l.Name, l.Basis)
		}
		if l.MonthlyCents < 0 { // integer cents (int64) end-to-end (ADR-025)
			t.Fatalf("estimate line %q has negative cents: %d", l.Name, l.MonthlyCents)
		}
		sum += l.MonthlyCents
	}
	if sum != total {
		t.Fatalf("estimate lines do not sum to the total: Σ %d ≠ %d", sum, total)
	}
	if len(lines) == 0 {
		t.Fatal("no estimate lines checked — the grammar assertion is inert")
	}
}

// US-3.7: Canonical is the CONFIGURATION identity, so equal prices must not
// make two different configurations interchangeable, and one configuration
// spelled two ways must not look like two.
func TestCanonicalIdentity(t *testing.T) {
	t.Run("colliding prices are DIFFERENT configurations", func(t *testing.T) {
		dev78 := ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev", "storage_gb": 78}}
		standard := ShapeInput{Product: "postgres", Shape: map[string]any{"size": "standard"}}
		// The collision is real — if this ever stops holding, the test below
		// proves nothing and must be re-based on a live collision.
		a, err := Price(dev78)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Price(standard)
		if err != nil {
			t.Fatal(err)
		}
		if a.MonthlyCents != b.MonthlyCents {
			t.Fatalf("the fixture is not a price collision any more (%d vs %d) — pick a live one", a.MonthlyCents, b.MonthlyCents)
		}
		ca, err := Canonical(dev78)
		if err != nil {
			t.Fatal(err)
		}
		cb, err := Canonical(standard)
		if err != nil {
			t.Fatal(err)
		}
		if ca == cb {
			t.Fatalf("dev+78GB and standard share the identity %q — a caller could price one and provision the other", ca)
		}
	})

	t.Run("the same configuration spelled differently is ONE identity", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			a, b ShapeInput
		}{
			{"omitted default vs explicit",
				ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev"}},
				ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev", "ha": false, "storage_gb": 0}}},
			{"omitted intent vs its default",
				ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev"}},
				ShapeInput{Product: "postgres", Intent: defaultIntent("postgres"), Shape: map[string]any{"size": "dev"}}},
			{"name is not part of the contract",
				ShapeInput{Product: "postgres", Name: "orders", Shape: map[string]any{"size": "dev"}},
				ShapeInput{Product: "postgres", Name: "anything-else", Shape: map[string]any{"size": "dev"}}},
			{"valkey default memory",
				ShapeInput{Product: "valkey", Shape: map[string]any{}},
				ShapeInput{Product: "valkey", Shape: map[string]any{"memory_mb": 1024}}},
			{"web default instances",
				ShapeInput{Product: "web", Shape: map[string]any{"size": "standard-1"}},
				ShapeInput{Product: "web", Shape: map[string]any{"size": "standard-1", "instances": 1}}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ca, err := Canonical(tc.a)
				if err != nil {
					t.Fatal(err)
				}
				cb, err := Canonical(tc.b)
				if err != nil {
					t.Fatal(err)
				}
				if ca != cb {
					t.Fatalf("the same configuration produced two identities:\n  %q\n  %q\nthis would refuse a legitimate create", ca, cb)
				}
			})
		}
	})

	t.Run("every priced difference is an identity difference", func(t *testing.T) {
		// Whatever the engine reads to compute a price must be part of the
		// identity — otherwise a field could change the bill without changing
		// the contract.
		base := ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev", "storage_gb": 10, "ha": false}}
		baseID, _ := Canonical(base)
		basePrice, _ := Price(base)
		for _, v := range []map[string]any{
			{"size": "standard", "storage_gb": 10, "ha": false},
			{"size": "dev", "storage_gb": 11, "ha": false},
			{"size": "dev", "storage_gb": 10, "ha": true},
		} {
			other := ShapeInput{Product: "postgres", Shape: v}
			id, err := Canonical(other)
			if err != nil {
				t.Fatal(err)
			}
			p, err := Price(other)
			if err != nil {
				t.Fatal(err)
			}
			if p.MonthlyCents != basePrice.MonthlyCents && id == baseID {
				t.Fatalf("%v prices differently (%d vs %d) but shares the identity %q — the bill can change without the contract changing",
					v, p.MonthlyCents, basePrice.MonthlyCents, id)
			}
		}
	})

	t.Run("an unknown field cannot widen the identity", func(t *testing.T) {
		_, err := Canonical(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev", "wibble": 1}})
		if err == nil {
			t.Fatal("an out-of-schema field was accepted into the configuration identity")
		}
	})
}

// US-3.7 round 2: every DECLARED field is part of the contract, not only the
// billed ones. A customer who priced `pgmq` off did not contract for it on,
// however equal the bill.
func TestCanonicalCoversEveryDeclaredField(t *testing.T) {
	for product, fields := range shapeSchema {
		for key, spec := range fields {
			t.Run(product+"/"+key, func(t *testing.T) {
				base := ShapeInput{Product: product, Shape: map[string]any{}}
				other := ShapeInput{Product: product, Shape: map[string]any{key: differentValue(spec)}}
				a, err := Canonical(base)
				if err != nil {
					t.Fatal(err)
				}
				b, err := Canonical(other)
				if err != nil {
					t.Fatal(err)
				}
				if a == b {
					t.Fatalf("%s.%s does not affect the contract identity (%q) — it can be substituted freely", product, key, a)
				}
			})
		}
	}
}

// differentValue returns a value distinct from the field's default.
func differentValue(spec fieldSpec) any {
	switch spec.kind {
	case "string":
		if spec.def == "zzz-other" {
			return "zzz-other2"
		}
		return "zzz-other"
	case "int":
		return spec.def.(int) + 7
	case "bool":
		return !spec.def.(bool)
	default: // opaque
		return map[string]any{"substituted": true}
	}
}

// A known key carrying the wrong TYPE must be refused, not silently defaulted:
// defaulting priced `{storage_gb: "78"}` to 0 GB bills for a shape nobody asked
// for, and the raw value used to be persisted verbatim.
func TestWrongTypedFieldIsRefusedNotDefaulted(t *testing.T) {
	for _, tc := range []struct {
		name  string
		shape map[string]any
	}{
		{"int as string", map[string]any{"storage_gb": "78"}},
		{"bool as string", map[string]any{"ha": "true"}},
		{"string as number", map[string]any{"size": 3}},
		{"fractional int", map[string]any{"storage_gb": 10.9}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Price(ShapeInput{Product: "postgres", Shape: tc.shape}); err == nil {
				t.Fatalf("%v was accepted and silently defaulted — it would be billed as something else", tc.shape)
			}
			if _, err := Canonical(ShapeInput{Product: "postgres", Shape: tc.shape}); err == nil {
				t.Fatalf("%v produced a contract identity despite being ill-typed", tc.shape)
			}
		})
	}
}

// Resolve is what gets persisted, so it must carry every declared field with
// defaults made explicit.
func TestResolveMakesDefaultsExplicit(t *testing.T) {
	got, err := Resolve(ShapeInput{Product: "postgres", Shape: map[string]any{"size": "dev"}})
	if err != nil {
		t.Fatal(err)
	}
	for k := range shapeSchema["postgres"] {
		if _, ok := got[k]; !ok {
			t.Fatalf("resolved shape omits declared field %q — the stored configuration would be implicit", k)
		}
	}
	if got["storage_gb"] != 0 || got["ha"] != false {
		t.Fatalf("defaults not applied: %v", got)
	}
}

// A product declared in shapeSchema but missing a pricing arm must ERROR, not
// price at zero. resolve() owns the unknown-product check, so without an
// explicit guard the two tables drift in the mis-billing direction.
func TestDeclaredButUnpricedProductIsRefused(t *testing.T) {
	shapeSchema["ghost"] = map[string]fieldSpec{"size": {"string", "small"}}
	t.Cleanup(func() {
		delete(shapeSchema, "ghost")
		delete(allowedShapeKeys, "ghost")
	})
	allowedShapeKeys["ghost"] = map[string]bool{"size": true}

	// The refusal must come from the switch's own default arm — not from a
	// side table asserting which products are priced, which would only narrow
	// the gap rather than close it.
	line, err := Price(ShapeInput{Product: "ghost", Shape: map[string]any{"size": "small"}})
	if err == nil {
		t.Fatalf("a declared product with no pricing arm was priced at %d cents instead of refused", line.MonthlyCents)
	}
	var se ShapeError
	if !errors.As(err, &se) || se.Field != "product" {
		t.Fatalf("want a product ShapeError, got %v", err)
	}
}

// The schema's own well-formedness, and the round trip that makes
// TestCanonicalCoversEveryDeclaredField actually self-extending.
//
// That test compares two identities and passes as long as they DIFFER — which a
// silently-dropped field still satisfies (absent → default, present → dropped).
// This asserts the stronger property: every declared field survives resolve()
// carrying the value it was given.
func TestEveryDeclaredKindIsHandledAndRoundTrips(t *testing.T) {
	known := map[string]bool{"string": true, "int": true, "bool": true, "opaque": true}
	for product, fields := range shapeSchema {
		for key, spec := range fields {
			t.Run(product+"/"+key, func(t *testing.T) {
				if !known[spec.kind] {
					t.Fatalf("%s.%s declares kind %q, which resolve() does not handle — the field would be dropped from the price AND the identity", product, key, spec.kind)
				}
				want := differentValue(spec)
				got, err := Resolve(ShapeInput{Product: product, Shape: map[string]any{key: want}})
				if err != nil {
					t.Fatalf("resolving %s.%s = %v: %v", product, key, want, err)
				}
				have, present := got[key]
				if !present {
					t.Fatalf("%s.%s was DROPPED by resolve — it cannot reach the price or the identity", product, key)
				}
				if fmt.Sprintf("%v", have) != fmt.Sprintf("%v", want) {
					t.Fatalf("%s.%s round-tripped as %v, want %v", product, key, have, want)
				}
			})
		}
	}
}

// Every product must price with an EMPTY shape, which pins every default at
// once: a default that stops being a valid catalog value fails here.
func TestEveryProductPricesWithAnEmptyShape(t *testing.T) {
	for product := range shapeSchema {
		t.Run(product, func(t *testing.T) {
			line, err := Price(ShapeInput{Product: product, Shape: map[string]any{}})
			if err != nil {
				t.Fatalf("%s cannot be priced with defaults alone: %v — a declared default is not a valid catalog value", product, err)
			}
			if line.MonthlyCents <= 0 {
				t.Fatalf("%s prices at %d with defaults — a zero floor means a default silently costs nothing", product, line.MonthlyCents)
			}
		})
	}
}

// An UNSET unpriced field is not the same contract as a set one. The comment on
// shapeSchema states this invariant; nothing was asserting it, so changing any
// unpriced default away from its zero value went unnoticed.
func TestUnsetUnpricedFieldIsADifferentContract(t *testing.T) {
	// "Unpriced" is DERIVED, not listed: a field is unpriced when changing it
	// leaves the price unchanged. Selecting by "its default looks like a zero
	// value" would skip exactly the field whose default was wrongly changed.
	unpriced := func(product, key string, spec fieldSpec) bool {
		base, err := Price(ShapeInput{Product: product, Shape: map[string]any{}})
		if err != nil {
			return false
		}
		other, err := Price(ShapeInput{Product: product, Shape: map[string]any{key: differentValue(spec)}})
		if err != nil {
			return false
		}
		return base.MonthlyCents == other.MonthlyCents
	}
	for product, fields := range shapeSchema {
		for key, spec := range fields {
			if !unpriced(product, key, spec) {
				continue
			}
			t.Run(product+"/"+key, func(t *testing.T) {
				// An unpriced field's default MUST be the zero value: any other
				// default silently means "the customer asked for this", so
				// pricing `{}` and creating with the field set would match.
				switch spec.kind {
				case "string":
					if spec.def != "" {
						t.Fatalf("%s.%s is unpriced but defaults to %q — an unset field would be treated as a request for it", product, key, spec.def)
					}
				case "opaque":
					if spec.def != nil {
						t.Fatalf("%s.%s is unpriced but defaults to %v, not nil", product, key, spec.def)
					}
				}
				empty, err := Canonical(ShapeInput{Product: product, Shape: map[string]any{}})
				if err != nil {
					t.Fatal(err)
				}
				set, err := Canonical(ShapeInput{Product: product, Shape: map[string]any{key: differentValue(spec)}})
				if err != nil {
					t.Fatal(err)
				}
				if empty == set {
					t.Fatalf("%s.%s unset and set share the identity %q — pricing {} and creating with it are different contracts", product, key, empty)
				}
			})
		}
	}
}

// Product and intent are part of the contract. `web` and `worker` share their
// entire schema and default intent, so without the product in the identity they
// are byte-identical — and the old gate's explicit product comparison was
// deleted in favour of Canonical.
func TestProductAndIntentAreInTheIdentity(t *testing.T) {
	t.Run("product", func(t *testing.T) {
		a, err := Canonical(ShapeInput{Product: "web", Shape: map[string]any{"size": "standard-1"}})
		if err != nil {
			t.Fatal(err)
		}
		b, err := Canonical(ShapeInput{Product: "worker", Shape: map[string]any{"size": "standard-1"}})
		if err != nil {
			t.Fatal(err)
		}
		if a == b {
			t.Fatalf("web and worker share the identity %q — one could be provisioned from the other's estimate", a)
		}
	})
	t.Run("intent", func(t *testing.T) {
		a, err := Canonical(ShapeInput{Product: "postgres", Intent: "database", Shape: map[string]any{"size": "dev"}})
		if err != nil {
			t.Fatal(err)
		}
		b, err := Canonical(ShapeInput{Product: "postgres", Intent: "jobs", Shape: map[string]any{"size": "dev"}})
		if err != nil {
			t.Fatal(err)
		}
		if a == b {
			t.Fatalf("two intents share the identity %q — intent is persisted and shipped to the cell, so it is contracted", a)
		}
	})
}

// The opaque identity must distinguish TYPE, not just rendering. %v collapses
// map[dlq:true] and the string "map[dlq:true]" into the same bytes.
func TestOpaqueIdentityDistinguishesType(t *testing.T) {
	id := func(v any) string {
		t.Helper()
		c, err := Canonical(ShapeInput{Product: "postgres", Shape: map[string]any{"pgmq": v}})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	asMap := id(map[string]any{"dlq": true})
	asString := id("map[dlq:true]")
	asSlice := id([]any{"dlq", true})
	if asMap == asString || asMap == asSlice || asString == asSlice {
		t.Fatalf("opaque values of different TYPES share an identity:\n map:    %s\n string: %s\n slice:  %s", asMap, asString, asSlice)
	}
	// ...while key order within a map must NOT matter.
	one := id(map[string]any{"a": 1, "b": 2})
	two := id(map[string]any{"b": 2, "a": 1})
	if one != two {
		t.Fatalf("key order changed the identity:\n %s\n %s", one, two)
	}
}

// The identity is a `|`-delimited string in which intent is written UNESCAPED
// (shape values go through json.Marshal, which escapes). That is only safe
// because intent is now constrained to the catalog — so this asserts the
// constraint holds AND that no catalog value could forge a delimiter.
//
// If intent ever becomes free-form again, this fails, which is the point:
// the escaping argument and the validation are the same guarantee.
func TestIntentCannotForgeAnIdentityDelimiter(t *testing.T) {
	if len(catalogIntents) == 0 {
		t.Fatal("the catalog is empty — every intent would be refused")
	}
	for intent := range catalogIntents {
		if strings.ContainsAny(intent, "|=\"\\") {
			t.Fatalf("catalog intent %q contains an identity delimiter — it could forge a collision", intent)
		}
	}
	// And an intent carrying delimiters is refused before it can be rendered.
	for _, forged := range []string{"database|size=performance", "app=x", `db"`} {
		if _, err := Canonical(ShapeInput{Product: "postgres", Intent: forged, Shape: map[string]any{"size": "dev"}}); err == nil {
			t.Fatalf("intent %q was accepted into the identity unescaped", forged)
		}
	}
}

// The VALUE of every default, asserted as an explicit statement.
//
// TestEveryProductPricesWithAnEmptyShape only requires "no error, positive
// price", which a DIFFERENT valid catalog value also satisfies — so swapping
// postgres's default size from dev to standard kept the suite green while
// silently changing what an omitted field means (2400¢ → 5800¢, and a
// standard cluster shipped to the cell).
//
// This is not a substitution hole — both sides of the gate resolve the same —
// but it is mis-billing and mis-provisioning. There is no external authority to
// assert against: openapi.yaml documents no shape defaults, and every canon
// fixture spells size/instances explicitly, so canon never exercises one.
// The pin therefore has to be a second deliberate statement: changing a default
// must force a visible edit here.
func TestResolvedDefaultsAreExactlyTheDeclaredConfiguration(t *testing.T) {
	want := map[string]map[string]any{
		"postgres": {"size": "dev", "storage_gb": 0, "ha": false,
			"connections": nil, "pgmq": nil, "version": ""},
		"valkey": {"memory_mb": 1024, "eviction": "", "mode": ""},
		"web":    {"size": "standard-1", "instances": 1, "health_check": ""},
		"worker": {"size": "standard-1", "instances": 1, "health_check": ""},
	}
	if len(want) != len(shapeSchema) {
		t.Fatalf("this test covers %d products, the schema declares %d — a new product needs its defaults stated here", len(want), len(shapeSchema))
	}
	for product, expected := range want {
		t.Run(product, func(t *testing.T) {
			got, err := Resolve(ShapeInput{Product: product, Shape: map[string]any{}})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(expected) {
				t.Fatalf("resolved %d fields, expected %d:\n got:  %v\n want: %v", len(got), len(expected), got, expected)
			}
			for k, wv := range expected {
				gv, ok := got[k]
				if !ok {
					t.Fatalf("%s.%s missing from the resolved defaults", product, k)
				}
				if fmt.Sprintf("%v", gv) != fmt.Sprintf("%v", wv) {
					t.Fatalf("%s.%s defaults to %v, this test declares %v — changing a default changes what an OMITTED field means, so it must be a deliberate edit here", product, k, gv, wv)
				}
			}
		})
	}
}

// The instance count is bounded — US-3.8's one inseparable arithmetic guard,
// because a pin's price reaches metering.Rollup through repriceSpan, and a
// wrapped price is a DECREASE, which the spend cap does not enforce on.
//
// The postgres and valkey arms of the same switch wrap identically and are NOT
// fixed here: they are pre-existing, reachable with no pin involved, and they
// belong to O16 with the money.Cents type that makes the class uncompilable.
func TestThePricedInstanceCountRefusesWhatItCannotRepresent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      ShapeInput
		wantFld string
	}{
		{"web instances", ShapeInput{Product: "web", Name: "a",
			Shape: map[string]any{"size": "standard-1", "instances": int(1) << 60}}, "shape.instances"},
		{"worker instances", ShapeInput{Product: "worker", Name: "w",
			Shape: map[string]any{"size": "standard-1", "instances": int(1) << 60}}, "shape.instances"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line, err := Price(tc.in)
			if err == nil {
				t.Fatalf("accepted, priced at %d — money is integer cents (ADR-025), and a wrapped price also disables the spend cap, because the cap only enforces on an increase", line.MonthlyCents)
			}
			var se ShapeError
			if !errors.As(err, &se) || se.Field != tc.wantFld {
				t.Fatalf("want a ShapeError naming %s, got %v", tc.wantFld, err)
			}
		})
	}
}

// The instance ceiling sits where REPRESENTABILITY stops, not where a business
// decision would put it — and the boundary is asserted from both sides.
//
// Four mutations survived the branch's own tests before this existed: dividing
// the ceiling by 1000, moving `>` to `>=`, and dropping `- ServiceBaseCents`.
// The suite pinned the refuse side only at 1<<60 and MaxInt64 and the accept
// side only at 1, 3, 9 and 1000 — so the frontier was unconstrained across six
// orders of magnitude, and the engine could silently acquire the commercial
// ceiling its own comment forbids it from having.
//
// Every expectation is DERIVED from the embedded pricing table and from
// maxMonthlyCents, never retyped, and never by calling maxPriceableInstances —
// asserting a function against itself is the mirror this repo already records
// against a test that re-implemented the guard it was named for.
func TestTheInstanceCeilingIsRepresentabilityNotPolicy(t *testing.T) {
	for _, product := range []string{"web", "worker"} {
		sizes := table.Web.Sizes
		if product == "worker" {
			sizes = table.Worker.Sizes
		}
		for size, sz := range sizes {
			if sz.InstanceCents <= 0 {
				// LOUD, not skipped. maxPriceableInstances marks this arm as
				// unreachable from the shipped catalog; the day it becomes
				// reachable is the day that mark stops being true, and a silent
				// `continue` would drop this size's coverage at exactly that
				// moment without saying so.
				t.Fatalf("%s/%s prices at 0 per instance — the unreachable arm in maxPriceableInstances is now LIVE, so its comment is stale and this test no longer covers this size", product, size)
			}
			max := (maxMonthlyCents - sz.ServiceBaseCents) / sz.InstanceCents
			t.Run(product+"/"+size, func(t *testing.T) {
				// A ceiling low enough to be a product decision is a product
				// decision, whatever the comment says. The floor is DERIVED from
				// the catalog's cheapest priced instance rather than a literal:
				// a literal 1e9 trips the moment an expensive size is added
				// (instance_cents > 3443), which would report "commercial
				// ceiling" for what is really "we added a bigger machine". The
				// message names both readings, because the test cannot tell them
				// apart and should not pretend to.
				floor := maxMonthlyCents / (cheapestInstanceCents() * 1000)
				if max < floor {
					t.Fatalf("%s/%s refuses above %d instances (floor %d) — either the ceiling has become a commercial limit, which the engine does not impose (founder, 2026-07-27), or this size is expensive enough that the floor needs re-deriving. Decide which; do not just lower the floor.",
						product, size, max, floor)
				}
				at, err := Price(ShapeInput{Product: product, Name: "x",
					Shape: map[string]any{"size": size, "instances": int(max)}})
				if err != nil {
					t.Fatalf("the largest representable count (%d) was refused: %v", max, err)
				}
				want := sz.ServiceBaseCents + max*sz.InstanceCents
				if at.MonthlyCents != want {
					t.Fatalf("price at the boundary = %d, want %d", at.MonthlyCents, want)
				}
				// The whole reason for the derivation: this price is multiplied
				// by a month of seconds in metering.Rollup.
				if at.MonthlyCents <= 0 || at.MonthlyCents*secondsInLongestMonth <= 0 {
					t.Fatalf("the boundary price %d does not survive a billing month", at.MonthlyCents)
				}
				if _, err := Price(ShapeInput{Product: product, Name: "x",
					Shape: map[string]any{"size": size, "instances": int(max + 1)}}); err == nil {
					t.Fatalf("one past the boundary (%d) was accepted", max+1)
				}
			})
		}
	}
}

// PriceWithInstances is exported API added by US-3.8 and had no package-level
// test — its only direct caller in test code used it to compute its own
// expectation, which proves nothing about it.
func TestAPinIsPriceableOnlyWhereTheShapeDeclaresInstances(t *testing.T) {
	for _, tc := range []struct {
		product string
		shape   map[string]any
		ok      bool
	}{
		{"postgres", map[string]any{"size": "dev"}, false},
		{"valkey", map[string]any{"memory_mb": 1024}, false},
		{"web", map[string]any{"size": "standard-1"}, true},
		{"worker", map[string]any{"size": "standard-1"}, true},
	} {
		t.Run(tc.product, func(t *testing.T) {
			line, err := PriceWithInstances(ShapeInput{Product: tc.product, Name: "x", Shape: tc.shape}, 3)
			if !tc.ok {
				var se ShapeError
				if !errors.As(err, &se) || se.Field != "override.instances" {
					t.Fatalf("a pin on %s must be refused naming override.instances, got %v / %v", tc.product, line.MonthlyCents, err)
				}
				// The DETAIL is the contract, not just the field. Removing the
				// priced-field check still produces an override.instances error —
				// `resolve` rejects `instances` as an unknown key for these
				// products — but the message becomes "unknown shape field", which
				// tells the operator their request was malformed rather than that
				// the capacity cannot be METERED. The founder ruling (2026-07-27)
				// is that the refusal exists because the catalog cannot price the
				// capacity; a message that says otherwise misreports why.
				if !strings.Contains(se.Detail, "metered") {
					t.Fatalf("a pin on %s is refused with %q — it must say the capacity could not be METERED, which is the reason it is refused, not that the field is unknown", tc.product, se.Detail)
				}
				return
			}
			if err != nil {
				t.Fatalf("a pin on %s must price: %v", tc.product, err)
			}
			sizes := table.Web.Sizes
			if tc.product == "worker" {
				sizes = table.Worker.Sizes
			}
			sz := sizes["standard-1"]
			if want := sz.ServiceBaseCents + 3*sz.InstanceCents; line.MonthlyCents != want {
				t.Fatalf("pinned price = %d, want %d", line.MonthlyCents, want)
			}
		})
	}
}

// The billing-month constant is pinned against the arithmetic that actually
// multiplies it — not against itself.
//
// `secondsInLongestMonth` had one representation in `engine.go` and another in
// `metering.Rollup`, which multiplies a rate by the REAL elapsed seconds of a
// period. Changing the constant from 31 days to 30 survived every package,
// because the ceiling and the test that checked it moved together. The
// consequence was not academic: a 30-day constant admits a rate of
// 3,558,399,704,200 cents/month, and multiplying that by a real 31-day period
// gives 9.53e18, which wraps to -8,915,926,305,980,271,616 in
// `weighted += secs * rate` — persisted as `quota_usage.rate_cents`, the number
// billing derives charges from.
//
// So this derives the longest real period from the same `AddDate(0, 1, 0)`
// arithmetic `metering.periodBounds` uses, across a leap year and a non-leap
// year, and asserts the constant covers it. Two representations, one invariant,
// and now the test depends on the one the constant does not control.
func TestTheBillingMonthConstantCoversTheLongestRealPeriod(t *testing.T) {
	var longest int64
	var when string
	for _, year := range []int{2024, 2026} { // leap and non-leap
		for month := 1; month <= 12; month++ {
			start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
			// The same expression periodBounds uses to find a period's end.
			secs := int64(start.AddDate(0, 1, 0).Sub(start).Seconds())
			if secs > longest {
				longest, when = secs, start.Format("2006-01")
			}
		}
	}
	if secondsInLongestMonth < longest {
		t.Fatalf("secondsInLongestMonth is %d but %s is %d seconds — the ceiling would admit a rate whose real-month product wraps in metering.Rollup's `weighted += secs * rate`",
			secondsInLongestMonth, when, longest)
	}
	// And the ceiling it produces genuinely survives that period.
	if got := maxMonthlyCents * longest; got <= 0 || got/longest != maxMonthlyCents {
		t.Fatalf("the maximum accepted rate wraps across %s: %d × %d = %d", when, maxMonthlyCents, longest, got)
	}
}

// cheapestInstanceCents is the lowest per-instance rate in the shipped catalog,
// so the commercial-ceiling floor scales with the catalog instead of with a
// literal that a new size invalidates.
func cheapestInstanceCents() int64 {
	cheapest := int64(0)
	for _, sizes := range []map[string]computeSize{table.Web.Sizes, table.Worker.Sizes} {
		for _, sz := range sizes {
			if sz.InstanceCents > 0 && (cheapest == 0 || sz.InstanceCents < cheapest) {
				cheapest = sz.InstanceCents
			}
		}
	}
	return cheapest
}
