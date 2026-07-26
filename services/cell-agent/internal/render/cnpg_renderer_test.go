package render

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/steloit/cloud/services/cell-agent/internal/agent"
	"github.com/steloit/cloud/services/cell-agent/internal/driver"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/cnpg"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeApplier records what was applied and returns a scripted cluster phase, so
// the whole render→apply→observe path is provable without a cluster.
type fakeApplier struct {
	mu         sync.Mutex
	applied    map[string][][]byte // namespace/name → objects
	deleted    []string
	phase      string
	applyErr   error
	observeErr error
	applies    int
}

func newFakeApplier(phase string) *fakeApplier {
	return &fakeApplier{applied: map[string][][]byte{}, phase: phase}
}

func (f *fakeApplier) Apply(_ context.Context, ns string, m [][]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.applyErr != nil {
		return f.applyErr
	}
	f.applies++
	f.applied[ns] = m
	return nil
}
func (f *fakeApplier) Observe(_ context.Context, _, _ string) (string, error) {
	if f.observeErr != nil {
		return "", f.observeErr
	}
	return f.phase, nil
}
func (f *fakeApplier) Delete(_ context.Context, ns, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, ns+"/"+name)
	return nil
}

func svc(id, status string) agent.DesiredService {
	return agent.DesiredService{
		ID: id, CellID: "cell-0", Product: "postgres", Status: status, Generation: 1,
		Desired: map[string]any{
			"product": "postgres", "shape": map[string]any{"size": "dev"},
			"placement": map[string]any{
				"namespace": "acme--prod", "gsa": "sa@steloit-dev.iam.gserviceaccount.com",
				"wal_bucket": "steloit-dev-wal-customer",
			},
		},
	}
}

func newRenderer(applier *fakeApplier) *CNPGRenderer {
	return NewCNPGRenderer(cnpg.New(), applier, "cell-0", quiet())
}

func TestCNPGRendererAppliesRenderedManifests(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	status, err := newRenderer(a).Converge(context.Background(), svc("svc_db01", "provisioning"))
	if err != nil {
		t.Fatal(err)
	}
	if status != "ready" {
		t.Fatalf("healthy cluster → want ready, got %q", status)
	}
	objs := a.applied["acme--prod"]
	if len(objs) != 2 { // Cluster + ScheduledBackup (T3.4)
		t.Fatalf("expected the CNPG Cluster + base backup applied, got %d objects", len(objs))
	}
	joined := string(objs[0]) + string(objs[1])
	if !strings.Contains(joined, "kind: Cluster") || !strings.Contains(joined, "kind: ScheduledBackup") {
		t.Fatal("applied objects are not the CNPG cluster + base backup")
	}
	// D8-adjacent: the namespace came from placement, not a guess.
	if !strings.Contains(string(objs[0]), "namespace: acme--prod") {
		t.Fatal("rendered manifest not placed in the resolved namespace")
	}
}

func TestConvergeObservesClusterStatus(t *testing.T) {
	cases := map[string]string{
		"Cluster in healthy state": "ready",
		"":                         "provisioning",
		"Setting up primary":       "provisioning",
		"Failed to create primary": "failed",
	}
	for phase, want := range cases {
		a := newFakeApplier(phase)
		got, err := newRenderer(a).Converge(context.Background(), svc("svc_db01", "provisioning"))
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("phase %q → want %q, got %q", phase, want, got)
		}
	}
}

func TestConvergeReturnsProvisioningUntilReady(t *testing.T) {
	a := newFakeApplier("") // not created yet
	got, _ := newRenderer(a).Converge(context.Background(), svc("svc_db01", "provisioning"))
	if got != "provisioning" {
		t.Fatalf("a cluster with no status yet must read provisioning, got %q", got)
	}
	if a.applies != 1 {
		t.Fatal("converge must apply the manifests even while provisioning")
	}
}

func TestDeletingConvergesToGoneAndDeletes(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	got, err := newRenderer(a).Converge(context.Background(), svc("svc_db01", "deleting"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "gone" {
		t.Fatalf("a deleting service must converge to gone, got %q", got)
	}
	if len(a.deleted) != 1 || a.deleted[0] != "acme--prod/svc_db01" {
		t.Fatalf("teardown not applied to the cluster: %v", a.deleted)
	}
	if a.applies != 0 {
		t.Fatal("a deleting service must not apply create manifests")
	}
}

func TestNamespaceDerivedFromDesired(t *testing.T) {
	// No placement → error, never a guessed namespace.
	s := svc("svc_db01", "provisioning")
	delete(s.Desired, "placement")
	if _, err := newRenderer(newFakeApplier("")).Converge(context.Background(), s); err == nil {
		t.Fatal("a service with no resolved namespace must error, not guess")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	for range 3 {
		if _, err := r.Converge(context.Background(), svc("svc_db01", "provisioning")); err != nil {
			t.Fatal(err)
		}
	}
	// The renderer applies each converge (SSA is idempotent server-side); the
	// applied object set is byte-identical each time.
	if a.applies != 3 {
		t.Fatalf("expected 3 idempotent applies, got %d", a.applies)
	}
	if !bytesEqual(a.applied["acme--prod"], mustRender(t)) {
		t.Fatal("repeated converge produced different manifests — not idempotent")
	}
}

func mustRender(t *testing.T) [][]byte {
	t.Helper()
	m, err := cnpg.New().Render(driver.Spec{
		Name: "svc_db01", Namespace: "acme--prod", Product: "postgres",
		Shape: map[string]any{"size": "dev"}, Instances: 1, Cell: "cell-0",
		GSAEmail: "sa@steloit-dev.iam.gserviceaccount.com", WALBucket: "steloit-dev-wal-customer",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := make([][]byte, len(m))
	for i, o := range m {
		out[i] = o.YAML
	}
	return out
}

func bytesEqual(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if string(a[i]) != string(b[i]) {
			return false
		}
	}
	return true
}

func TestApplyErrorSurfaces(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	a.applyErr = context.DeadlineExceeded
	if _, err := newRenderer(a).Converge(context.Background(), svc("svc_db01", "provisioning")); err == nil {
		t.Fatal("an apply failure must surface (the reconciler leaves the row outstanding)")
	}
}
