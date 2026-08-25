package metering

import (
	"os"
	"regexp"
	"testing"
)

// BillingEdge: metering starts at ready, degraded still bills, everything
// that stops the resource closes the span.
func TestBillingEdge(t *testing.T) {
	cases := []struct{ from, to, want string }{
		{"provisioning", "ready", "open"},
		{"suspended", "ready", "open"},
		{"failed", "provisioning", ""},
		{"ready", "degraded", ""}, // still billing
		{"degraded", "ready", ""}, // still billing
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

// THE TRIPWIRE THAT KEEPS THE RUNTIME GUARD FROM EVER FIRING.
//
// `carryForward` refuses a closed period holding a meter it cannot account for —
// fail-closed, because silently carrying one meter and dropping another is exactly
// the invisible under-billing O39 exists to prevent. But that refusal reaches the
// customer: `GET /orgs/{org}/usage?month=…` calls Rollup on the read path, so a
// second meter would turn viewing any closed month into a 500.
//
// Neither outcome is acceptable, so the tripwire moves to CI. If Rollup ever
// writes a meter `carryForward` does not handle, THIS fails — before it ships,
// rather than on someone's billing page.
func TestRollupWritesOnlyTheMeterCarryForwardCanAccountFor(t *testing.T) {
	src, err := os.ReadFile("rollup.go")
	if err != nil {
		t.Fatal(err)
	}
	// Every meter literal Rollup hands to UpsertQuotaUsage. Read from the source
	// rather than asserted from memory: the point is to notice a NEW one.
	written := regexp.MustCompile(`Meter:\s*(SpanMeter|"[a-z_]+")`).FindAllStringSubmatch(string(src), -1)
	if len(written) == 0 {
		t.Fatal("no meter assignment found in rollup.go — this check would pass vacuously")
	}
	for _, m := range written {
		if m[1] != "SpanMeter" {
			t.Fatalf("rollup.go writes meter %s, which carryForward cannot account for. Late usage "+
				"on it would be silently lost for every closed period. Extend carryForward to "+
				"iterate the frozen rows per meter, then add it here.", m[1])
		}
	}
}
