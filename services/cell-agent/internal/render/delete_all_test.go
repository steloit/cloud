package render

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/steloit/cloud/services/cell-agent/internal/driver/tenancy"
)

func TestDeleteRemovesEveryRenderedObject(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	// create both objects first
	if _, err := r.Converge(context.Background(), svc("svc_db01", "provisioning")); err != nil {
		t.Fatal(err)
	}
	// now tear down
	if _, err := r.Converge(context.Background(), svc("svc_db01", "deleting")); err != nil {
		t.Fatal(err)
	}
	// the ScheduledBackup must not survive the cluster (it would keep firing
	// against a cluster that no longer exists)
	if len(a.deleted) < 2 {
		t.Fatalf("teardown deleted %d object(s); the driver rendered 2 (Cluster + ScheduledBackup): %v", len(a.deleted), a.deleted)
	}
	for _, ns := range []string{"env-9f3c1a2b/Cluster/svc-db01", "env-9f3c1a2b/ScheduledBackup/svc-db01-nightly"} {
		var found bool
		for _, d := range a.deleted {
			if d == ns {
				found = true
			}
		}
		if !found {
			t.Fatalf("teardown left %s behind: deleted=%v", ns, a.deleted)
		}
	}
}

// US-3.6 guarantee 2 — "a failure never strands state", the RETRY half.
//
// A provisioning attempt that fails partway (the apply landed, then the cluster
// went to a failed phase) must converge to exactly ONE cluster when it is
// retried — not a second one alongside the first. This is what server-side
// apply buys: the retry addresses the same names and reconciles them, so a
// partial attempt is repaired rather than duplicated.
func TestFailedProvisioningRetryLeavesExactlyOneCluster(t *testing.T) {
	a := newFakeApplier("Cluster in failure state")
	r := newRenderer(a)
	ctx := context.Background()

	status, err := r.Converge(ctx, svc("svc_db01", "provisioning"))
	if err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("a failed CNPG phase must report failed, got %q", status)
	}
	liveAfterFailure := len(a.live)
	firstManifests := append([][]byte(nil), a.applied["env-9f3c1a2b"]...)
	if liveAfterFailure == 0 {
		// Guard against a vacuous pass: if the failed attempt applied nothing,
		// the "no new objects" check below compares 0 to 0 and proves nothing.
		t.Fatal("the failed attempt applied no objects — this test would prove nothing")
	}

	// The retry: same desired state, cluster now healthy.
	a.phase = "Cluster in healthy state"
	status, err = r.Converge(ctx, svc("svc_db01", "provisioning"))
	if err != nil {
		t.Fatal(err)
	}
	if status != "ready" {
		t.Fatalf("the retry must reach ready, got %q", status)
	}
	if len(a.live) != liveAfterFailure {
		t.Fatalf("a retry after failure created NEW objects (%d → %d): %v — a partial attempt must be repaired, not duplicated",
			liveAfterFailure, len(a.live), a.live)
	}
	// A stable object COUNT alone would also be satisfied by "the retry applied
	// nothing at all", so pin that the retry really re-applied, and that what it
	// sent was byte-identical to the first attempt — that is what makes
	// server-side apply repair a partial attempt instead of duplicating it.
	if a.applies != 2 {
		t.Fatalf("the retry applied %d times, want 2 — a retry that skips the apply repairs nothing", a.applies)
	}
	second := a.applied["env-9f3c1a2b"]
	if len(second) != len(firstManifests) {
		t.Fatalf("the retry rendered %d objects, the first attempt %d", len(second), len(firstManifests))
	}
	for i := range second {
		if !bytes.Equal(second[i], firstManifests[i]) {
			t.Fatalf("manifest %d differs between the attempt and the retry — the apply is not idempotent:\n--- first ---\n%s\n--- retry ---\n%s",
				i, firstManifests[i], second[i])
		}
	}
	// And nothing was orphaned under a different name. Environment objects (the
	// namespace and its D7 policies) are expected and excluded — they are not
	// this service's, and every converge reapplies them by design.
	envObjs := envObjectKeys(t)
	for k := range a.live {
		if envObjs[k] {
			continue
		}
		if !strings.Contains(k, "svc-db01") {
			t.Fatalf("retry left an object under an unexpected name: %s", k)
		}
	}
}

// testNamespace is the ADR-0012 shape, sanitize(env_id) — e.g. env_9f3c1a2b
// becomes env-9f3c1a2b. The fixtures used the pre-ADR-0012 `proj--env` form
// (`acme--prod`), which the platform can no longer produce.
const testNamespace = "env-9f3c1a2b"

// envObjectKeys is the set of objects that belong to the ENVIRONMENT rather than
// to any service — today the namespace, and whatever US-3.3c adds beside it.
// DERIVED from tenancy.Render, never retyped: a hardcoded list silently stops
// covering an object added there, which is how a test starts asserting less
// than it claims.
func envObjectKeys(t *testing.T) map[string]bool {
	t.Helper()
	ms, err := tenancy.Render(tenancy.Spec{Namespace: testNamespace, Cell: "cell-0"})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, m := range ms {
		out[testNamespace+"/"+m.Name] = true
	}
	return out
}

// The DELETE half: a service abandoned in `failed` and then deleted must leave
// NOTHING behind. A failed attempt still applied objects, so teardown has real
// work to do — "it failed, so there is nothing to clean up" is the assumption
// that strands state.
func TestDeletingAFailedServiceLeavesNothingBehind(t *testing.T) {
	a := newFakeApplier("Cluster in failure state")
	r := newRenderer(a)
	ctx := context.Background()

	if _, err := r.Converge(ctx, svc("svc_db01", "provisioning")); err != nil {
		t.Fatal(err)
	}
	if len(a.live) == 0 {
		t.Fatal("the failed attempt applied nothing — this test would prove nothing")
	}
	status, err := r.Converge(ctx, svc("svc_db01", "deleting"))
	if err != nil {
		t.Fatal(err)
	}
	if status != "gone" {
		t.Fatalf("teardown must report gone, got %q", status)
	}
	// Nothing of the SERVICE may remain. The environment's namespace and its D7
	// policies deliberately DO remain — they belong to the environment, not to
	// this service, and other services live in the same namespace. Deleting one
	// service must not dismantle the tenant boundary around its siblings.
	//
	// The env-scoped set is DERIVED from tenancy.Render, not retyped: a list here
	// would silently stop covering a policy added there.
	envObjs := envObjectKeys(t)
	for k := range a.live {
		if !envObjs[k] {
			t.Fatalf("deleting a FAILED service stranded a SERVICE object: %s (live: %v)", k, a.live)
		}
	}
	// And the boundary must still be standing — asserted positively, because
	// "no service object remains" is also satisfied by having deleted the
	// namespace along with everything in it.
	for k := range envObjs {
		if !a.live[k] {
			t.Fatalf("service teardown removed the ENVIRONMENT object %s — the tenant boundary "+
				"belongs to the env and its siblings still need it", k)
		}
	}
}

// The TEARDOWN path must validate the namespace too.
//
// Converge's deleting branch returns before tenancy.Render is ever reached, so a
// check living inside Render guards the create path alone — and teardown is the
// path that fmt.Sprintf's the value straight into a DELETE URL. Probed before
// the fix: a namespace of "../../../api/v1/namespaces/kube-system" was refused on
// create and accepted on teardown, reporting gone after issuing deletes against
// paths that walk out of the namespace entirely.
func TestTeardownRefusesANamespaceItWouldNotHaveCreated(t *testing.T) {
	for _, bad := range []string{
		"../../../api/v1/namespaces/kube-system",
		"env-x/../kube-system",
		"kube-system",
		"env-UPPER",
		"env-x\nmetadata: injected",
	} {
		a := newFakeApplier("Cluster in healthy state")
		s := svc("svc_db01", "deleting")
		s.Desired["namespace"] = bad

		if _, err := newRenderer(a).Converge(context.Background(), s); err == nil {
			t.Errorf("teardown accepted namespace %q", bad)
		}
		if len(a.deleted) != 0 {
			t.Errorf("teardown issued deletes for %q: %v", bad, a.deleted)
		}
	}

	// And the legitimate teardown still works, or the above would be satisfied by
	// a Converge that refuses every deletion.
	a := newFakeApplier("Cluster in healthy state")
	if _, err := newRenderer(a).Converge(context.Background(), svc("svc_db01", "provisioning")); err != nil {
		t.Fatal(err)
	}
	s := svc("svc_db01", "deleting")
	if status, err := newRenderer(a).Converge(context.Background(), s); err != nil {
		t.Fatalf("a legitimate teardown was refused: %v", err)
	} else if status != "gone" {
		t.Fatalf("teardown reported %q, want gone", status)
	}
}
