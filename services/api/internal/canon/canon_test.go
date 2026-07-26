package canon

import "testing"

// The Go layer of the three-layer arithmetic invariant (Q2). The estimate
// engine and the invoice generator (E11) both assert THIS against the same
// fixtures; a canon change that breaks the math fails here first.
func TestCanonArithmeticInvariants(t *testing.T) {
	w, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.CheckArithmetic(); err != nil {
		t.Fatalf("canon arithmetic invariant broke: %v", err)
	}
	// spot the ratified anchors so a silently-zeroed fixture can't pass
	if got := w.EcommerceProjectCents(); got != 20800 {
		t.Fatalf("ecommerce project = %d, canon ratifies 20800", got)
	}
	if w.Billing.ResourcesCents != 38300 || w.Billing.ForecastCents != 48200 {
		t.Fatalf("org totals drifted: resources=%d total=%d (want 38300, 48200)",
			w.Billing.ResourcesCents, w.Billing.ForecastCents)
	}
	t.Logf("canon arithmetic holds: %d services, %d projects, org total %d cents",
		len(w.Services), len(w.Projects), w.Billing.ForecastCents)
}
