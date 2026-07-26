package render

import (
	"context"
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
