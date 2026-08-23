package render

import (
	"context"
	"encoding/json"
	"errors"
	"gopkg.in/yaml.v3"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/steloit/cloud/services/cell-agent/internal/agent"
	"github.com/steloit/cloud/services/cell-agent/internal/driver"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/cnpg"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/tenancy"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeApplier records what was applied and returns a scripted cluster phase, so
// the whole render→apply→observe path is provable without a cluster.
type fakeApplier struct {
	mu         sync.Mutex
	applied    map[string][][]byte // namespace/name → objects
	deleted    []string
	live       map[string]bool // ns/name → exists, like an API server
	phase      string
	applyErr   error
	observeErr error
	applies    int
}

func newFakeApplier(phase string) *fakeApplier {
	return &fakeApplier{applied: map[string][][]byte{}, phase: phase}
}

// Apply records objects BY NAME, exactly like an API server: whatever name the
// manifest carries is the only name Observe/Delete can find. The original fake
// ignored names, which is precisely why it could not see the renderer asking for
// `svc_x` while the driver had created `svc-x`.
func (f *fakeApplier) Apply(_ context.Context, ns string, m [][]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.applyErr != nil {
		return f.applyErr
	}
	f.applies++
	f.applied[ns] = m
	if f.live == nil {
		f.live = map[string]bool{}
	}
	for _, obj := range m {
		f.live[ns+"/"+nameOf(obj)] = true
	}
	return nil
}

// nameOf extracts metadata.name from a rendered manifest (good enough for the
// two-space-indented YAML the driver emits).
func nameOf(obj []byte) string {
	for _, line := range strings.Split(string(obj), "\n") {
		if strings.HasPrefix(line, "  name: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "  name: "))
		}
	}
	return ""
}

func (f *fakeApplier) Observe(_ context.Context, ns, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.observeErr != nil {
		return "", f.observeErr
	}
	// A name that was never applied does not exist — "" like a real 404.
	if !f.live[ns+"/"+name] {
		return "", nil
	}
	return f.phase, nil
}

// Delete records KIND/name — a real API server routes by kind, so a fake that
// ignores it cannot see a delete sent to the wrong resource path (exactly how a
// ScheduledBackup orphan hid in review round 2).
func (f *fakeApplier) Delete(_ context.Context, ns, kind, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, ns+"/"+kind+"/"+name)
	delete(f.live, ns+"/"+name)
	return nil
}

func svc(id, status string) agent.DesiredService {
	return agent.DesiredService{
		ID: id, CellID: "cell-0", Product: "postgres", Status: status, Generation: 1,
		Desired: map[string]any{
			"product": "postgres", "shape": map[string]any{"size": "dev"},
			"namespace": "env-9f3c1a2b",
		},
	}
}

func newRenderer(applier *fakeApplier) *CNPGRenderer {
	return NewCNPGRenderer(cnpg.New(), applier, "cell-0", "sa@steloit-dev.iam.gserviceaccount.com", "steloit-dev-wal-customer", quiet())
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
	objs := a.applied["env-9f3c1a2b"]
	// Asserted by KIND, not by count or position: a count breaks whenever either
	// renderer legitimately grows an object, and then gets "fixed" by bumping the
	// number, which asserts nothing.
	joined := ""
	for _, o := range objs {
		joined += string(o)
	}
	for _, kind := range []string{"kind: Cluster", "kind: ScheduledBackup"} {
		if !strings.Contains(joined, kind) {
			t.Fatalf("the applied set does not contain %q", kind)
		}
	}

	// ORDERING IS THE INVARIANT (US-3.3a). The Namespace must be applied before
	// anything namespaced, or the first converge into a new environment 404s.
	// Asserted on the real applied slice rather than on tenancy.Render's output,
	// because the ordering that matters is the one the APPLIER sees.
	nsAt, clusterAt := -1, -1
	for i, o := range objs {
		if strings.Contains(string(o), "kind: Namespace") && nsAt < 0 {
			nsAt = i
		}
		if strings.Contains(string(o), "kind: Cluster") && clusterAt < 0 {
			clusterAt = i
		}
	}
	if nsAt < 0 {
		t.Fatal("no Namespace was applied — nothing creates the env namespace, which is the whole of US-3.3a")
	}
	if clusterAt < 0 || nsAt > clusterAt {
		t.Fatalf("Namespace applied at %d, Cluster at %d — the namespace must come FIRST or the "+
			"first converge into a new environment 404s", nsAt, clusterAt)
	}

	// D8-adjacent: the SERVICE manifest came from placement, not a guess.
	//
	// Asserted against the Cluster object specifically. US-3.3a's first version
	// searched the concatenated applied set, which the tenancy Namespace also
	// satisfies — so mutating driver.Spec.Namespace to "env-victim", rendering
	// one tenant's database into another tenant's namespace, left this test
	// GREEN. An assertion answered by a different object than the one it names
	// is not an assertion.
	if clusterAt < 0 {
		t.Fatal("no Cluster in the applied set")
	}
	// Compared as a VALUE, not as a substring. `strings.Contains(…, "namespace:
	// env-9f3c1a2b")` is satisfied by "env-9f3c1a2b-shadow", so mutating the
	// namespace to namespace+"-shadow" survived this assertion in isolation and
	// was caught only by TestApplyIsIdempotent's byte comparison, under an
	// unrelated name. Round 2's own lesson, one level down.
	var got struct {
		Metadata struct {
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal(objs[clusterAt], &got); err != nil {
		t.Fatalf("the applied Cluster does not parse: %v", err)
	}
	if got.Metadata.Namespace != "env-9f3c1a2b" {
		t.Fatalf("the CNPG Cluster was placed in %q, want the resolved namespace "+
			"env-9f3c1a2b", got.Metadata.Namespace)
	}
}

func TestConvergeObservesClusterStatus(t *testing.T) {
	// Terminal phases surface as a status; transient ones signal ErrNotConverged.
	// The three phase strings a REAL CNPG operator emitted during the US-3.3 live
	// drill are included verbatim.
	terminalCases := map[string]string{"Cluster in healthy state": "ready"}
	for phase, want := range terminalCases {
		a := newFakeApplier(phase)
		got, err := newRenderer(a).Converge(context.Background(), svc("svc_db01", "provisioning"))
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("phase %q → want %q, got %q", phase, want, got)
		}
	}
	for _, phase := range []string{"", "Setting up primary", "Waiting for the instances to become active"} {
		a := newFakeApplier(phase)
		if _, err := newRenderer(a).Converge(context.Background(), svc("svc_db01", "provisioning")); !errors.Is(err, agent.ErrNotConverged) {
			t.Fatalf("transient phase %q must signal ErrNotConverged, got %v", phase, err)
		}
	}
}

// A cluster that is applied but not yet healthy must NOT report a transient
// status: it returns ErrNotConverged so the agent leaves the row outstanding.
// Reporting "provisioning" would advance observed_generation and the service
// would never be re-polled to reach ready (the blocker this test now pins).
func TestNotYetReadyDoesNotReportTransientStatus(t *testing.T) {
	a := newFakeApplier("Setting up primary")
	_, err := newRenderer(a).Converge(context.Background(), svc("svc_db01", "provisioning"))
	if !errors.Is(err, agent.ErrNotConverged) {
		t.Fatalf("a still-converging cluster must signal ErrNotConverged, got %v", err)
	}
	if a.applies != 1 {
		t.Fatal("converge must still apply the manifests while converging")
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
	// The objects must be deleted under the DRIVER's sanitized names, not the raw
	// service id — addressing the wrong name 404s and silently reports gone while
	// a real cluster keeps running (the blocker this pins).
	if len(a.deleted) == 0 {
		t.Fatal("teardown deleted nothing")
	}
	for _, d := range a.deleted {
		if strings.Contains(d, "svc_db01") {
			t.Fatalf("teardown addressed the RAW service id %q — the driver named it svc-db01", d)
		}
	}
	if a.deleted[0] != "env-9f3c1a2b/Cluster/svc-db01" {
		t.Fatalf("teardown must delete the cluster by its driver name: %v", a.deleted)
	}
	if a.applies != 0 {
		t.Fatal("a deleting service must not apply create manifests")
	}
}

func TestNamespaceDerivedFromDesired(t *testing.T) {
	// No placement → error, never a guessed namespace.
	s := svc("svc_db01", "provisioning")
	delete(s.Desired, "namespace")
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
	if !bytesEqual(a.applied["env-9f3c1a2b"], mustRender(t)) {
		t.Fatal("repeated converge produced different manifests — not idempotent")
	}
}

// mustRender is what ONE converge applies: the environment's namespace first
// (US-3.3a), then the service's own manifests. Both halves are
// DERIVED from the renderers rather than retyped, so a manifest added to either
// is covered without anyone remembering.
func mustRender(t *testing.T) [][]byte {
	t.Helper()
	tm, err := tenancy.Render(tenancy.Spec{
		Namespace: "env-9f3c1a2b", Cell: "cell-0",
	})
	if err != nil {
		t.Fatal(err)
	}
	var out [][]byte
	for _, o := range tm {
		out = append(out, o.YAML)
	}
	m, err := cnpg.New().Render(driver.Spec{
		Name: "svc_db01", Namespace: "env-9f3c1a2b", Product: "postgres",
		Shape: map[string]any{"size": "dev"}, Instances: 1, Cell: "cell-0",
		GSAEmail: "sa@steloit-dev.iam.gserviceaccount.com", WALBucket: "steloit-dev-wal-customer",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range m {
		out = append(out, o.YAML)
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

// The namespace crosses TWO module boundaries: the api embeds it in desired
// (US-3.3 Step 1), the poll ships it as raw JSON, the agent decodes it to
// map[string]any, and the renderer reads it. This pins that seam end-to-end
// over the actual wire encoding — a json tag change on either side breaks here
// rather than at apply time on a live cluster.
func TestNamespaceSurvivesTheWire(t *testing.T) {
	// exactly what the api's desiredDoc produces (US-3.3 Step 1)
	apiDesired := []byte(`{"product":"postgres","namespace":"env-9f3c1a2b","shape":{"size":"dev"}}`)
	// exactly how the poll ships a row and the agent decodes it
	wire := []byte(`{"id":"svc_db01","cell_id":"cell-0","product":"postgres","status":"provisioning","generation":1,"desired":` + string(apiDesired) + `}`)
	var decoded agent.DesiredService
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	a := newFakeApplier("Cluster in healthy state")
	if _, err := newRenderer(a).Converge(context.Background(), decoded); err != nil {
		t.Fatalf("the renderer could not use the api-produced desired doc: %v", err)
	}
	if _, ok := a.applied["env-9f3c1a2b"]; !ok {
		t.Fatalf("namespace did not survive api → wire → agent → renderer; applied into %v", keysOf(a.applied))
	}
}

func keysOf(m map[string][][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
