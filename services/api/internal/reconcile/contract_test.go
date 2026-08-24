package reconcile

import (
	"testing"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/provisioning"
)

// THE CONTRACT AND THE ENFORCEMENT ARE THE SAME SET.
//
// US-3.3h made the API refuse `suspended` and `deleting` from a cell — they are
// lifecycle states the control plane sets, and accepting one let a single POST
// with the reconciler token brick a service permanently. It did NOT narrow the
// request enum, so `openapi.yaml` went on advertising both. The contract is the
// authority generated clients are built from: an agent generated from it would
// have believed both were valid and got a 422 at runtime.
//
// This reads the GENERATED enum rather than a retyped list — probing
// `Valid()`, which oapi-codegen derives from the spec — so the assertion is
// against the contract as shipped, not against a third copy of it. It runs over
// the whole ADR-024 vocabulary plus the two non-status values, so a status added
// to the machine is swept automatically.
func TestTheRequestEnumAdmitsExactlyWhatACellMayReport(t *testing.T) {
	candidates := append(provisioning.StatusVocabulary(), "gone", "", "not-a-status")
	checked := 0
	for _, s := range candidates {
		inContract := gen.PostReconcileStatusJSONBodyStatus(s).Valid()
		// `gone` is not a workload observation (it says the workload is absent),
		// but it is the teardown signal and must stay on the wire. "" is the
		// observation-only heartbeat: it is carried by omitting the field, so it
		// is deliberately NOT an enum member.
		want := provisioning.ReportableByCell(s) || s == "gone"
		checked++
		if inContract != want {
			verb := "advertises"
			if want {
				verb = "omits"
			}
			t.Errorf("the request enum %s %q while provisioning.ReportableByCell says %v — "+
				"a client generated from the contract would send something the API refuses "+
				"with 422, or refuse to send something it accepts", verb, s, want)
		}
	}
	if checked < len(candidates) || checked == 0 {
		t.Fatalf("swept %d candidates of %d", checked, len(candidates))
	}
	// And the sweep must actually contain the two that caused this.
	var sawSuspended, sawDeleting bool
	for _, s := range candidates {
		sawSuspended = sawSuspended || s == "suspended"
		sawDeleting = sawDeleting || s == "deleting"
	}
	if !sawSuspended || !sawDeleting {
		t.Fatal("the sweep no longer covers `suspended`/`deleting` — the two values this test exists for")
	}
}

// The WIRE gate stays deliberately WIDER than the contract enum, and that is not
// drift: `statusVocab` mirrors the customer-facing ServiceStatus so that a cell
// sending `suspended` gets the specific 422 ("a lifecycle state the control
// plane sets, not something a cell can observe") instead of the generic "not in
// the ADR-024 vocabulary". Narrowing it would make the refusal less useful.
//
// What must hold is containment: nothing the contract advertises may be rejected
// by the wire gate.
func TestTheWireGateAcceptsEverythingTheContractAdvertises(t *testing.T) {
	for _, s := range append(provisioning.StatusVocabulary(), "gone", "") {
		if !gen.PostReconcileStatusJSONBodyStatus(s).Valid() && s != "" {
			continue // not advertised; the enum test above owns that
		}
		if !statusVocab[s] {
			t.Errorf("the contract advertises %q and the wire gate rejects it as unknown", s)
		}
	}
	// The intended asymmetry, stated so removing it is a deliberate act: the two
	// lifecycle states reach the handler and are refused with the specific
	// message (TestHTTPACellCannotSuspendOrDeleteAService drives that on the route).
	for _, s := range []string{"suspended", "deleting"} {
		if !statusVocab[s] {
			t.Errorf("%q no longer reaches the specific refusal — it would now be reported as "+
				"'not in the ADR-024 vocabulary', which is true of neither", s)
		}
		if gen.PostReconcileStatusJSONBodyStatus(s).Valid() {
			t.Errorf("%q is advertised in the request enum again", s)
		}
	}
}
