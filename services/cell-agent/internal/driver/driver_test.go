package driver_test

import (
	"testing"

	"github.com/steloit/cloud/services/cell-agent/internal/driver"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/cnpg"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/valkey"
)

// The T3.4 headline AC: the Driver interface is reused by a second product
// UNCHANGED. CNPG is a BranchingDriver (implements the D2 extension); valkey is
// a plain Driver (no branching). Both satisfy driver.Driver — proven at compile
// time by these assignments, and at runtime by rendering each.
var (
	_ driver.Driver          = (*cnpg.Driver)(nil)
	_ driver.BranchingDriver = (*cnpg.Driver)(nil)
	_ driver.Driver          = (*valkey.Driver)(nil)
)

func TestValkeyDriverReusesInterfaceUnchanged(t *testing.T) {
	// A caller that only knows driver.Driver renders either product.
	drivers := map[string]driver.Driver{"postgres": cnpg.New(), "valkey": valkey.New()}
	for product, d := range drivers {
		if d.Product() != product {
			t.Fatalf("driver.Product() = %q, want %q", d.Product(), product)
		}
		m, err := d.Render(driver.Spec{
			Name: "svcx", Namespace: "proj--prod", Product: product, Cell: "cell-0",
			Shape: map[string]any{"size": "dev"}, Instances: 1,
		})
		if err != nil {
			t.Fatalf("%s render: %v", product, err)
		}
		if len(m) == 0 || m[0].Name != "svcx" {
			t.Fatalf("%s produced no usable manifest: %+v", product, m)
		}
	}
}

// Valkey is NOT a BranchingDriver — a cache does not snapshot-branch or PITR.
// That the type assertion fails is the proof the interface fits products with
// different capabilities, not just CNPG.
func TestValkeyIsNotABranchingDriver(t *testing.T) {
	var d driver.Driver = valkey.New()
	if _, ok := d.(driver.BranchingDriver); ok {
		t.Fatal("valkey must NOT be a BranchingDriver — a cache has no D2 branch primitive")
	}
}

// D8-adjacent: substrate names appear in driver OUTPUT (that is the translation),
// but the driver never requires a substrate name in its INPUT grammar — the Spec
// carries only product/shape/intent, no CNPG/pd/barman terms.
func TestNoSubstrateNamesRequireACustomerSurface(t *testing.T) {
	// The Spec's grammar fields are product-level; a customer never types "cnpg".
	// This is a structural assertion: Render takes a Spec (grammar), not a manifest.
	s := driver.Spec{Name: "svcx", Namespace: "proj--prod", Product: "postgres", Shape: map[string]any{"size": "dev"}}
	if _, err := cnpg.New().Render(s); err != nil {
		t.Fatal(err)
	}
	// (The reverse — output DOES contain substrate names — is correct and covered
	// by the CNPG golden tests; the driver's job is grammar → substrate.)
}
