package render

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	mu           sync.Mutex
	applied      map[string][][]byte // namespace/name → objects
	deleted      []string
	live         map[string]bool // ns/name → exists, like an API server
	phase        string
	applyErr     error
	observeErr   error
	deleteErr    error // fault seam: a Delete that the API server refuses
	deleteErrAt  int   // fail on the Nth Delete (0 = the first)
	applies      int
	observedName string

	// onDelete runs after a Delete is applied, so a test can model an object
	// that OUTLIVES an accepted delete (a finalizer, graceful termination) —
	// which is what a real API server does and what the fake otherwise hides.
	onDelete func()
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
	f.observedName = name
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
	if f.deleteErr != nil && len(f.deleted) == f.deleteErrAt {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, ns+"/"+kind+"/"+name)
	delete(f.live, ns+"/"+name)
	if f.onDelete != nil {
		f.onDelete()
	}
	return nil
}

func svc(id, status string) agent.DesiredService {
	return agent.DesiredService{
		ID: id, CellID: "cell-0", Product: "postgres", Status: status, Generation: 1,
		Desired: map[string]any{
			"product": "postgres", "shape": map[string]any{"size": "dev"},
			"namespace": "env-0123456789abcdef0123456789abcdef",
			// The plan's per-environment envelope, as the control plane ships it:
			// resolved VALUES, never a plan name. `pro` here.
			"quota": map[string]any{"cpu": "8", "memory": "16Gi", "storage": "100Gi"},
		},
	}
}

func newRenderer(applier *fakeApplier) *CNPGRenderer {
	return NewCNPGRenderer(cnpg.New(), applier, "cell-0", "sa@steloit-dev.iam.gserviceaccount.com", "steloit-dev-wal-customer", testAPIServerCIDR, quiet())
}

func TestCNPGRendererAppliesRenderedManifests(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	status, err := newRenderer(a).Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning"))
	if err != nil {
		t.Fatal(err)
	}
	if status != "ready" {
		t.Fatalf("healthy cluster → want ready, got %q", status)
	}
	objs := a.applied["env-0123456789abcdef0123456789abcdef"]
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
	// env-0123456789abcdef0123456789abcdef")` is satisfied by "env-0123456789abcdef0123456789abcdef-shadow", so mutating the
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
	if got.Metadata.Namespace != "env-0123456789abcdef0123456789abcdef" {
		t.Fatalf("the CNPG Cluster was placed in %q, want the resolved namespace "+
			"env-0123456789abcdef0123456789abcdef", got.Metadata.Namespace)
	}
}

func TestConvergeObservesClusterStatus(t *testing.T) {
	// Terminal phases surface as a status; transient ones signal ErrNotConverged.
	// The three phase strings a REAL CNPG operator emitted during the US-3.3 live
	// drill are included verbatim.
	terminalCases := map[string]string{"Cluster in healthy state": "ready"}
	for phase, want := range terminalCases {
		a := newFakeApplier(phase)
		got, err := newRenderer(a).Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning"))
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("phase %q → want %q, got %q", phase, want, got)
		}
	}
	for _, phase := range []string{"", "Setting up primary", "Waiting for the instances to become active"} {
		a := newFakeApplier(phase)
		if _, err := newRenderer(a).Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); !errors.Is(err, agent.ErrNotConverged) {
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
	_, err := newRenderer(a).Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning"))
	if !errors.Is(err, agent.ErrNotConverged) {
		t.Fatalf("a still-converging cluster must signal ErrNotConverged, got %v", err)
	}
	if a.applies != 1 {
		t.Fatal("converge must still apply the manifests while converging")
	}
}

func TestDeletingConvergesToGoneAndDeletes(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	got, err := newRenderer(a).Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "deleting"))
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
		if strings.Contains(d, "svc_0123456789abcdef0123456789abcdef") {
			t.Fatalf("teardown addressed the RAW service id %q — the driver named it svc-0123456789abcdef0123456789abcdef", d)
		}
	}
	if a.deleted[0] != "env-0123456789abcdef0123456789abcdef/Cluster/svc-0123456789abcdef0123456789abcdef" {
		t.Fatalf("teardown must delete the cluster by its driver name: %v", a.deleted)
	}
	if a.applies != 0 {
		t.Fatal("a deleting service must not apply create manifests")
	}
}

func TestNamespaceDerivedFromDesired(t *testing.T) {
	// No placement → error, never a guessed namespace.
	s := svc("svc_0123456789abcdef0123456789abcdef", "provisioning")
	delete(s.Desired, "namespace")
	if _, err := newRenderer(newFakeApplier("")).Converge(context.Background(), s); err == nil {
		t.Fatal("a service with no resolved namespace must error, not guess")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	for range 3 {
		if _, err := r.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err != nil {
			t.Fatal(err)
		}
	}
	// The renderer applies each converge (SSA is idempotent server-side); the
	// applied object set is byte-identical each time.
	if a.applies != 3 {
		t.Fatalf("expected 3 idempotent applies, got %d", a.applies)
	}
	if !bytesEqual(a.applied["env-0123456789abcdef0123456789abcdef"], mustRender(t)) {
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
		APIServerCIDR: testAPIServerCIDR,
		Namespace:     "env-0123456789abcdef0123456789abcdef", Cell: "cell-0",
		Quota: tenancy.Quota{CPU: "8", Memory: "16Gi", Storage: "100Gi"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out [][]byte
	for _, o := range tm {
		out = append(out, o.YAML)
	}
	m, err := cnpg.New().Render(driver.Spec{
		Name: "svc_0123456789abcdef0123456789abcdef", Namespace: "env-0123456789abcdef0123456789abcdef", Product: "postgres",
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
	if _, err := newRenderer(a).Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err == nil {
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
	// The QUOTA crosses the same two boundaries and is the newer half: the api
	// resolves the org's plan to values, and the agent must read them back as
	// strings out of a decoded map[string]any. A json tag or type change on
	// either side breaks here rather than at apply time on a live cluster.
	apiDesired := []byte(`{"product":"postgres","namespace":"env-0123456789abcdef0123456789abcdef",` +
		`"quota":{"cpu":"8","memory":"16Gi","storage":"100Gi"},"shape":{"size":"dev"}}`)
	// exactly how the poll ships a row and the agent decodes it
	wire := []byte(`{"id":"svc_0123456789abcdef0123456789abcdef","cell_id":"cell-0","product":"postgres","status":"provisioning","generation":1,"desired":` + string(apiDesired) + `}`)
	var decoded agent.DesiredService
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	a := newFakeApplier("Cluster in healthy state")
	if _, err := newRenderer(a).Converge(context.Background(), decoded); err != nil {
		t.Fatalf("the renderer could not use the api-produced desired doc: %v", err)
	}
	if _, ok := a.applied["env-0123456789abcdef0123456789abcdef"]; !ok {
		t.Fatalf("namespace did not survive api → wire → agent → renderer; applied into %v", keysOf(a.applied))
	}
	// And the envelope arrived intact, in the rendered ResourceQuota.
	var quota []byte
	for _, o := range a.applied["env-0123456789abcdef0123456789abcdef"] {
		if bytes.Contains(o, []byte("kind: ResourceQuota")) {
			quota = o
		}
	}
	if quota == nil {
		t.Fatal("no ResourceQuota was applied — the environment has no ceiling")
	}
	// The envelope's two RENDERED dimensions. storage is carried in the doc and
	// deliberately not rendered (US-3.3e / US-3.3i), so it cannot be asserted here.
	for _, want := range []string{`requests.cpu: "8"`, "requests.memory: 16Gi"} {
		if !bytes.Contains(quota, []byte(want)) {
			t.Fatalf("the envelope did not survive the wire: %q missing from\n%s", want, quota)
		}
	}
}

func keysOf(m map[string][][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// GONE MUST MEAN GONE. The renderer already refuses to report `gone` when a
// Delete fails or when its objects cannot be enumerated — and both were green
// mutations, because `fakeApplier` had no way to fail a Delete at all
// (`observeErr` existed and was never set; there was no `deleteErr`).
//
// What the guard protects: `gone` makes the control plane mark the service
// deleted and STOP METERING, while the CNPG cluster keeps running and keeps
// costing us. A 403 or a 409 must not read as success.
func TestTeardownNeverReportsGoneWhenSomethingFailed(t *testing.T) {
	// Fail the first Delete, then the second — an error at any position must
	// prevent `gone`, not just at position zero.
	for at := 0; at < 2; at++ {
		t.Run(fmt.Sprintf("deleteFailsAt%d", at), func(t *testing.T) {
			a := newFakeApplier("Cluster in healthy state")
			r := newRenderer(a)
			ctx := context.Background()
			if _, err := r.Converge(ctx, svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err != nil {
				t.Fatal(err)
			}
			a.deleteErr, a.deleteErrAt = errors.New("403 forbidden"), at
			a.deleted = nil

			status, err := r.Converge(ctx, svc("svc_0123456789abcdef0123456789abcdef", "deleting"))
			if err == nil {
				t.Fatalf("a refused Delete at %d reported success", at)
			}
			if status == "gone" {
				t.Fatalf("reported gone though a Delete was refused — the control plane marks " +
					"the service deleted and stops metering while its cluster keeps running")
			}
		})
	}

	// Observe failing must not read as "not created yet" either.
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	if _, err := r.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err != nil {
		t.Fatal(err)
	}
	a.observeErr = errors.New("403 forbidden")
	_, err := r.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning"))
	if err == nil {
		t.Fatal("a refused Observe was swallowed — the service sits in provisioning forever")
	}
	// It must be THE OBSERVE FAILURE, not ErrNotConverged. Swallowing the error
	// leaves phase == "", which maps to ErrNotConverged — also an error, so an
	// `err != nil` assertion passes either way and the mutation survives. A 403
	// is permanent and must be visible as itself; ErrNotConverged says "ask
	// again in 10s", which is how a broken cell looks exactly like a slow one.
	if errors.Is(err, agent.ErrNotConverged) {
		t.Fatalf("a refused Observe surfaced as ErrNotConverged — a permanent failure "+
			"disguised as a transient one, retried forever with no signal: %v", err)
	}
	if !strings.Contains(err.Error(), "403 forbidden") {
		t.Fatalf("the error does not carry the Observe failure: %v", err)
	}

	// A teardown that cannot ENUMERATE its objects must not report gone either.
	// The driver refuses a non-postgres product, which is the reachable form of
	// "this binary cannot work out what this service owns".
	c := newFakeApplier("Cluster in healthy state")
	bad := svc("svc_0123456789abcdef0123456789abcdef", "deleting")
	bad.Product = "valkey"
	if status, err := newRenderer(c).Converge(context.Background(), bad); err == nil || status == "gone" {
		t.Fatalf("teardown reported %q/%v for a service whose objects it cannot enumerate — "+
			"the row is marked deleted while whatever it owns keeps running", status, err)
	}

	// Positive control: with nothing failing, teardown still reports gone.
	b := newFakeApplier("Cluster in healthy state")
	r2 := newRenderer(b)
	if _, err := r2.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err != nil {
		t.Fatal(err)
	}
	if status, err := r2.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "deleting")); err != nil || status != "gone" {
		t.Fatalf("a clean teardown must report gone: %q %v", status, err)
	}
}

// Converge observes the CLUSTER, not whichever manifest happens to be first or
// last. Observing manifests[len-1] instead of the Cluster was green: in
// production that GETs /clusters/<name>-nightly, 404s, and the service never
// reaches ready and is never metered. The expected name is DERIVED from the
// driver, never retyped.
func TestConvergeObservesTheClusterObject(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	if _, err := newRenderer(a).Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err != nil {
		t.Fatal(err)
	}
	ms, err := cnpg.New().Render(driver.Spec{
		Name: "svc_0123456789abcdef0123456789abcdef", Namespace: "env-0123456789abcdef0123456789abcdef", Product: "postgres",
		Shape: map[string]any{"size": "dev"}, Instances: 1, Cell: "cell-0",
		GSAEmail: "sa@steloit-dev.iam.gserviceaccount.com", WALBucket: "steloit-dev-wal-customer",
	})
	if err != nil {
		t.Fatal(err)
	}
	var want string
	for _, m := range ms {
		if m.Kind == "Cluster" {
			want = m.Name
		}
	}
	if want == "" {
		t.Fatal("the driver rendered no Cluster")
	}
	if a.observedName != want {
		t.Fatalf("Converge observed %q, want the Cluster %q — observing any other object "+
			"404s, so the service never reaches ready and is never metered", a.observedName, want)
	}
}

// EVERY STATUS THIS AGENT CAN EMIT MUST BE A LEGAL EDGE FROM `provisioning`.
//
// The control plane allows provisioning → {ready, failed, deleting}. A writeback
// of anything else is rejected every tick: observed_generation never advances,
// the row stays outstanding, and the service is retried forever with nothing
// visible — the failure statusFromPhase explicitly chose `failed` over
// `degraded` to avoid, which `"Waiting for user action": "degraded"` then
// reintroduced from the other side.
//
// The two modules have separate go.mod files and no go.work, so this set is
// duplicated rather than imported. TestTheAgentsLegalEdgesMatchTheStatusMachine
// on the API side pins the other copy; changing one alone fails there.
func TestEveryTerminalStatusIsALegalEdgeFromProvisioning(t *testing.T) {
	legalFromProvisioning := map[string]bool{"ready": true, "failed": true, "deleting": true}

	seen := 0
	for phase, status := range phaseStatus {
		if !terminal(status) {
			continue
		}
		seen++
		if !legalFromProvisioning[status] {
			t.Errorf("phase %q maps to terminal status %q, which provisioning cannot "+
				"transition to — the writeback is rejected every tick and the service is "+
				"retried forever with no signal", phase, status)
		}
	}
	if got := statusFromPhase("a phase CNPG has not shipped yet"); !legalFromProvisioning[got] {
		t.Errorf("an unknown phase maps to %q, which provisioning cannot transition to", got)
	}
	if seen == 0 {
		t.Fatal("phaseStatus yielded no terminal status — this test would prove nothing")
	}
}

// THE RENDERER READS THE ENVELOPE FROM THE DOC, it does not carry one.
//
// Every other fixture ships `pro`, so hardcoding pro's envelope in the renderer
// was a GREEN mutation — indistinguishable from reading it. This drives the
// free and enterprise rows instead, which is the only way that distinction
// shows up.
func TestTheRenderedQuotaComesFromTheDocNotTheRenderer(t *testing.T) {
	for _, tc := range []struct{ plan, cpu, mem, sto string }{
		{"free", "1", "2Gi", "10Gi"},
		{"enterprise", "16", "32Gi", "250Gi"},
	} {
		t.Run(tc.plan, func(t *testing.T) {
			a := newFakeApplier("Cluster in healthy state")
			s := svc("svc_0123456789abcdef0123456789abcdef", "provisioning")
			s.Desired["quota"] = map[string]any{"cpu": tc.cpu, "memory": tc.mem, "storage": tc.sto}
			if _, err := newRenderer(a).Converge(context.Background(), s); err != nil {
				t.Fatal(err)
			}
			var quota []byte
			for _, o := range a.applied["env-0123456789abcdef0123456789abcdef"] {
				if bytes.Contains(o, []byte("kind: ResourceQuota")) {
					quota = o
				}
			}
			if quota == nil {
				t.Fatal("no ResourceQuota applied")
			}
			// storage is deliberately NOT among these: US-3.3e withholds
			// requests.storage because the API cannot predict the PVC the cell
			// will create, so it cannot refuse an order that will not fit
			// (US-3.3i). The doc still CARRIES it — the value below is the
			// founder's — which is why the fixture sets it and the assertion
			// list does not.
			for _, want := range []string{
				`requests.cpu: "` + tc.cpu + `"`,
				"requests.memory: " + tc.mem,
			} {
				if !bytes.Contains(quota, []byte(want)) {
					t.Errorf("%s: %q missing — the renderer is not reading the doc:\n%s", tc.plan, want, quota)
				}
			}
			if bytes.Contains(quota, []byte("requests.storage")) {
				t.Errorf("%s: requests.storage is rendered again — see US-3.3i:\n%s", tc.plan, quota)
			}
			// And emphatically NOT pro's, which every other fixture uses.
			if bytes.Contains(quota, []byte(`requests.cpu: "8"`)) && tc.cpu != "8" {
				t.Errorf("%s rendered pro's CPU ceiling", tc.plan)
			}
		})
	}
}

// A DOC WITH NO ENVELOPE MUST BE REFUSED, and the refusal must be reachable
// through quotaOf — the layer that FEEDS tenancy.Render.
//
// tenancy.Render's absence guard was verified one layer down, but nothing drove
// a desired doc that simply lacks the key: making quotaOf return a default when
// `desired["quota"]` is absent was a green mutation, and the renderer would then
// invent a ceiling nobody granted. The sibling field is already covered exactly
// this way (the namespace case deletes the key and asserts the refusal); this is
// the same line for the quota.
func TestAServiceWithNoEnvelopeInItsDocIsRefused(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	s := svc("svc_0123456789abcdef0123456789abcdef", "provisioning")
	delete(s.Desired, "quota")

	if _, err := newRenderer(a).Converge(context.Background(), s); err == nil {
		t.Fatal("a desired doc with no envelope was accepted — the renderer invented a ceiling " +
			"the control plane never granted")
	} else if !strings.Contains(err.Error(), "no quota envelope") {
		t.Fatalf("the refusal must name the missing envelope: %v", err)
	}
	if len(a.applied) != 0 {
		t.Fatalf("objects were applied despite the refusal: %v", keysOf(a.applied))
	}
}

// TEARDOWN MUST NOT DEPEND ON THE PLAN TABLE. Deleting a service is
// `never_gated: self_deletion` in plans.json — plans gate capabilities, never
// safety — and the deleting branch returns before any tenancy object is
// rendered, so the envelope is never read on that path.
func TestTeardownDoesNotNeedAnEnvelope(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	if _, err := r.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err != nil {
		t.Fatal(err)
	}
	del := svc("svc_0123456789abcdef0123456789abcdef", "deleting")
	delete(del.Desired, "quota")

	status, err := r.Converge(context.Background(), del)
	if err != nil {
		t.Fatalf("teardown required an envelope it never reads: %v", err)
	}
	if status != "gone" {
		t.Fatalf("teardown reported %q, want gone", status)
	}
}

// EVERY PHASE IS PINNED BY CLASSIFICATION, NOT JUST BY COUNT.
//
// The sibling tests assert that the table is non-empty and that its terminal
// values are legal edges. Neither sees a RECLASSIFICATION: measured, moving
// "Cluster has incomplete or invalid image catalog" from `failed` to
// `provisioning` left the whole suite green in both modules — a permanently
// broken cluster read as still-converging and retried forever, which is the
// exact defect statusFromPhase's own comment argues thirty lines about.
//
// The expectations are written out, because a test that derives them from the
// table it is checking asserts nothing. Iterating phaseStatus (rather than the
// expectations) also means a NEW phase cannot be added unclassified: it has no
// entry here and fails.
func TestEveryCNPGPhaseKeepsItsClassification(t *testing.T) {
	want := map[string]string{
		"Cluster in healthy state": "ready",

		"":                   "provisioning",
		"Setting up primary": "provisioning",
		"Waiting for the instances to become active":   "provisioning",
		"Creating a new replica":                       "provisioning",
		"Primary instance is being restarted in-place": "provisioning",
		"Switchover in progress":                       "provisioning",
		"Failing over":                                 "provisioning",
		"Upgrading cluster":                            "provisioning",

		// Terminal-bad. None of these contains "failed", "failure" or "error",
		// which is why the substring heuristic was abandoned — and why each one
		// has to be named rather than matched.
		"Waiting for user action":                                                                 "failed",
		"Cluster is unrecoverable and needs manual intervention":                                  "failed",
		"Invalid cluster definition":                                                              "failed",
		"Unable to create required cluster objects":                                               "failed",
		"Cluster has incomplete or invalid image catalog":                                         "failed",
		"Cluster cannot proceed to reconciliation due to an unknown plugin being required":        "failed",
		"Cluster cannot execute instance online upgrade due to missing architecture binary":       "failed",
		"Cluster cannot proceed to reconciliation due to an error while interacting with plugins": "failed",
	}

	for phase, got := range phaseStatus {
		exp, ok := want[phase]
		if !ok {
			t.Errorf("phase %q is in the table but not classified here — a new CNPG phase must be "+
				"classified deliberately, not inherit whatever the table says", phase)
			continue
		}
		if got != exp {
			t.Errorf("phase %q maps to %q, want %q. Reclassifying a terminal-bad phase as "+
				"transient makes a permanently broken cluster read as still-converging and "+
				"retried forever with no signal.", phase, got, exp)
		}
	}
	for phase := range want {
		if _, ok := phaseStatus[phase]; !ok {
			t.Errorf("phase %q was removed from the table; CNPG still emits it", phase)
		}
	}
	if len(phaseStatus) != len(want) {
		t.Fatalf("table has %d phases, expectations have %d", len(phaseStatus), len(want))
	}
}

// THE TEARDOWN TRIGGER HAS TWO REPRESENTATIONS AND ONLY ONE WAS DRIVEN.
//
// `svc.Status == "deleting" || asBool(svc.Desired["deleting"])` — every existing
// teardown test uses the STATUS arm (the svc() helper sets it), so deleting the
// desired-flag arm was a green mutation.
//
// That arm is not redundant: DeleteService writes the deleting desired doc and
// bumps the generation BEFORE the status transition, so there is a window where
// the agent is handed a service whose DESIRED says deleting and whose STATUS does
// not. Without this arm the agent re-APPLIES that service's manifests in that
// window and reports it ready — recreating what the control plane is tearing down.
func TestATeardownFlagInDesiredIsHonouredWithoutTheStatus(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	s := svc("svc_0123456789abcdef0123456789abcdef", "provisioning")
	s.Desired["deleting"] = true

	status, err := newRenderer(a).Converge(context.Background(), s)
	if err != nil {
		t.Fatalf("converge: %v", err)
	}
	if status != "gone" {
		t.Errorf("a service flagged deleting in its desired doc converged to %q, not gone — "+
			"DeleteService bumps the generation before the status edge, so this is the window "+
			"where the agent would re-apply what is being torn down", status)
	}
	if len(a.applied) != 0 {
		t.Errorf("manifests were APPLIED for a service being torn down: %v", keysOf(a.applied))
	}
	if len(a.deleted) == 0 {
		t.Error("nothing was deleted for a service flagged deleting in its desired doc")
	}
}

// TEARDOWN MUST NOT DELETE OBJECTS FOR A PRODUCT THIS DRIVER DOES NOT OWN.
//
// There is ONE renderer for all four products (cmd/cell-agent has no dispatch),
// so `[postgres, valkey, web, worker]` all reach this code. Replacing Render with
// Objects on the teardown path dropped Render's two entry guards, and measured:
// a `valkey` service in `deleting` converged to "gone" and issued Delete for
// `Cluster/svc-...` and `ScheduledBackup/svc-...-nightly` — postgres shapes for a
// product that has none. Reporting "gone" then marks the service deleted and
// stops its metering while whatever actually runs it keeps running.
func TestTeardownRefusesAProductThisDriverDoesNotOwn(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	s := svc("svc_0123456789abcdef0123456789abcdef", "deleting")
	s.Product = "valkey"
	s.Desired["product"] = "valkey"

	status, err := newRenderer(a).Converge(context.Background(), s)
	if err == nil {
		t.Fatalf("a valkey teardown was accepted and converged to %q", status)
	}
	if status == "gone" {
		t.Error("reported gone for a product this driver does not own — the service is marked " +
			"deleted and its metering stops while the real resource keeps running")
	}
	if len(a.deleted) != 0 {
		t.Errorf("deleted postgres-shaped objects for a valkey service: %v", a.deleted)
	}
}

// THE PRICED storage_gb MUST REACH THE APPLIED MANIFEST — asserted here, at the
// renderer, and read out of the YAML rather than the spec struct.
//
// T3.4c proved the DRIVER sizes the PVC from `storage_gb`. Nothing proved its
// only production caller hands it one: measured, `storage_gb` appeared ZERO times
// anywhere in this package, and deleting the key on the way from the desired doc
// to the driver left the entire cell-agent suite green. That is exactly the shape
// of the original defect — the driver read `shape["storage"]`, which the API's
// schema forbids, so the priced value was never read and every customer got the
// size default. Proving the driver right while leaving the wire between them
// unpinned is how that survived.
//
// float64, not int: this value arrives from JSON, so the production type is what
// the test must use.
func TestTheDesiredDocsStorageSizesTheAppliedCluster(t *testing.T) {
	for _, tc := range []struct {
		storage any
		want    string
	}{
		{float64(200), "200Gi"},
		{float64(78), "78Gi"},
		{float64(50), "50Gi"},
	} {
		a := newFakeApplier("Cluster in healthy state")
		s := svc("svc_0123456789abcdef0123456789abcdef", "provisioning")
		// Keep the fixture's own namespace and envelope — US-3.3a made the
		// namespace env-derived and US-3.3e made a doc with no `quota` a REFUSAL,
		// so replacing the whole Desired map here would fail for reasons that
		// have nothing to do with storage.
		s.Desired["shape"] = map[string]any{"size": "standard", "storage_gb": tc.storage}

		if _, err := newRenderer(a).Converge(context.Background(), s); err != nil {
			t.Fatalf("storage_gb=%v: %v", tc.storage, err)
		}
		var cluster string
		for _, doc := range a.applied[s.Desired["namespace"].(string)] {
			if strings.Contains(string(doc), "kind: Cluster") {
				cluster = string(doc)
			}
		}
		if cluster == "" {
			t.Fatalf("storage_gb=%v: no Cluster was applied", tc.storage)
		}
		if !strings.Contains(cluster, tc.want) {
			t.Errorf("storage_gb=%v: the applied Cluster does not carry %s — the priced storage "+
				"never reached the manifest:\n%s", tc.storage, tc.want, cluster)
		}
	}
}

// RECONCILIATION CONVERGES ON THE RETAINED VOLUME — measured through Converge,
// against what actually reaches the (fake) API server, on every tick.
//
// The founder ruling's last clause is "repeated reconciliation must converge
// rather than continuously attempting an impossible downgrade". An earlier
// version of this test lived in the driver package and rendered the same shape
// five times: `Driver.Render` is a pure function of its Spec with no state, no
// clock and no I/O, so those were five identical calls to a deterministic
// function and the only thing they could catch was non-determinism. Review
// named it, and it was right — the property is about SUCCESSIVE CONVERGES, so
// the test has to converge successively and read what was applied.
func TestSuccessiveConvergesNeverAskForASmallerVolume(t *testing.T) {
	const ns = "env-0123456789abcdef0123456789abcdef"
	id := "svc_0123456789abcdef0123456789abcdef"

	downgraded := svc(id, "ready")
	// The desired doc AFTER a standard->dev downgrade: the size fell, the
	// storage the cluster already carries did not. This is exactly the document
	// UpdateService writes (its API-side test is
	// TestASizeDowngradeKeepsTheProvisionedStorageInDesiredState).
	// 200, NOT 50. 50 is `standard`'s catalog floor, so a driver that re-derived
	// from the catalog instead of reading storage_gb would render the same number
	// and this test could not tell the two apart — the defect QA measured
	// surviving the whole api module. 200 is a value no floor produces.
	downgraded.Desired["shape"] = map[string]any{"size": "dev", "storage_gb": 200}

	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	for tick := range 3 {
		if _, err := r.Converge(context.Background(), downgraded); err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}
		got := appliedStorageGB(t, a, ns)
		if got != 200 {
			t.Fatalf("tick %d applied a %dGi volume against the 200Gi already provisioned — a PVC "+
				"cannot shrink, so the CSI driver refuses this, the service never reports observed "+
				"and every subsequent tick asks for the same impossible size again", tick, got)
		}
	}
	if a.applies != 3 {
		t.Fatalf("expected 3 converges to apply 3 times, got %d", a.applies)
	}

	// THE DISCRIMINATOR. Without it the loop above passes for a reason unrelated
	// to the ratchet — e.g. if the driver ignored storage_gb entirely and always
	// rendered 50. A doc whose storage_gb is GONE must apply the size's own
	// floor, which for dev is smaller. If this stops being smaller, the loop
	// above proves nothing and this fails loudly rather than silently.
	lost := svc(id, "ready")
	lost.Desired["shape"] = map[string]any{"size": "dev"}
	b := newFakeApplier("Cluster in healthy state")
	if _, err := NewCNPGRenderer(cnpg.New(), b, "cell-0", "sa@steloit-dev.iam.gserviceaccount.com",
		"steloit-dev-wal-customer", testAPIServerCIDR, quiet()).Converge(context.Background(), lost); err != nil {
		t.Fatal(err)
	}
	if floor := appliedStorageGB(t, b, ns); floor >= 200 {
		t.Fatalf("a desired doc with NO storage_gb applied %dGi, not less than the retained 200 — "+
			"the assertions above cannot distinguish the ratchet from a driver that ignores the "+
			"key, so they prove nothing", floor)
	}
}

// appliedStorageGB reads spec.storage.size off the CNPG Cluster the renderer
// actually handed the applier — not off a re-render, which would assert the
// driver against itself.
func appliedStorageGB(t *testing.T, a *fakeApplier, ns string) int {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, o := range a.applied[ns] {
		var doc map[string]any
		if yaml.Unmarshal(o, &doc) != nil {
			continue
		}
		if doc["kind"] != "Cluster" {
			continue
		}
		storage, _ := doc["spec"].(map[string]any)
		if storage == nil {
			continue
		}
		st, _ := storage["storage"].(map[string]any)
		if st == nil {
			continue
		}
		s := fmt.Sprint(st["size"])
		n, err := strconv.Atoi(strings.TrimSuffix(s, "Gi"))
		if err != nil {
			t.Fatalf("spec.storage.size is %q — the driver must render whole Gi, and a unit change "+
				"would otherwise read as a number change", s)
		}
		return n
	}
	t.Fatalf("no CNPG Cluster was applied to %s: %d objects", ns, len(a.applied[ns]))
	return 0
}

// EVERY DIMENSION THE CUSTOMER PAYS FOR MUST MOVE ITS OWN PROVISIONED FIELD.
//
// US-3.16's root cause was not a forgotten key. "Priced" and "provisioned" were
// two representations with nothing tying them together: `estimates.Price` read
// `shape["ha"]` and charged $19/month, nothing here read it, and both suites were
// green while customers got one instance.
//
// THE FIRST VERSION OF THIS GUARD COMPARED BYTES, AND QA BROKE IT. Neutralise
// both priced reads AND add a `steloit.dev/shape` annotation to the template —
// a routine SSA drift-detection pattern, not a contrived change — and the guard
// PASSED with the cell provisioning neither dimension. The manifests differed;
// they just differed somewhere that builds nothing. That is the very bug this
// test exists to prevent, reported green by the test.
//
// So each row now names the FIELD its dimension is supposed to move, and the
// assertion reads that field out of the Cluster by path. A row without a reader
// does not compile, so the next dimension cannot be added as a bytes-only row.
func TestEveryPricedDimensionMovesItsOwnProvisionedField(t *testing.T) {
	const ns = "env-0123456789abcdef0123456789abcdef"
	id := "svc_0123456789abcdef0123456789abcdef"

	priced := []struct {
		key      string // the pricing.json term that makes it cost money
		a, b     map[string]any
		read     func(t *testing.T, a *fakeApplier) string // the provisioned field it must move
		whatItIs string
	}{
		{
			key:      "postgres.storage_cents_per_gb",
			a:        map[string]any{"size": "standard", "storage_gb": 50},
			b:        map[string]any{"size": "standard", "storage_gb": 200},
			read:     func(t *testing.T, a *fakeApplier) string { return clusterField(t, a, ns, "spec.storage.size") },
			whatItIs: "storage beyond the included amount -> spec.storage.size",
		},
		{
			key:      "postgres.ha_cents",
			a:        map[string]any{"size": "standard", "ha": false},
			b:        map[string]any{"size": "standard", "ha": true},
			read:     func(t *testing.T, a *fakeApplier) string { return clusterField(t, a, ns, "spec.instances") },
			whatItIs: "high availability — a standby -> spec.instances",
		},
		// postgres.sizes[*].base_cents is DELIBERATELY ABSENT — see
		// TestTheSizeYouPayForBuysOnlyItsStorageFloor. A row here would have to
		// name a field, and there is none: the Cluster declares no `resources:`
		// until US-3.3d rules the SIZE -> vCPU/RAM mapping. Under the old
		// bytes-comparison it "passed", answered by the storage floor one row up.
	}

	converge := func(t *testing.T, shape map[string]any) *fakeApplier {
		t.Helper()
		a := newFakeApplier("Cluster in healthy state")
		s := svc(id, "ready")
		s.Desired["shape"] = shape
		if _, err := newRenderer(a).Converge(context.Background(), s); err != nil {
			t.Fatalf("converge %v: %v", shape, err)
		}
		return a
	}

	// THE ROW SET MUST COVER THE CATALOG, and the catalog is the one that moves.
	// Review measured the gap: adding `postgres.connections_cents` to pricing.json
	// left every package `ok`. A hand-written list claiming "a dimension that
	// changes the bill and not the cluster now fails, whoever adds it next" while
	// nothing binds it to the bill is the same defect this test exists to catch,
	// committed by the test. The cell-agent is a separate module and must not
	// IMPORT the API's pricing table, so the file is read from the repo root —
	// the precedent is cnpg_test.go's catalogSizes.
	covered := map[string]bool{}
	for _, p := range priced {
		covered[p.key] = true
	}
	for _, key := range pricedPostgresDimensions(t) {
		if covered[key] || key == exemptBaseCents {
			continue
		}
		t.Errorf("pricing.json charges for %s and no row here names the field it provisions. "+
			"Add one — or, if nothing provisions it yet, add it to the exemption beside "+
			"%s with the task that owns it.", key, exemptBaseCents)
	}

	for _, p := range priced {
		t.Run(p.key, func(t *testing.T) {
			got, want := p.read(t, converge(t, p.a)), p.read(t, converge(t, p.b))
			if got == want {
				t.Fatalf("the catalog charges for %s (%s), and changing it leaves that field at %q.\n"+
					"The customer is billed for something the cell does not build.\n  %v\n  %v",
					p.key, p.whatItIs, got, p.a, p.b)
			}
		})
	}
}

// THE GUARD MUST STILL BE ABLE TO FAIL.
//
// The companion to the test above, and independent of it: a shape key the driver
// provably does NOT read must render an identical Cluster. If this ever fails,
// something is echoing the shape onto a surface that builds nothing — which is
// exactly what disarmed the first version of the guard — and the guard's passes
// have stopped meaning anything.
//
// `version` is the right probe: it is in the closed shape schema, it is NOT
// priced, and no template consumes it.
func TestTheSeamGuardCanStillFail(t *testing.T) {
	const ns = "env-0123456789abcdef0123456789abcdef"
	render := func(shape map[string]any) string {
		a := newFakeApplier("Cluster in healthy state")
		s := svc("svc_0123456789abcdef0123456789abcdef", "ready")
		s.Desired["shape"] = shape
		if _, err := newRenderer(a).Converge(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, o := range a.applied[ns] {
			b.Write(o)
		}
		return b.String()
	}
	if render(map[string]any{"size": "standard", "version": "16"}) !=
		render(map[string]any{"size": "standard", "version": "17"}) {
		t.Fatal("an UNPRICED, unread shape key changed the rendered manifests. Something now " +
			"echoes the shape onto a surface that provisions nothing (a shape-hash annotation " +
			"would do it), so a byte-difference no longer implies anything was provisioned. " +
			"TestEveryPricedDimensionMovesItsOwnProvisionedField reads fields by path and is " +
			"unaffected — but check that nothing else in this package compares whole manifests.")
	}
}

// clusterField reads one dotted path out of the CNPG Cluster that reached the
// applier — the object the API server would store, not a re-render.
func clusterField(t *testing.T, a *fakeApplier, ns, path string) string {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, o := range a.applied[ns] {
		var doc map[string]any
		if yaml.Unmarshal(o, &doc) != nil || doc["kind"] != "Cluster" {
			continue
		}
		var cur any = doc
		for _, seg := range strings.Split(path, ".") {
			m, ok := cur.(map[string]any)
			if !ok {
				t.Fatalf("%s: %q is not a map", path, seg)
			}
			cur, ok = m[seg]
			if !ok {
				t.Fatalf("the applied Cluster has no %s — the field this dimension must move "+
					"does not exist, so nothing provisions it", path)
			}
		}
		return fmt.Sprint(cur)
	}
	t.Fatalf("no CNPG Cluster applied to %s", ns)
	return ""
}

// exemptBaseCents is the ONE priced dimension with no provisioned field, and it
// is named here rather than left out of the list, so that deleting the reminder
// test below cannot silently drop a $39/month dimension from the seam guard.
//
// US-3.3d owns removing it: all three sizes ARE ruled by the create frame's Size
// block (Dev 1 vCPU · 2 GB · $19, Standard 2 · 4 · $58, Performance 4 · 8 · $112),
// but the Cluster declares no `resources:` yet, so there is no field to read.
const exemptBaseCents = "postgres.sizes[*].base_cents"

// pricedPostgresDimensions reads every money term the catalog charges for under
// `postgres`, so this test's coverage is checked against the bill rather than
// against a list someone remembered to update.
func pricedPostgresDimensions(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRootFromRender(t), "services", "api", "internal", "estimates", "pricing.json"))
	if err != nil {
		t.Fatalf("read the pricing table: %v", err)
	}
	var doc struct {
		Postgres map[string]json.RawMessage `json:"postgres"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Postgres) == 0 {
		t.Fatal("no postgres block in pricing.json — this check would pass vacuously")
	}
	var out []string
	for k, v := range doc.Postgres {
		switch {
		case strings.HasPrefix(k, "$"):
			continue
		case k == "sizes":
			var sizes map[string]map[string]any
			if err := json.Unmarshal(v, &sizes); err != nil {
				t.Fatal(err)
			}
			inner := map[string]bool{}
			for _, fields := range sizes {
				for f := range fields {
					if strings.Contains(f, "cents") {
						inner[f] = true
					}
				}
			}
			for f := range inner {
				out = append(out, "postgres.sizes[*]."+f)
			}
		case strings.Contains(k, "cents"):
			out = append(out, "postgres."+k)
		}
	}
	sort.Strings(out)
	return out
}

// repoRootFromRender walks up for AGENTS.md and FAILS rather than defaulting —
// same rule as the driver package's repoRoot: a parity check must not silently
// disarm because it could not find the file it compares against.
func repoRootFromRender(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("repo root not found (AGENTS.md) — the catalog-coverage check must not silently disarm")
	return ""
}

// THE GAP US-3.3d OWNS, measured rather than asserted.
//
// Hold storage equal and `dev` ($19/mo) and `standard` ($58/mo) render
// BYTE-IDENTICAL manifests. The $39 difference buys nothing the cell builds: the
// Cluster declares no `resources:`, so both get the same pod. The only thing a
// size changes today is its storage floor.
//
// This test asserts the CURRENT truth on purpose. When US-3.3d rules the
// SIZE -> vCPU/RAM mapping it will fail, which is the point — it is the reminder
// that lands in the right place at the right time, rather than a comment nobody
// reads. Two of the three sizes are already fixed by the design spec's create
// frames ("PostgreSQL 16.4 Dev · 1 vCPU / 2 GB", "PostgreSQL db-main · 2 vCPU /
// 4 GB", and canon's db-main is `standard`); `performance` is not.
func TestTheSizeYouPayForBuysOnlyItsStorageFloor(t *testing.T) {
	const ns = "env-0123456789abcdef0123456789abcdef"
	render := func(shape map[string]any) string {
		a := newFakeApplier("Cluster in healthy state")
		s := svc("svc_0123456789abcdef0123456789abcdef", "ready")
		s.Desired["shape"] = shape
		if _, err := newRenderer(a).Converge(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, o := range a.applied[ns] {
			b.Write(o)
		}
		return b.String()
	}
	dev := render(map[string]any{"size": "dev", "storage_gb": 50})
	std := render(map[string]any{"size": "standard", "storage_gb": 50})
	if dev != std {
		t.Fatalf("dev and standard now render differently with storage held equal — if US-3.3d " +
			"has ruled the SIZE->vCPU/RAM mapping, DELETE this test and add base_cents to " +
			"TestEveryPricedDimensionChangesWhatIsApplied, which can then prove it honestly")
	}
	t.Log("dev($19) and standard($58) are byte-identical at equal storage — US-3.3d")
}

// The count itself, pinned to the promise it comes from.
func TestHARendersAStandby(t *testing.T) {
	const ns = "env-0123456789abcdef0123456789abcdef"
	for _, tc := range []struct {
		ha   bool
		want int
	}{{false, 1}, {true, 2}} {
		a := newFakeApplier("Cluster in healthy state")
		s := svc("svc_0123456789abcdef0123456789abcdef", "ready")
		s.Desired["shape"] = map[string]any{"size": "standard", "ha": tc.ha}
		if _, err := newRenderer(a).Converge(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if got := appliedInstances(t, a, ns); got != tc.want {
			t.Fatalf("ha=%v applied instances: %d, want %d — the create frame sells HA as "+
				"\"standby + auto-failover\", so exactly one standby must exist", tc.ha, got, tc.want)
		}
	}
}

// A manual pin may raise the count and must never lower it below what was sold.
func TestAnInstancePinCannotRemoveTheStandbyTheCustomerPaidFor(t *testing.T) {
	const ns = "env-0123456789abcdef0123456789abcdef"
	for _, tc := range []struct {
		name string
		pin  float64
		want int
	}{
		{"a pin of 1 on an HA service does not drop the standby", 1, 2},
		{"a pin below HA is floored, not honoured", 0, 2},
		// UNREACHABLE THROUGH THE API, and pinned as behaviour rather than as
		// intent: PriceWithInstances 422s any postgres pin ("capacity we cannot
		// price is capacity we must not provision"), so five unpriced instances
		// is not something the platform will produce. Kept because the arm exists
		// and hand-planted rows reach it; it is not an endorsement of the raise.
		{"a pin above HA still raises (unreachable via the API — see the comment)", 5, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newFakeApplier("Cluster in healthy state")
			s := svc("svc_0123456789abcdef0123456789abcdef", "ready")
			s.Desired["shape"] = map[string]any{"size": "standard", "ha": true}
			s.Desired["override"] = map[string]any{"instances": tc.pin, "reason": "capacity test"}
			if _, err := newRenderer(a).Converge(context.Background(), s); err != nil {
				t.Fatal(err)
			}
			if got := appliedInstances(t, a, ns); got != tc.want {
				t.Fatalf("pin=%v with ha applied %d instances, want %d — the price still charges "+
					"for HA, so no pin may take the service below it", tc.pin, got, tc.want)
			}
		})
	}
}

// appliedInstances reads spec.instances off the Cluster that reached the applier.
func appliedInstances(t *testing.T, a *fakeApplier, ns string) int {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, o := range a.applied[ns] {
		var doc map[string]any
		if yaml.Unmarshal(o, &doc) != nil || doc["kind"] != "Cluster" {
			continue
		}
		spec, _ := doc["spec"].(map[string]any)
		if spec == nil {
			continue
		}
		n, err := strconv.Atoi(fmt.Sprint(spec["instances"]))
		if err != nil {
			t.Fatalf("spec.instances is %v, not a number", spec["instances"])
		}
		return n
	}
	t.Fatalf("no CNPG Cluster applied to %s", ns)
	return 0
}

// THE INPUT SPACE, because "absent" and "unreadable" are not the same thing.
//
// QA probed the first version and found six classes that silently returned the
// WRONG count: `ha: "true"` (a string) meant no standby — US-3.16's own bug
// arriving by type instead of by omission — and an `override.instances` held as
// an int or a json.Number was dropped entirely. The wire path decodes into
// float64 so those shapes come from migrated or hand-planted rows, which is
// exactly the population the override machinery documents itself as serving.
//
// `cnpg.asGB` had already solved this one file away, with a comment saying
// reading a single numeric type "would silently ignore every storage_gb on the
// wire, which is the defect class this task exists to close". This is that rule,
// applied to the two keys the new code reads.
func TestInstancesOfReadsEveryShapeADesiredDocCanCarry(t *testing.T) {
	const ns = "env-0123456789abcdef0123456789abcdef"
	instances := func(t *testing.T, shape, override map[string]any) int {
		t.Helper()
		a := newFakeApplier("Cluster in healthy state")
		s := svc("svc_0123456789abcdef0123456789abcdef", "ready")
		s.Desired["shape"] = shape
		if override != nil {
			s.Desired["override"] = override
		}
		if _, err := newRenderer(a).Converge(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		return appliedInstances(t, a, ns)
	}

	t.Run("ha in every representation a doc can hold", func(t *testing.T) {
		for _, ha := range []any{true, "true"} {
			if got := instances(t, map[string]any{"size": "standard", "ha": ha}, nil); got != haInstances {
				t.Errorf("ha=%#v (%T) rendered %d instances, want %d — HA is billed either way, "+
					"so a representation this code cannot read must not mean 'no standby'", ha, ha, got, haInstances)
			}
		}
		// Genuinely absent, or explicitly false, is one instance.
		for _, shape := range []map[string]any{
			{"size": "standard"}, {"size": "standard", "ha": false}, {"size": "standard", "ha": nil},
		} {
			if got := instances(t, shape, nil); got != 1 {
				t.Errorf("%v rendered %d instances, want 1", shape, got)
			}
		}
	})

	t.Run("a pin in every numeric representation", func(t *testing.T) {
		for _, pin := range []any{float64(3), 3, int64(3), json.Number("3")} {
			got := instances(t, map[string]any{"size": "standard"}, map[string]any{"instances": pin, "reason": "capacity"})
			if got != 3 {
				t.Errorf("pin %#v (%T) rendered %d instances, want 3 — a deliberate pin must not "+
					"silently vanish because of its JSON type", pin, pin, got)
			}
		}
		// Unreadable or absurd values fall back to the floor rather than being trusted.
		for _, pin := range []any{"3", 2.7, 1e19, nil} {
			if got := instances(t, map[string]any{"size": "standard"}, map[string]any{"instances": pin}); got != 1 {
				t.Errorf("pin %#v (%T) rendered %d instances, want the floor of 1", pin, pin, got)
			}
		}
	})

	// The non-HA path lost its old `n >= 1` guard in the rewrite. Same answers,
	// different mechanism — so pin them, since every other case here sets ha.
	t.Run("the non-HA path is unchanged by the rewrite", func(t *testing.T) {
		for _, tc := range []struct {
			pin  any
			want int
		}{{float64(0), 1}, {float64(1), 1}, {float64(3), 3}, {float64(-2), 1}} {
			got := instances(t, map[string]any{"size": "standard", "ha": false},
				map[string]any{"instances": tc.pin, "reason": "capacity"})
			if got != tc.want {
				t.Errorf("ha=false pin=%v rendered %d, want %d", tc.pin, got, tc.want)
			}
		}
	})
}
