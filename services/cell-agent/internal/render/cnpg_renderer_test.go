package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	return nil
}

func svc(id, status string) agent.DesiredService {
	return agent.DesiredService{
		ID: id, CellID: "cell-0", Product: "postgres", Status: status, Generation: 1,
		Desired: map[string]any{
			"product": "postgres", "shape": map[string]any{"size": "dev"},
			"namespace": "env-0123456789abcdef0123456789abcdef",
		},
	}
}

func newRenderer(applier *fakeApplier) *CNPGRenderer {
	return NewCNPGRenderer(cnpg.New(), applier, "cell-0", "sa@steloit-dev.iam.gserviceaccount.com", "steloit-dev-wal-customer", quiet())
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
		Namespace: "env-0123456789abcdef0123456789abcdef", Cell: "cell-0",
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
	apiDesired := []byte(`{"product":"postgres","namespace":"env-0123456789abcdef0123456789abcdef","shape":{"size":"dev"}}`)
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
