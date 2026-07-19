// Package contract holds the schema round-trip test (Q3): every canonical
// response shape in 19-canon/fixtures.json must decode into the oapi-codegen
// types generated FROM openapi.yaml — with unknown fields REFUSED and every
// required field populated. That makes the generated types (the contract) and
// the canon world (the demo data every mock/story/test serves) prove each
// other: a fixture field the schema doesn't declare fails here; a schema that
// drops a field the fixtures rely on fails here; a required field missing from
// a fixture fails here. Type drift fails the build, not the user.
//
// Scope note: this proves canon ↔ generated-types. Handler ↔ schema conformance
// is separate and comes from oapi-codegen's strict server (handlers return
// gen.*Response types), enforced at compile time — not asserted here.
package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
)

func fixtures(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	_, self, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(self), "..", "..", "..", "..", "docs", "product", "19-canon", "fixtures.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("canon fixtures: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// stripAnnotations removes canon's "$"-prefixed annotation keys (e.g. "$note",
// "$representative") recursively — they document the fixtures for humans and are
// never part of the wire contract (mocks strip them). What remains is the exact
// shape a handler returns.
func stripAnnotations(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if strings.HasPrefix(k, "$") {
				continue
			}
			out[k] = stripAnnotations(val)
		}
		return out
	case []any:
		for i := range t {
			t[i] = stripAnnotations(t[i])
		}
		return t
	default:
		return v
	}
}

func clean(t *testing.T, raw json.RawMessage) json.RawMessage {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(stripAnnotations(v))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// checkOne strict-decodes one object into T (refusing undeclared fields) and
// asserts every REQUIRED field's key is PRESENT. Strict decode catches EXTRA
// fields; requiredPresent catches MISSING ones — by key presence in the raw
// JSON, not zero-ness of the decoded value, so a required field that is
// legitimately zero (e.g. a quota's `used: 0`) is not a false positive.
func checkOne[T any](t *testing.T, raw json.RawMessage, label string) {
	t.Helper()
	cleaned := clean(t, raw)
	dec := json.NewDecoder(bytes.NewReader(cleaned))
	dec.DisallowUnknownFields()
	var v T
	if err := dec.Decode(&v); err != nil {
		t.Errorf("%s does not conform to the generated contract type: %v", label, err)
		return
	}
	var present map[string]json.RawMessage
	if err := json.Unmarshal(cleaned, &present); err != nil {
		t.Errorf("%s: not a JSON object: %v", label, err)
		return
	}
	requiredPresent(t, reflect.TypeOf(v), present, label)
}

func checkArray[T any](t *testing.T, raw json.RawMessage, label string) {
	t.Helper()
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		t.Errorf("%s: not a JSON array: %v", label, err)
		return
	}
	if len(elems) == 0 {
		t.Errorf("%s: empty — nothing to check", label)
	}
	for i, e := range elems {
		checkOne[T](t, e, fmt.Sprintf("%s[%d]", label, i))
	}
}

// requiredPresent asserts every field REQUIRED by the contract — a non-pointer
// field WITHOUT `omitempty` — has its key present in the raw fixture object.
// Optional fields (pointer or omitempty) are skipped. Presence (not non-zero)
// is the right test: a required field may legitimately be zero (`used: 0`), but
// it must be PRESENT — a dropped or renamed required field is the drift to
// catch.
func requiredPresent(t *testing.T, rt reflect.Type, present map[string]json.RawMessage, label string) {
	t.Helper()
	if rt.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if strings.Contains(tag, "omitempty") || f.Type.Kind() == reflect.Ptr {
			continue // optional by contract
		}
		name, _, _ := strings.Cut(tag, ",")
		if _, ok := present[name]; !ok {
			t.Errorf("%s: required field %q is absent — dropped from the fixture or renamed in the schema", label, name)
		}
	}
}

// TestCanonConformsToGeneratedTypes strict-decodes EVERY canon response section
// into its openapi-generated type. A registry drives it, and a completeness
// check (below) fails if any fixtures section lacks a contract check — so
// coverage can't silently rot as canon grows (derive, don't hand-maintain).
func TestCanonConformsToGeneratedTypes(t *testing.T) {
	doc := fixtures(t)
	reg := registry()
	for name, check := range reg {
		raw := sectionByName(t, doc, name)
		if raw == nil {
			t.Errorf("registry names %q but no such fixtures section exists", name)
			continue
		}
		check(t, raw)
	}
	t.Logf("contract-checked %d canon response sections against the generated types", len(reg))
}

// TestEveryFixtureSectionHasAContractCheck is the tripwire: a new response
// section added to canon without a contract check fails here.
func TestEveryFixtureSectionHasAContractCheck(t *testing.T) {
	doc := fixtures(t)
	reg := registry()
	for key := range doc {
		if strings.HasPrefix(key, "$") {
			continue // annotation, not a section
		}
		name, _, _ := strings.Cut(key, " (")
		if _, ok := reg[name]; !ok {
			t.Errorf("fixtures section %q has no contract check — add it to registry()", name)
		}
	}
}

// registry maps each fixtures section name to its contract check. Every
// response-shaped section MUST appear here (enforced by the tripwire above).
func registry() map[string]func(*testing.T, json.RawMessage) {
	return map[string]func(*testing.T, json.RawMessage){
		"orgs":                   func(t *testing.T, r json.RawMessage) { checkArray[gen.Org](t, r, "orgs") },
		"members":                func(t *testing.T, r json.RawMessage) { checkArray[gen.Member](t, r, "members") },
		"projects":               func(t *testing.T, r json.RawMessage) { checkArray[gen.Project](t, r, "projects") },
		"environments":           func(t *testing.T, r json.RawMessage) { checkArray[gen.Environment](t, r, "environments") },
		"services":               func(t *testing.T, r json.RawMessage) { checkArray[gen.Service](t, r, "services") },
		"bindings":               func(t *testing.T, r json.RawMessage) { checkArray[gen.Binding](t, r, "bindings") },
		"deployments":            func(t *testing.T, r json.RawMessage) { checkArray[gen.Deployment](t, r, "deployments") },
		"events":                 func(t *testing.T, r json.RawMessage) { checkArray[gen.Event](t, r, "events") },
		"trace":                  func(t *testing.T, r json.RawMessage) { checkOne[gen.Trace](t, r, "trace") },
		"alert_rules":            func(t *testing.T, r json.RawMessage) { checkArray[gen.AlertRule](t, r, "alert_rules") },
		"insights_and_proposals": checkInsightsAndProposals,
		"templates":              func(t *testing.T, r json.RawMessage) { checkArray[gen.Template](t, r, "templates") },
		"dashboards":             func(t *testing.T, r json.RawMessage) { checkArray[gen.Dashboard](t, r, "dashboards") },
		"billing_overview":       func(t *testing.T, r json.RawMessage) { checkOne[gen.BillingOverview](t, r, "billing_overview") },
		"quotas":                 func(t *testing.T, r json.RawMessage) { checkArray[gen.Quota](t, r, "quotas") },
		"cells":                  func(t *testing.T, r json.RawMessage) { checkArray[gen.Cell](t, r, "cells") },
		"estimate_example":       func(t *testing.T, r json.RawMessage) { checkOne[gen.Estimate](t, r, "estimate_example") },
		"borealis_lifecycle":     checkBorealisLifecycle,
	}
}

// insights_and_proposals is an array of {insight, proposal} pairs.
func checkInsightsAndProposals(t *testing.T, raw json.RawMessage) {
	var pairs []struct {
		Insight  json.RawMessage `json:"insight"`
		Proposal json.RawMessage `json:"proposal"`
	}
	if err := json.Unmarshal(raw, &pairs); err != nil {
		t.Errorf("insights_and_proposals: %v", err)
		return
	}
	for i, p := range pairs {
		checkOne[gen.Insight](t, p.Insight, fmt.Sprintf("insights_and_proposals[%d].insight", i))
		checkOne[gen.Proposal](t, p.Proposal, fmt.Sprintf("insights_and_proposals[%d].proposal", i))
	}
}

// borealis_lifecycle is an object whose values are Subscription states.
func checkBorealisLifecycle(t *testing.T, raw json.RawMessage) {
	var states map[string]json.RawMessage
	if err := json.Unmarshal(raw, &states); err != nil {
		t.Errorf("borealis_lifecycle: %v", err)
		return
	}
	for state, v := range states {
		checkOne[gen.Subscription](t, v, "borealis_lifecycle."+state)
	}
}

func sectionByName(t *testing.T, doc map[string]json.RawMessage, name string) json.RawMessage {
	t.Helper()
	var match json.RawMessage
	found := 0
	for key, v := range doc {
		if key == name || strings.HasPrefix(key, name+" (") {
			match = v
			found++
		}
	}
	if found > 1 {
		t.Fatalf("section %q: %d matches, want 1", name, found)
	}
	return match
}
