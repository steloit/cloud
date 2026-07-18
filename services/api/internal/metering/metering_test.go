package metering

import "testing"

// BillingEdge: metering starts at ready, degraded still bills, everything
// that stops the resource closes the span.
func TestBillingEdge(t *testing.T) {
	cases := []struct{ from, to, want string }{
		{"provisioning", "ready", "open"},
		{"suspended", "ready", "open"},
		{"failed", "provisioning", ""},
		{"ready", "degraded", ""},  // still billing
		{"degraded", "ready", ""},  // still billing
		{"ready", "suspended", "close"},
		{"ready", "deleting", "close"},
		{"degraded", "deleting", "close"},
		{"degraded", "failed", "close"},
		{"provisioning", "failed", ""}, // never billed: failed provisioning never bills (C4)
		{"provisioning", "deleting", ""},
	}
	for _, c := range cases {
		if got := BillingEdge(c.from, c.to); got != c.want {
			t.Fatalf("%s→%s: got %q want %q", c.from, c.to, got, c.want)
		}
	}
}
