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

func TestDesiredDocShape(t *testing.T) {
	// product + intent + shape + scaling, no deleting flag
	d := desiredDoc("postgres", "database", "acme--prod", []byte(`{"size":"dev"}`), []byte(`{"mode":"auto"}`), nil, false)
	var m map[string]any
	if err := json.Unmarshal(d, &m); err != nil {
		t.Fatal(err)
	}
	if m["product"] != "postgres" || m["intent"] != "database" {
		t.Fatalf("desired doc missing product/intent: %v", m)
	}
	if _, ok := m["shape"]; !ok {
		t.Fatal("desired doc missing shape")
	}
	if _, ok := m["deleting"]; ok {
		t.Fatal("a live service must not carry a deleting flag")
	}
	// deleting flag present when set
	d2 := desiredDoc("postgres", "", "acme--prod", []byte(`{}`), nil, nil, true)
	_ = json.Unmarshal(d2, &m)
	if m["deleting"] != true {
		t.Fatal("deleting flag not set")
	}
	// no substrate names leak (D8)
	for _, banned := range []string{"cnpg", "zfs", "gvisor", "gke"} {
		if strings.Contains(strings.ToLower(string(d)), banned) {
			t.Fatalf("desired doc leaked substrate name %q", banned)
		}
	}
}

func TestDesiredDocEdgeCases(t *testing.T) {
	// nil shape + nil scaling → just product, valid JSON
	d := desiredDoc("postgres", "", "", nil, nil, nil, false)
	var m map[string]any
	if err := json.Unmarshal(d, &m); err != nil {
		t.Fatalf("nil shape/scaling produced invalid JSON: %v", err)
	}
	if len(m) != 1 || m["product"] != "postgres" {
		t.Fatalf("want just product, got %v", m)
	}
	// malformed shape JSON is dropped (not fatal) — pin current behavior
	d2 := desiredDoc("postgres", "", "", []byte("not json"), nil, nil, false)
	_ = json.Unmarshal(d2, &m)
	if _, has := m["shape"]; has {
		t.Fatal("malformed shape must be dropped, not embedded")
	}
	// empty product still produces valid JSON (no panic)
	if !json.Valid(desiredDoc("", "", "", nil, nil, nil, true)) {
		t.Fatal("empty product produced invalid JSON")
	}
}
