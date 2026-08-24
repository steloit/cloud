package render

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/steloit/cloud/services/cell-agent/internal/agent"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/tenancy"
	"github.com/steloit/cloud/services/cell-agent/internal/kube"
)

func TestDeleteRemovesEveryRenderedObject(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	// create both objects first
	if _, err := r.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err != nil {
		t.Fatal(err)
	}
	// now tear down
	if _, err := r.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "deleting")); err != nil {
		t.Fatal(err)
	}
	// the ScheduledBackup must not survive the cluster (it would keep firing
	// against a cluster that no longer exists)
	if len(a.deleted) < 2 {
		t.Fatalf("teardown deleted %d object(s); the driver rendered 2 (Cluster + ScheduledBackup): %v", len(a.deleted), a.deleted)
	}
	for _, ns := range []string{"env-0123456789abcdef0123456789abcdef/Cluster/svc-0123456789abcdef0123456789abcdef", "env-0123456789abcdef0123456789abcdef/ScheduledBackup/svc-0123456789abcdef0123456789abcdef-nightly"} {
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

	status, err := r.Converge(ctx, svc("svc_0123456789abcdef0123456789abcdef", "provisioning"))
	if err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("a failed CNPG phase must report failed, got %q", status)
	}
	liveAfterFailure := len(a.live)
	firstManifests := append([][]byte(nil), a.applied["env-0123456789abcdef0123456789abcdef"]...)
	if liveAfterFailure == 0 {
		// Guard against a vacuous pass: if the failed attempt applied nothing,
		// the "no new objects" check below compares 0 to 0 and proves nothing.
		t.Fatal("the failed attempt applied no objects — this test would prove nothing")
	}

	// The retry: same desired state, cluster now healthy.
	a.phase = "Cluster in healthy state"
	status, err = r.Converge(ctx, svc("svc_0123456789abcdef0123456789abcdef", "provisioning"))
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
	second := a.applied["env-0123456789abcdef0123456789abcdef"]
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
		if !strings.Contains(k, "svc-0123456789abcdef0123456789abcdef") {
			t.Fatalf("retry left an object under an unexpected name: %s", k)
		}
	}
}

// testNamespace is the ADR-0012 shape, sanitize(env_id) — e.g. env_9f3c1a2b
// becomes env-0123456789abcdef0123456789abcdef. The fixtures used the pre-ADR-0012 `proj--env` form
// (`acme--prod`), which the platform can no longer produce.
// PRODUCTION-SHAPED IDENTIFIERS. ids.New mints a 32-hex suffix, so a real
// namespace is `env-<32 hex>` (36 chars) and a real service id `svc_<32 hex>`.
// These fixtures were `env-9f3c1a2b` (12) and `svc_db01` (8) — three times
// shorter than anything the platform can produce — so every rule keyed on
// identifier LENGTH was unpinned. Four such mutations survived, two of which
// switched off this task's headline behaviours for every real environment: a
// Delete that no-ops above 12 chars, and a teardown that deletes nothing and
// still reports gone.
//
// Provenance worth recording: ADR-0012 writes the shape as `env_9f3c… →
// env-9f3c…` with a typographic ELLIPSIS, and the fixture read that elision as
// a literal. An elided example became the test data (AGENTS.md: examples are
// normative).
const testNamespace = "env-0123456789abcdef0123456789abcdef"

// envObjectKeys is the set of objects that belong to the ENVIRONMENT rather than
// to any service — today the namespace, and whatever US-3.3c adds beside it.
// DERIVED from tenancy.Render, never retyped: a hardcoded list silently stops
// covering an object added there, which is how a test starts asserting less
// than it claims.
func envObjectKeys(t *testing.T) map[string]bool {
	t.Helper()
	ms, err := tenancy.Render(tenancy.Spec{Namespace: testNamespace, Cell: "cell-0",
		Quota: tenancy.Quota{CPU: "8", Memory: "16Gi", Storage: "100Gi"}})
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

	if _, err := r.Converge(ctx, svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err != nil {
		t.Fatal(err)
	}
	if len(a.live) == 0 {
		t.Fatal("the failed attempt applied nothing — this test would prove nothing")
	}
	status, err := r.Converge(ctx, svc("svc_0123456789abcdef0123456789abcdef", "deleting"))
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
		s := svc("svc_0123456789abcdef0123456789abcdef", "deleting")
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
	if _, err := newRenderer(a).Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err != nil {
		t.Fatal(err)
	}
	s := svc("svc_0123456789abcdef0123456789abcdef", "deleting")
	if status, err := newRenderer(a).Converge(context.Background(), s); err != nil {
		t.Fatalf("a legitimate teardown was refused: %v", err)
	} else if status != "gone" {
		t.Fatalf("teardown reported %q, want gone", status)
	}
}

// A SERVICE MUST STAY DELETABLE WHATEVER ITS SIZE.
//
// T3.4c made the driver refuse an uncatalogued size, which is right for CREATE —
// silently sizing an unknown size is how `performance` shipped with the smallest
// volume. But teardown ran through the same Render call, so the refusal made such
// a service UNDELETABLE: its cluster kept running, and UpdateService refuses a
// row that is already `deleting`.
//
// This is reachable by ordinary deploy skew, with no bad actor: the API accepts a
// size the moment it is in pricing.json, while a cell still on the previous agent
// binary has no includedFloorGB entry for it. The cross-module catalog test
// cannot see it, because it binds the two FILES, not the two BINARIES.
func TestAServiceWithAnUncatalogedSizeIsStillDeletable(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	ctx := context.Background()

	// Create with a size this binary knows, so there is something to tear down.
	if _, err := r.Converge(ctx, svc("svc_db01", "provisioning")); err != nil {
		t.Fatal(err)
	}
	if len(a.live) == 0 {
		t.Fatal("nothing was applied — this test would prove nothing")
	}

	// Now the row's shape names a size this binary does not know: a newer catalog
	// against an older agent.
	del := svc("svc_db01", "deleting")
	del.Desired["shape"] = map[string]any{"size": "performance-2"}

	status, err := r.Converge(ctx, del)
	if err != nil {
		t.Fatalf("teardown refused a service whose size this binary does not know — the "+
			"cluster keeps running and the row cannot be edited out of `deleting`: %v", err)
	}
	if status != "gone" {
		t.Fatalf("teardown reported %q, want gone", status)
	}
	if len(a.deleted) == 0 {
		t.Fatal("teardown deleted nothing")
	}

	// And CREATE with that same size must still be refused — the fix must not
	// have turned the refusal off, only moved it off the teardown path.
	create := svc("svc_db02", "provisioning")
	create.Desired["shape"] = map[string]any{"size": "performance-2"}
	if _, err := r.Converge(ctx, create); err == nil {
		t.Fatal("create accepted an uncatalogued size — the refusal was removed, not relocated")
	}
}

// THE TEARDOWN COVERS EVERY OBJECT tenancy.Render PRODUCES.
//
// Not by listing them — by rendering the real set and checking each one is
// removed by SOMETHING. A namespaced object dies with the namespace; a
// cluster-scoped one must appear in TeardownObjects and be deleted by name.
//
// This is the invariant that survives US-3.3c widening the object set: whatever
// it adds is covered automatically if it is namespaced, and fails here loudly if
// it is cluster-scoped and not torn down. A hardcoded list would silently stop
// covering it, which is the same defect envObjectKeys exists to avoid.
func TestTeardownCoversEveryObjectTenancyRenders(t *testing.T) {
	const ns = testNamespace
	all, err := tenancy.Render(tenancy.Spec{Namespace: ns, Cell: "cell-0",
		Quota: tenancy.Quota{CPU: "8", Memory: "16Gi", Storage: "100Gi"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 2 {
		t.Fatalf("tenancy.Render produced %d objects — this test would prove almost nothing", len(all))
	}
	torn, err := tenancy.TeardownObjects(ns)
	if err != nil {
		t.Fatal(err)
	}
	tornByKind := map[string]string{}
	for _, m := range torn {
		tornByKind[m.Kind] = m.Name
	}

	sawClusterScoped := 0
	for _, m := range all {
		// The oracle is kube's OWN scope table, not a re-implementation of the
		// production classifier and not a string match. `strings.Contains(YAML,
		// "namespace: "+ns)` agreed with the parser on today's objects and
		// disagrees on realistic future ones — a quoted namespace, or a
		// ClusterRoleBinding whose `subjects[].namespace` is not its own scope —
		// so it would have failed with a message naming the wrong cause.
		declared := !kube.IsClusterScoped(m.Kind)
		if declared {
			// Namespaced: removed by the namespace. It must NOT also be in the
			// teardown set — deleting it by name is redundant, and doing so
			// would mean the teardown depends on enumerating them correctly.
			if _, ok := tornByKind[m.Kind]; ok {
				t.Errorf("%s/%s is namespaced and is ALSO deleted explicitly — the namespace "+
					"already removes it, and an explicit list is what goes stale", m.Kind, m.Name)
			}
			continue
		}
		sawClusterScoped++
		name, ok := tornByKind[m.Kind]
		if !ok {
			t.Errorf("%s/%s is CLUSTER-SCOPED and nothing tears it down — it outlives the "+
				"environment, which is exactly the leak US-3.3b closes", m.Kind, m.Name)
			continue
		}
		if name != m.Name {
			t.Errorf("%s: teardown deletes %q but Render creates %q", m.Kind, name, m.Name)
		}
	}
	if sawClusterScoped == 0 {
		t.Fatal("no cluster-scoped object in tenancy.Render's output — the namespace itself " +
			"should be one, so this test is not measuring what it claims")
	}
	if len(torn) != sawClusterScoped {
		t.Errorf("teardown deletes %d objects but only %d of Render's are cluster-scoped",
			len(torn), sawClusterScoped)
	}
}

// A TEARDOWN REMOVES ONE ENVIRONMENT'S NAMESPACE AND NO OTHER — the AC's
// "proven with two environments present, not one".
func TestEnvironmentTeardownRemovesOnlyItsOwnNamespace(t *testing.T) {
	const nsA = "env-0123456789abcdef0123456789abcdef"
	const nsB = "env-fedcba9876543210fedcba9876543210"
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)

	// Both environments exist, each with its own namespace-scoped objects.
	for _, ns := range []string{nsA, nsB} {
		objs, err := tenancy.Render(tenancy.Spec{Namespace: ns, Cell: "cell-0",
			Quota: tenancy.Quota{CPU: "8", Memory: "16Gi", Storage: "100Gi"}})
		if err != nil {
			t.Fatal(err)
		}
		var raw [][]byte
		for _, o := range objs {
			raw = append(raw, o.YAML)
		}
		if err := a.Apply(context.Background(), ns, raw); err != nil {
			t.Fatal(err)
		}
	}
	before := len(a.live)

	if err := r.TeardownEnvironment(context.Background(), nsA); err != nil {
		t.Fatalf("teardown: %v", err)
	}

	// A's namespace object is gone...
	if a.live[nsA+"/"+nsA] {
		t.Error("the environment's own Namespace survived its teardown")
	}
	// ...and B is untouched, every object of it. (A's namespaced objects are
	// still in the fake: an API server cascades them with the namespace, this
	// fake does not, and simulating that would be testing Kubernetes rather than
	// the teardown. What is asserted here is which object we DELETE.)
	for _, o := range mustRenderEnvObjects(t, nsB) {
		if !a.live[nsB+"/"+o.Name] {
			t.Errorf("tearing down %s removed %s/%s from the OTHER environment",
				nsA, nsB, o.Name)
		}
	}
	if len(a.live) >= before {
		t.Error("the teardown deleted nothing at all")
	}
}

func mustRenderEnvObjects(t *testing.T, ns string) []tenancy.Manifest {
	t.Helper()
	objs, err := tenancy.Render(tenancy.Spec{Namespace: ns, Cell: "cell-0",
		Quota: tenancy.Quota{CPU: "8", Memory: "16Gi", Storage: "100Gi"}})
	if err != nil {
		t.Fatal(err)
	}
	return objs
}

// A NAMESPACE THE CONTROL PLANE DID NOT RESOLVE IS REFUSED, NOT DELETED.
//
// The namespace arrives over the wire and is interpolated into a request path. A
// teardown is the one operation where a wrong-but-plausible value is
// unrecoverable, so it is validated before anything is deleted.
func TestEnvironmentTeardownRefusesAnInvalidNamespace(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	for _, ns := range []string{"", "Env-Upper", "kube-system/../default", "a b", strings.Repeat("x", 300)} {
		if err := r.TeardownEnvironment(context.Background(), ns); err == nil {
			t.Errorf("teardown accepted the namespace %q", ns)
		}
		if len(a.deleted) != 0 {
			t.Fatalf("teardown of %q deleted %v before validating", ns, a.deleted)
		}
	}
}

// A DELETE THAT WAS ACCEPTED IS NOT A WORKLOAD THAT IS GONE.
//
// Kubernetes answers 2xx the MOMENT it accepts a delete — finalizers and
// graceful termination still pending, and a CNPG Cluster has both. The teardown
// used to report `gone` on the strength of that 2xx, which reports acceptance as
// completion. Everything downstream reads `gone` as absence: US-3.3h converges
// `deleting` + `gone`, and US-3.3b then advertises the environment's NAMESPACE
// for deletion — removing a database whose pods are still terminating.
//
// So the teardown observes the Cluster absent first. Here the object survives
// the delete (a finalizer, modelled by re-adding it), and the converge must NOT
// report gone.
func TestATeardownDoesNotReportGoneWhileTheClusterSurvives(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	s := svc("svc_0123456789abcdef0123456789abcdef", "provisioning")
	if _, err := r.Converge(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	// The delete is accepted, but a finalizer keeps the object alive: whatever
	// Delete removes, it is still there when we look.
	a.mu.Lock()
	live := map[string]bool{}
	for k, v := range a.live {
		live[k] = v
	}
	// onDelete runs INSIDE Delete, which already holds f.mu — so it must not
	// take the lock again.
	a.onDelete = func() { a.live = live }
	a.mu.Unlock()

	status, err := r.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "deleting"))
	if status == "gone" {
		t.Fatal("reported `gone` while the Cluster is still present — the control plane would " +
			"converge the service and then delete the whole namespace out from under a " +
			"terminating database")
	}
	if !errors.Is(err, agent.ErrNotConverged) {
		t.Fatalf("err = %v, want ErrNotConverged — the teardown landed and is not finished", err)
	}
}

// ...and once it IS absent, the same converge reports gone.
func TestATeardownReportsGoneOnceTheClusterIsAbsent(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	r := newRenderer(a)
	if _, err := r.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "provisioning")); err != nil {
		t.Fatal(err)
	}
	status, err := r.Converge(context.Background(), svc("svc_0123456789abcdef0123456789abcdef", "deleting"))
	if err != nil {
		t.Fatalf("teardown: %v", err)
	}
	if status != "gone" {
		t.Fatalf("status = %q, want gone once the Cluster is actually absent", status)
	}
}

// A DELETE THE API SERVER REFUSES IS NOT A TEARDOWN.
//
// The failure was injected at the fake RENDERER (TestAFailedEnvironmentTeardownIsNotConfirmed);
// the APPLIER representation — a 403/409/500 from the API server — was uncovered,
// and swallowing that error left the whole module green. The renderer would
// return nil, the loop would confirm, the control plane would stamp
// torn_down_at and stop advertising, and the namespace would leak forever with
// the control plane believing it gone.
func TestATeardownWhoseDeleteIsRefusedIsNotReportedDone(t *testing.T) {
	a := newFakeApplier("Cluster in healthy state")
	a.deleteErr = errors.New("403 forbidden")
	r := newRenderer(a)
	if err := r.TeardownEnvironment(context.Background(), testNamespace); err == nil {
		t.Fatal("a refused Delete was reported as a successful teardown — the control plane " +
			"would stamp torn_down_at and never ask again")
	}
}
