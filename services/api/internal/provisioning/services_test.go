package provisioning

import (
	"encoding/json"
	"github.com/steloit/cloud/services/api/internal/metering"
	"strings"
	"testing"
	"time"

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

// overrideInstances is the whole read-side expiry contract, and two of US-3.8's
// acceptance criteria rest entirely on it: "a pin with NO expiry is not
// honoured" and "an expired pin is never shipped to the cell even before the
// sweep runs". Every one of its five liveness branches previously survived
// mutation — nothing executed the function at all.
func TestOverrideLiveness(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339Nano) }

	for _, tc := range []struct {
		name     string
		raw      string
		wantN    int
		wantLive bool
		why      string
	}{
		{"absent", "", 0, false, "no override at all"},
		{"empty object", `{}`, 0, false, "no instances"},
		{"malformed json", `{not json`, 0, false, "unparseable"},
		{"zero instances", `{"instances":0,"expires_at":"` + at(time.Hour) + `"}`, 0, false,
			"a pin of 0 is not a pin"},
		{"negative instances", `{"instances":-5,"expires_at":"` + at(time.Hour) + `"}`, 0, false,
			"openapi declares no minimum, so this floor is the only guard against a negative price"},
		{"no expires_at", `{"instances":9,"reason":"x"}`, 0, false,
			"unset must not mean forever — D22 makes the pin temporary"},
		{"unparseable expires_at", `{"instances":9,"expires_at":"not-a-date"}`, 0, false,
			"an unreadable expiry is not a live pin"},
		{"expired an hour ago", `{"instances":9,"expires_at":"` + at(-time.Hour) + `"}`, 0, false,
			"past its window"},
		{"exactly at expiry", `{"instances":9,"expires_at":"` + at(0) + `"}`, 0, false,
			"the window is half-open: at the boundary the pin is over"},
		{"one nanosecond before expiry", `{"instances":9,"expires_at":"` + at(time.Nanosecond) + `"}`, 9, true,
			"still inside the window"},
		{"live", `{"instances":9,"reason":"load","expires_at":"` + at(time.Hour) + `"}`, 9, true,
			"the ordinary case"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, live := overrideInstances([]byte(tc.raw), now)
			if live != tc.wantLive {
				t.Fatalf("live=%v, want %v — %s", live, tc.wantLive, tc.why)
			}
			if n != tc.wantN {
				t.Fatalf("instances=%d, want %d", n, tc.wantN)
			}
		})
	}
}

// desiredDoc is a RENDERER, not a filter: it embeds whatever pin it is handed
// and consults no expiry. That is deliberate — the liveness decision belongs to
// the one caller that also prices the pin — and it is the reason
// `TestTheDesiredDocNeverCarriesADeadPin` (identity, integration) has to drive
// UpdateService rather than this function.
//
// An earlier version of this test applied `overrideInstances` to the input
// itself and then asserted the doc came out unpinned. It passed for the same
// reason a mirror is flat: it had COPIED the production guard into the test
// body, so deleting that guard from services.go left it green. What it actually
// pinned was that the two functions COMPOSE — worth keeping, but not what its
// name claimed, and the class it was named for was uncovered while it reported
// covered.
func TestDesiredDocEmbedsThePinItIsHanded(t *testing.T) {
	for _, tc := range []struct {
		name     string
		override []byte
		wantPin  bool
	}{
		{"a pin is rendered verbatim", []byte(`{"instances":9,"reason":"x"}`), true},
		{"an expired pin is rendered too — this function does not judge",
			[]byte(`{"instances":9,"expires_at":"2000-01-01T00:00:00Z"}`), true},
		{"nil is the only thing that omits it", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := desiredDoc("web", "app", "env-x", []byte(`{"size":"standard-1"}`), nil, tc.override, false)
			var d map[string]any
			if err := json.Unmarshal(doc, &d); err != nil {
				t.Fatal(err)
			}
			if _, present := d["override"]; present != tc.wantPin {
				t.Fatalf("override in desired = %v, want %v: %s", present, tc.wantPin, doc)
			}
		})
	}
}

// repriceSpan's IsBilling guard carries US-3.6's invariant through this new code
// path: metering opens at `ready`, never before. Without it a PATCH on a
// still-provisioning service emits an OPEN span, and the rollup accrues an open
// span to the cutoff — billing a service that never ran.
func TestIsBillingGatesTheStatusesThatHaveAnOpenSpan(t *testing.T) {
	for status, want := range map[string]bool{
		"ready": true, "degraded": true,
		"provisioning": false, "failed": false, "suspended": false, "deleting": false, "": false,
	} {
		if got := metering.IsBilling(status); got != want {
			t.Fatalf("IsBilling(%q) = %v, want %v — a rate change is only meaningful while a span is open, and emitting one otherwise bills a service that never ran",
				status, got, want)
		}
	}
}

// The OTHER copy of the agent's legal-edge set.
//
// services/cell-agent has a separate go.mod and no go.work, so it cannot import
// this table; its TestEveryTerminalStatusIsALegalEdgeFromProvisioning duplicates
// {ready, failed, deleting}. Duplicated knowledge drifts silently unless both
// copies are pinned, and the first live consequence was
// `"Waiting for user action": "degraded"` — a status the agent could emit that
// this machine rejects forever.
func TestTheAgentsLegalEdgesMatchTheStatusMachine(t *testing.T) {
	want := map[string]bool{"ready": true, "failed": true, "deleting": true}
	got := map[string]bool{}
	for _, s := range transitions["provisioning"] {
		got[s] = true
	}
	if len(got) != len(want) {
		t.Fatalf("provisioning transitions to %v; the cell-agent's copy says %v. Update "+
			"services/cell-agent/internal/render's legalFromProvisioning in the same change, "+
			"or the agent will emit a status this machine rejects forever.", got, want)
	}
	for s := range want {
		if !got[s] {
			t.Fatalf("provisioning can no longer transition to %q, but the cell-agent's copy "+
				"still lists it", s)
		}
	}
}
