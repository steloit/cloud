package reconcile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/provisioning"
	"gopkg.in/yaml.v3"
)

// specEnum reads the request enum for POST /reconcile/{cell}/status out of
// openapi.yaml itself.
//
// THE SPEC IS THE AUTHORITY, and a test that reads only the generated artifact
// asserts a derived copy. It also cannot see a value added to the contract that
// is not an ADR-024 status: building the candidate set from
// StatusVocabulary() means `paused` in the enum is never probed, and that was
// measured green. Reading the enum makes the sweep bidirectional — every value
// the contract advertises is checked, whatever it is.
//
// Precedent for reading the spec in a test: TestRouteTableMatchesOpenAPI
// (platform/idempotency) and assistant_gate_coverage_test.
func specEnum(t *testing.T) []string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile),
		"..", "..", "..", "..", "docs", "product", "08-api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	var doc struct {
		Paths map[string]struct {
			Post struct {
				RequestBody struct {
					Content map[string]struct {
						Schema struct {
							Properties struct {
								Status struct {
									Enum []string `yaml:"enum"`
								} `yaml:"status"`
							} `yaml:"properties"`
						} `yaml:"schema"`
					} `yaml:"content"`
				} `yaml:"requestBody"`
			} `yaml:"post"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	got := doc.Paths["/reconcile/{cell}/status"].Post.RequestBody.
		Content["application/json"].Schema.Properties.Status.Enum
	if len(got) == 0 {
		t.Fatal("no request enum found at paths./reconcile/{cell}/status.post — " +
			"this test would prove nothing, and a silent zero is exactly how it would rot")
	}
	return got
}

// THE CONTRACT AND THE ENFORCEMENT ARE THE SAME SET, IN BOTH DIRECTIONS.
//
// US-3.3h made the API refuse `suspended` and `deleting` from a cell — they are
// lifecycle states the control plane sets, and accepting one let a single POST
// with the reconciler token brick a service permanently. It did NOT narrow the
// request enum, so openapi.yaml went on advertising both. An agent generated
// from the contract would have believed both were valid.
//
// Swept from BOTH sides: every value the spec advertises must be reportable, and
// every reportable value must be advertised. One direction alone is what let a
// non-ADR-024 addition slip through.
func TestTheRequestEnumAdmitsExactlyWhatACellMayReport(t *testing.T) {
	advertised := map[string]bool{}
	for _, s := range specEnum(t) {
		advertised[s] = true
	}

	// `gone` is not a workload observation — it says the workload is ABSENT —
	// but it is the teardown signal and must stay on the wire. "" is the
	// observation-only heartbeat, carried by OMITTING the field, so it is
	// deliberately not an enum member.
	reportable := func(s string) bool { return provisioning.ReportableByCell(s) || s == "gone" }

	// Direction 1: nothing is advertised that the API refuses.
	for s := range advertised {
		if !reportable(s) {
			t.Errorf("the contract advertises %q, which the API refuses with 422 — a client "+
				"generated from it would send something that can never be accepted", s)
		}
	}
	// Direction 2: nothing reportable is left unadvertised.
	for _, s := range append(provisioning.StatusVocabulary(), "gone") {
		if reportable(s) && !advertised[s] {
			t.Errorf("a cell may report %q and the contract does not advertise it — a generated "+
				"client cannot send a value the API accepts", s)
		}
	}
	// And the generated artifact must agree with the spec it came from, so a
	// hand-edit of types.gen.go that CI's gen gate would overwrite is still RED here.
	for _, s := range append(provisioning.StatusVocabulary(), "gone", "", "not-a-status") {
		if got, want := gen.PostReconcileStatusJSONBodyStatus(s).Valid(), advertised[s]; got != want {
			t.Errorf("generated Valid(%q) = %v but the spec advertises it = %v — the generated "+
				"types have drifted from the contract they are derived from", s, got, want)
		}
	}
	// The sweep must still contain the two values it exists for.
	for _, s := range []string{"suspended", "deleting"} {
		if provisioning.ReportableByCell(s) {
			t.Fatalf("%q became reportable — this test's premise is gone", s)
		}
	}
}

// The WIRE gate stays deliberately WIDER than the contract enum, and that is not
// drift: `statusVocab` mirrors the customer-facing ServiceStatus so that a cell
// sending `suspended` gets the SPECIFIC 422 rather than the generic "not in the
// ADR-024 vocabulary", which is true of neither. The message is the entire
// justification for the asymmetry, so it is asserted on the route
// (TestHTTPACellCannotSuspendOrDeleteAService), not just here.
func TestTheWireGateAcceptsEverythingTheContractAdvertises(t *testing.T) {
	for _, s := range specEnum(t) {
		if !statusVocab[s] {
			t.Errorf("the contract advertises %q and the wire gate rejects it as unknown", s)
		}
	}
	if !statusVocab[""] {
		t.Error(`the wire gate rejects "" — an omitted status is the observation-only heartbeat`)
	}
	// The intended asymmetry, stated so removing it is a deliberate act.
	for _, s := range []string{"suspended", "deleting"} {
		if !statusVocab[s] {
			t.Errorf("%q no longer reaches the specific refusal — it would be reported as "+
				"'not in the ADR-024 vocabulary', which is true of neither", s)
		}
	}
}
