package render

import (
	"bytes"
	"context"
	"strings"
	"testing"
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
	for _, ns := range []string{"acme--prod/Cluster/svc-db01", "acme--prod/ScheduledBackup/svc-db01-nightly"} {
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
	firstManifests := append([][]byte(nil), a.applied["acme--prod"]...)
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
	second := a.applied["acme--prod"]
	if len(second) != len(firstManifests) {
		t.Fatalf("the retry rendered %d objects, the first attempt %d", len(second), len(firstManifests))
	}
	for i := range second {
		if !bytes.Equal(second[i], firstManifests[i]) {
			t.Fatalf("manifest %d differs between the attempt and the retry — the apply is not idempotent:\n--- first ---\n%s\n--- retry ---\n%s",
				i, firstManifests[i], second[i])
		}
	}
	// And nothing was orphaned under a different name.
	for k := range a.live {
		if !strings.Contains(k, "svc-db01") {
			t.Fatalf("retry left an object under an unexpected name: %s", k)
		}
	}
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
	if len(a.live) != 0 {
		t.Fatalf("deleting a FAILED service stranded %d object(s): %v", len(a.live), a.live)
	}
}
