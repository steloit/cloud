package provisioning

import "testing"

// The closed status machine (ADR-024): every edge, both directions checked.
func TestStatusMachine(t *testing.T) {
	legal := [][2]string{
		{"provisioning", "ready"}, {"provisioning", "failed"}, {"provisioning", "deleting"},
		{"ready", "degraded"}, {"ready", "suspended"}, {"ready", "deleting"},
		{"degraded", "ready"}, {"degraded", "failed"}, {"degraded", "deleting"},
		{"failed", "provisioning"}, {"failed", "deleting"},
		{"suspended", "ready"}, {"suspended", "deleting"},
	}
	for _, e := range legal {
		if !CanTransition(e[0], e[1]) {
			t.Fatalf("legal edge rejected: %s → %s", e[0], e[1])
		}
	}
	illegal := [][2]string{
		{"provisioning", "suspended"}, {"provisioning", "degraded"},
		{"ready", "provisioning"}, {"ready", "failed"}, // failure is observed via degraded, never direct from ready
		{"failed", "ready"},     // a failed provision re-provisions, it never becomes ready by fiat
		{"suspended", "degraded"},
		{"deleting", "ready"}, {"deleting", "provisioning"}, // deleting is terminal
		{"ready", "running"},  // ADR-024: the word does not exist
		{"running", "ready"},  // unknown FROM states have no edges
	}
	for _, e := range illegal {
		if CanTransition(e[0], e[1]) {
			t.Fatalf("illegal edge accepted: %s → %s", e[0], e[1])
		}
	}
	// self-loops are never legal
	for from := range transitions {
		if CanTransition(from, from) {
			t.Fatalf("self-loop accepted: %s", from)
		}
	}
}
