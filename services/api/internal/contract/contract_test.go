// Package contract holds the schema round-trip test (Q3): every canonical
// response shape in 19-canon/fixtures.json must decode into the oapi-codegen
// types generated FROM openapi.yaml — with unknown fields REFUSED. That makes
// the generated types (the contract) and the canon world (the demo data every
// mock/story/test serves) prove each other: a fixture field the schema doesn't
// declare fails here, and a schema that drops a field the fixtures rely on
// fails here too. Type drift fails the build, not the user.
package contract

import (
	"bytes"
	"encoding/json"
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
// shape a handler returns, which is what the generated types must accept.
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

// clean re-encodes raw with annotation keys stripped, so strict decoding sees
// only contract fields.
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

// section returns the ONE top-level fixtures section named `name` (keys are
// annotated: "orgs (Org)"), matched exactly or as "name (".
func section(t *testing.T, doc map[string]json.RawMessage, name string) json.RawMessage {
	t.Helper()
	var match json.RawMessage
	found := 0
	for key, v := range doc {
		if key == name || strings.HasPrefix(key, name+" (") {
			match = v
			found++
		}
	}
	if found != 1 {
		t.Fatalf("section %q: found %d matches, want 1", name, found)
	}
	return match
}

// strictDecodeArray decodes a JSON array into []T with unknown fields refused,
// so a fixture field absent from the generated (openapi) type fails the test.
func strictDecodeArray[T any](t *testing.T, raw json.RawMessage, label string) []T {
	t.Helper()
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		t.Fatalf("%s: not a JSON array: %v", label, err)
	}
	out := make([]T, 0, len(elems))
	for i, e := range elems {
		dec := json.NewDecoder(bytes.NewReader(clean(t, e)))
		dec.DisallowUnknownFields()
		var v T
		if err := dec.Decode(&v); err != nil {
			t.Errorf("%s[%d] does not conform to the generated contract type: %v", label, i, err)
			continue
		}
		out = append(out, v)
	}
	return out
}

func strictDecodeOne[T any](t *testing.T, raw json.RawMessage, label string) T {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(clean(t, raw)))
	dec.DisallowUnknownFields()
	var v T
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("%s does not conform to the generated contract type: %v", label, err)
	}
	return v
}

// TestCanonConformsToGeneratedTypes: each canon response section round-trips
// through the openapi-generated type with unknown fields refused. This is the
// handler↔schema contract — the handlers return these very types, so if canon
// (what mocks/docs/tests serve) fits them, the wire shapes agree.
func TestCanonConformsToGeneratedTypes(t *testing.T) {
	doc := fixtures(t)

	orgs := strictDecodeArray[gen.Org](t, section(t, doc, "orgs"), "orgs")
	if len(orgs) == 0 {
		t.Fatal("no orgs decoded")
	}
	projects := strictDecodeArray[gen.Project](t, section(t, doc, "projects"), "projects")
	envs := strictDecodeArray[gen.Environment](t, section(t, doc, "environments"), "environments")
	services := strictDecodeArray[gen.Service](t, section(t, doc, "services"), "services")
	_ = strictDecodeOne[gen.BillingOverview](t, section(t, doc, "billing_overview"), "billing_overview")

	// Required (non-omitempty) fields must be populated — a contract type whose
	// required field is absent in canon would silently zero, so assert the
	// anchors are non-zero.
	for _, p := range projects {
		if p.Id == "" || p.Name == "" || p.OrgId == "" {
			t.Errorf("project missing a required field: %+v", p)
		}
	}
	for _, e := range envs {
		if e.Id == "" || e.Name == "" || e.ProjectId == "" || e.Region == "" {
			t.Errorf("environment missing a required field: %+v", e)
		}
	}
	for _, s := range services {
		if s.Id == "" || s.Name == "" || s.EnvId == "" {
			t.Errorf("service missing a required field: %+v", s)
		}
	}
	t.Logf("canon conforms: %d orgs, %d projects, %d envs, %d services round-tripped strictly",
		len(orgs), len(projects), len(envs), len(services))
}

// TestGeneratedTypesRoundTripLossless: re-marshalling a decoded fixture yields
// the same logical JSON (no field silently dropped by the contract type). A
// round-trip that loses data means the type is narrower than the wire shape.
func TestGeneratedTypesRoundTripLossless(t *testing.T) {
	doc := fixtures(t)
	// projects carry the cost the whole product is about — prove no loss there.
	var elems []json.RawMessage
	if err := json.Unmarshal(section(t, doc, "projects"), &elems); err != nil {
		t.Fatal(err)
	}
	for i, e := range elems {
		e := clean(t, e) // drop canon annotations; compare only contract fields
		var p gen.Project
		if err := json.Unmarshal(e, &p); err != nil {
			t.Fatalf("projects[%d]: %v", i, err)
		}
		reencoded, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var a, b map[string]any
		_ = json.Unmarshal(e, &a)
		_ = json.Unmarshal(reencoded, &b)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("projects[%d] lost data through the contract type:\n in: %s\nout: %s", i, e, reencoded)
		}
	}
}
