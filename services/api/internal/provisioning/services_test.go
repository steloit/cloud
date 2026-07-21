package provisioning

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/steloit/cloud/services/api/internal/identity/store"
)

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
		{"failed", "ready"}, // a failed provision re-provisions, it never becomes ready by fiat
		{"suspended", "degraded"},
		{"deleting", "ready"}, {"deleting", "provisioning"}, // deleting is terminal
		{"ready", "running"}, // ADR-024: the word does not exist
		{"running", "ready"}, // unknown FROM states have no edges
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

// TestServiceViewDoesNotLeakReconcilerColumns pins D8/US-1.3: the internal
// desired-state columns (desired, generation, observed_generation, cell_id) must
// never appear in the customer-facing service payload. serviceToAPI selects
// fields explicitly, so a store.Service carrying those columns must marshal to
// JSON without them. A future "just return the row" refactor breaks this test.
func TestServiceViewDoesNotLeakReconcilerColumns(t *testing.T) {
	row := store.Service{
		ID: "svc_x", EnvID: "env_x", Name: "db", Product: "postgres", Status: "ready",
		Shape:              []byte(`{"size":"dev"}`),
		Desired:            []byte(`{"product":"postgres","internal":"substrate detail"}`),
		Generation:         7,
		ObservedGeneration: 7,
		CellID:             "cell-0",
	}
	b, err := json.Marshal(serviceToAPI(row))
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, banned := range []string{"desired", "generation", "observed_generation", "cell_id", "cell-0"} {
		if strings.Contains(js, banned) {
			t.Fatalf("customer service payload leaked internal field %q: %s", banned, js)
		}
	}
}
