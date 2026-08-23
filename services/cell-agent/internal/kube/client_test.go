package kube

import (
	"bytes"
	"context"
	"fmt"
	"github.com/steloit/cloud/services/cell-agent/internal/driver"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/cnpg"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/tenancy"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These prove the exact HTTP contract the real cluster will see — method, path,
// content-type, field manager, and the body being the driver's YAML verbatim —
// without needing a cluster. A wrong path or content-type is a live-apply
// failure that is expensive to find on a cluster and free to catch here.

const clusterYAML = `apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: svc-db01
  namespace: acme--prod
spec:
  instances: 1
`

const backupYAML = `apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: svc-db01-nightly
  namespace: acme--prod
spec:
  immediate: true
`

type capture struct {
	method, path, contentType, auth, query string
	body                                   string
}

func serverCapturing(t *testing.T, status int, respBody string, got *[]capture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// io.ReadAll, not one Read: a single Read is not guaranteed to fill the
		// buffer, and TestApplyUsesServerSideApplyContract compares the captured
		// body for exact equality. The sweep now reads this field hundreds of
		// times per run.
		buf, _ := io.ReadAll(r.Body)
		*got = append(*got, capture{
			method: r.Method, path: r.URL.Path, contentType: r.Header.Get("Content-Type"),
			auth: r.Header.Get("Authorization"), query: r.URL.RawQuery, body: string(buf),
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestApplyUsesServerSideApplyContract(t *testing.T) {
	var got []capture
	srv := serverCapturing(t, 200, `{}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())

	if err := c.Apply(context.Background(), "acme--prod", [][]byte{[]byte(clusterYAML), []byte(backupYAML)}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 applies, got %d", len(got))
	}
	c0 := got[0]
	if c0.method != http.MethodPatch {
		t.Fatalf("SSA must be PATCH, got %s", c0.method)
	}
	if c0.contentType != "application/apply-patch+yaml" {
		t.Fatalf("SSA content-type wrong: %q", c0.contentType)
	}
	// CNPG is a CRD → /apis/<group>/<version>/namespaces/<ns>/clusters/<name>
	if c0.path != "/apis/postgresql.cnpg.io/v1/namespaces/acme--prod/clusters/svc-db01" {
		t.Fatalf("cluster apply path wrong: %s", c0.path)
	}
	if !strings.Contains(c0.query, "fieldManager=steloit-cell-agent") || !strings.Contains(c0.query, "force=true") {
		t.Fatalf("SSA needs a field manager and force (level-triggered ownership): %q", c0.query)
	}
	if c0.auth != "Bearer tok" {
		t.Fatalf("service-account bearer not sent: %q", c0.auth)
	}
	// The body is the driver's YAML verbatim — never re-marshalled, so what the
	// cluster receives is exactly what T3.4's goldens pinned.
	if c0.body != clusterYAML {
		t.Fatalf("apply body was modified in flight:\n%s", c0.body)
	}
	if got[1].path != "/apis/postgresql.cnpg.io/v1/namespaces/acme--prod/scheduledbackups/svc-db01-nightly" {
		t.Fatalf("scheduledbackup path wrong: %s", got[1].path)
	}
}

func TestObserveReadsClusterPhase(t *testing.T) {
	var got []capture
	srv := serverCapturing(t, 200, `{"status":{"phase":"Cluster in healthy state"}}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())

	phase, err := c.Observe(context.Background(), "acme--prod", "svc-db01")
	if err != nil {
		t.Fatal(err)
	}
	if phase != "Cluster in healthy state" {
		t.Fatalf("phase wrong: %q", phase)
	}
	if got[0].method != http.MethodGet || !strings.HasSuffix(got[0].path, "/clusters/svc-db01") {
		t.Fatalf("observe request wrong: %s %s", got[0].method, got[0].path)
	}
}

// A cluster that does not exist yet is a NORMAL convergence state, not an error:
// Observe returns "" and the renderer maps it to provisioning.
func TestObserveNotFoundIsEmptyNotError(t *testing.T) {
	var got []capture
	srv := serverCapturing(t, 404, `{"kind":"Status","code":404}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())

	phase, err := c.Observe(context.Background(), "acme--prod", "svc-db01")
	if err != nil {
		t.Fatalf("a 404 must not be an error (the cluster is simply not created yet): %v", err)
	}
	if phase != "" {
		t.Fatalf("404 must yield an empty phase, got %q", phase)
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	var got []capture
	srv := serverCapturing(t, 404, `{"code":404}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())
	if err := c.Delete(context.Background(), "acme--prod", "Cluster", "svc-db01"); err != nil {
		t.Fatalf("deleting an already-absent cluster must succeed (idempotent teardown): %v", err)
	}
	if got[0].method != http.MethodDelete {
		t.Fatalf("delete must be DELETE, got %s", got[0].method)
	}
}

func TestApplyErrorSurfacesStatusAndBody(t *testing.T) {
	var got []capture
	srv := serverCapturing(t, 422, `{"message":"invalid spec"}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())
	err := c.Apply(context.Background(), "acme--prod", [][]byte{[]byte(clusterYAML)})
	if err == nil {
		t.Fatal("a rejected apply must surface as an error")
	}
	if !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "invalid spec") {
		t.Fatalf("the error must carry the API's reason for a diagnosable log: %v", err)
	}
}

func TestUnknownKindIsRefusedNotGuessed(t *testing.T) {
	c := NewClientForTest("http://x", "t", http.DefaultClient)
	bad := []byte("apiVersion: v1\nkind: Wibble\nmetadata:\n  name: n\n  namespace: ns\n")
	err := c.Apply(context.Background(), "ns", [][]byte{bad})
	if err == nil || !strings.Contains(err.Error(), "unknown kind") {
		t.Fatalf("an unknown kind must be refused, not pluralized by guess: %v", err)
	}
}

func TestCorePathVsCRDPath(t *testing.T) {
	// core group ("v1") → /api/v1/...; CRD group → /apis/<group>/<version>/...
	got, err := resourcePath("v1", "Secret", "ns", "s")
	if err != nil || got != "/api/v1/namespaces/ns/secrets/s" {
		t.Fatalf("core path wrong: %q %v", got, err)
	}
	got, err = resourcePath("postgresql.cnpg.io/v1", "Cluster", "ns", "c")
	if err != nil || got != "/apis/postgresql.cnpg.io/v1/namespaces/ns/clusters/c" {
		t.Fatalf("CRD path wrong: %q %v", got, err)
	}
}

func TestNewInClusterFailsOutsideCluster(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	if _, err := NewInCluster(); err == nil {
		t.Fatal("outside a cluster NewInCluster must error so main can fall back visibly")
	}
}

// A delete must route by KIND: sending a ScheduledBackup's name to /clusters/
// 404s and silently orphans it (review round 2 blocker).
func TestDeleteRoutesByKind(t *testing.T) {
	var got []capture
	srv := serverCapturing(t, 200, `{}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())
	if err := c.Delete(context.Background(), "acme--prod", "ScheduledBackup", "svc-db01-nightly"); err != nil {
		t.Fatal(err)
	}
	want := "/apis/postgresql.cnpg.io/v1/namespaces/acme--prod/scheduledbackups/svc-db01-nightly"
	if got[0].path != want {
		t.Fatalf("ScheduledBackup delete routed to %q, want %q (a /clusters/ path 404s and orphans it)", got[0].path, want)
	}
}

// A Namespace is CLUSTER-SCOPED: it lives at /api/v1/namespaces/<name>, not
// nested under /namespaces/<ns>/. Nesting it is a 404 at apply time on a live
// cluster — the same class the plural map exists to prevent, one level up.
// US-3.3a needs this because the agent creates the env namespace itself, and a
// namespace has no namespace.
func TestClusterScopedKindsGetAClusterScopedPath(t *testing.T) {
	got, err := resourcePath("v1", "Namespace", "", "env-9f3c1a2b")
	if err != nil {
		t.Fatalf("a Namespace with no namespace must be routable: %v", err)
	}
	if want := "/api/v1/namespaces/env-9f3c1a2b"; got != want {
		t.Fatalf("Namespace path = %q, want %q", got, want)
	}
	// Even when a namespace IS supplied (the applier passes the env namespace for
	// the whole batch), a cluster-scoped kind must ignore it rather than nest.
	got, err = resourcePath("v1", "Namespace", "env-9f3c1a2b", "env-9f3c1a2b")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/api/v1/namespaces/env-9f3c1a2b"; got != want {
		t.Fatalf("Namespace path with a namespace arg = %q, want %q — it must not nest", got, want)
	}
}

// The negative half: a NAMESPACED kind with no namespace must be refused, not
// silently routed to a cluster-scoped path where it would apply to the wrong
// place or 404.
func TestNamespacedKindsStillRequireANamespace(t *testing.T) {
	if _, err := resourcePath("v1", "Secret", "", "creds"); err == nil {
		t.Fatal("a namespaced kind with no namespace was accepted — it would apply to the wrong path")
	}
}

// The four D7 kinds must all be routable; an unknown kind is a 404 at apply time.

// Apply routes by the CALLER's namespace argument. A manifest declaring a
// DIFFERENT namespace was therefore written into the caller's, and the only
// thing stopping it was the API server answering 400 — a tenant boundary
// enforced a network hop away, on a code path nothing pinned. It must be refused
// here, before the request is built.
func TestApplyRefusesAManifestBelongingToAnotherNamespace(t *testing.T) {
	foreign := []byte(`apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db
  namespace: env-victim
`)
	var got []capture
	srv := serverCapturing(t, 200, `{}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())

	err := c.Apply(context.Background(), "env-mine", [][]byte{foreign})
	if err == nil {
		t.Fatal("Apply accepted a manifest declaring env-victim into env-mine")
	}
	if !strings.Contains(err.Error(), "env-victim") || !strings.Contains(err.Error(), "env-mine") {
		t.Fatalf("the error must name both namespaces so the operator can see the mismatch: %v", err)
	}
	// Refused BEFORE the request, not after a 400: a fake server that answers
	// 200 to everything must never have been asked.
	if len(got) != 0 {
		t.Fatalf("the foreign manifest was sent to the API server anyway: %+v", got)
	}
}

// The matching-namespace and the omitted-namespace cases must still apply, or
// the guard above would be satisfied by an Apply that refuses everything.
func TestApplyStillAcceptsMatchingAndUnqualifiedManifests(t *testing.T) {
	matching := []byte(`apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db
  namespace: env-mine
`)
	unqualified := []byte(`apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db2
`)
	var got []capture
	srv := serverCapturing(t, 200, `{}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())

	if err := c.Apply(context.Background(), "env-mine", [][]byte{matching, unqualified}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected both to be applied, got %d", len(got))
	}
	for _, g := range got {
		if !strings.Contains(g.path, "/namespaces/env-mine/") {
			t.Fatalf("applied to %q, want the caller's namespace", g.path)
		}
	}
}

// A cluster-scoped object carrying a namespace is a renderer bug in the other
// direction: resourcePath ignores the namespace for these kinds, so the
// declaration would be silently dropped rather than honoured.
func TestApplyRefusesAClusterScopedObjectThatDeclaresANamespace(t *testing.T) {
	confused := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: env-mine
  namespace: env-somewhere
`)
	var got []capture
	srv := serverCapturing(t, 200, `{}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())

	if err := c.Apply(context.Background(), "env-mine", [][]byte{confused}); err == nil {
		t.Fatal("Apply accepted a cluster-scoped object declaring a namespace")
	} else if !strings.Contains(err.Error(), "cluster-scoped") {
		t.Fatalf("unhelpful error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("it was sent anyway: %+v", got)
	}
}

// Widening `plurals` without widening its consumer converts a LOUD error into a
// SILENT success. Delete hardcoded apiVersion "postgresql.cnpg.io/v1"; adding a
// kind to `plurals` was therefore enough to make Delete build a plausible path
// under the wrong API group, receive a 404, and map it to "already gone".
//
// Measured on this branch before the fix:
//
//	Delete(…, "Namespace", "obj") -> /apis/postgresql.cnpg.io/v1/namespaces/obj, err=<nil>
//
// while origin/main refused it by name. US-3.3b is the task that will call
// Delete(ns, "Namespace", ns); it would have reported success while the
// namespace and everything in it survived.
func TestDeleteRoutesEveryKindToItsOwnAPIGroup(t *testing.T) {
	for kind, want := range map[string]string{
		"Cluster":         "/apis/postgresql.cnpg.io/v1/namespaces/env-x/clusters/obj",
		"ScheduledBackup": "/apis/postgresql.cnpg.io/v1/namespaces/env-x/scheduledbackups/obj",
		"VolumeSnapshot":  "/apis/snapshot.storage.k8s.io/v1/namespaces/env-x/volumesnapshots/obj",
		"Secret":          "/api/v1/namespaces/env-x/secrets/obj",
		"StatefulSet":     "/apis/apps/v1/namespaces/env-x/statefulsets/obj",
		"Namespace":       "/api/v1/namespaces/obj",
	} {
		var seen string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r.URL.Path
			w.WriteHeader(200)
		}))
		c := NewClientForTest(srv.URL, "tok", srv.Client())
		if err := c.Delete(context.Background(), "env-x", kind, "obj"); err != nil {
			t.Errorf("Delete %s: %v", kind, err)
		} else if seen != want {
			t.Errorf("Delete %s routed to %q, want %q", kind, seen, want)
		}
		srv.Close()
	}
}

// The other direction: a kind Delete has no apiVersion for must be REFUSED, not
// routed under a guess. Without this, adding a kind to `plurals` silently
// re-opens the hole above.
func TestDeleteRefusesAKindItCannotAddress(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(404)
	}))
	defer srv.Close()
	c := NewClientForTest(srv.URL, "tok", srv.Client())

	for _, kind := range []string{"NetworkPolicy", "ResourceQuota", "LimitRange", "Ingress"} {
		err := c.Delete(context.Background(), "env-x", kind, "obj")
		if err == nil {
			t.Errorf("Delete accepted %s — a 404 from a wrong path reads as 'already gone'", kind)
		}
	}
	if called {
		t.Fatal("a request was sent for a kind Delete cannot address")
	}
}

// A manifest is one object. yaml.Unmarshal returns only document 1 of a
// multi-document stream, so the kind we route on, the name we address and the
// namespace we compare all describe the first object while the WHOLE body is
// PATCHed — a second document could carry any kind into any namespace with the
// suite green.
func TestApplyRefusesAMultiDocumentManifest(t *testing.T) {
	multi := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: env-mine
---
apiVersion: v1
kind: Secret
metadata:
  name: stolen
  namespace: env-victim
`)
	var got []capture
	srv := serverCapturing(t, 200, `{}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())

	err := c.Apply(context.Background(), "env-mine", [][]byte{multi})
	if err == nil {
		t.Fatal("Apply accepted a 2-document manifest; the cross-namespace guard read document 1 only")
	}
	if !strings.Contains(err.Error(), "2 YAML documents") {
		t.Fatalf("unhelpful error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("it was sent anyway: %+v", got)
	}
}

// Trailing separators and an empty trailing document are ordinary YAML and must
// still apply, or the guard above would be satisfied by an Apply that refuses
// anything with a "---" in it.
func TestApplyStillAcceptsOneDocumentWithSeparators(t *testing.T) {
	withSep := []byte(`---
apiVersion: v1
kind: Namespace
metadata:
  name: env-mine
---
`)
	var got []capture
	srv := serverCapturing(t, 200, `{}`, &got)
	c := NewClientForTest(srv.URL, "tok", srv.Client())

	if err := c.Apply(context.Background(), "env-mine", [][]byte{withSep}); err != nil {
		t.Fatalf("a single document with separators was refused: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 apply, got %d", len(got))
	}
}

// EVERY manifest, at EVERY index, at ANY batch length, with PRODUCTION BYTES.
//
// The history of this test is the finding, and it is one class relocated five
// times: restricting the guards to `manifests[0]` was green, so the test drove
// two; `_mi < 2` was green, so it drove four; `_mi < 4` was green, so it swept to
// 16; and a skip keyed on the Namespace at index 0 — every production batch — was
// green, so the kinds were varied. Then a skip keyed on `len(m) > 200`, or on the
// manifest containing "\nspec:", was STILL green, because every fixture was a
// hand-written four-line metadata-only stub: no labels, no spec, 55–140 bytes.
// Five hand-written approximations of a batch this repo can render for real.
//
// So the fixtures ARE the real thing. tenancy.Render and cnpg.Driver.Render
// produce exactly what Converge applies — a ~133B Namespace with labels, an ~800B
// Cluster with a nested spec, a ~250B ScheduledBackup — and the offenders are
// those same manifests with their namespace rewritten or a document appended.
// A guard keyed on any property of the bytes now has nowhere to hide, because the
// bytes are the ones production sends.
func TestApplyGuardsEveryManifestAtEveryIndexAtAnyLength(t *testing.T) {
	const maxLen = 12

	real := productionManifests(t)
	if len(real) < 3 {
		t.Fatalf("expected the real renderers to produce at least 3 manifests, got %d", len(real))
	}
	// The sweep must cover bodies of materially different shape, or "production
	// bytes" is one byte-length again.
	var shortest, longest int = 1 << 30, 0
	for _, m := range real {
		if len(m) < shortest {
			shortest = len(m)
		}
		if len(m) > longest {
			longest = len(m)
		}
	}
	if longest < shortest*2 {
		t.Fatalf("the fixtures span %d..%d bytes — too uniform to catch a length-keyed guard", shortest, longest)
	}

	filler := func(i int) []byte { return real[i%len(real)] }

	offenders := map[string]struct {
		yaml  []byte
		names string
	}{}
	for idx, base := range real {
		kind := yamlKind(t, base)
		// Same object, foreign namespace. Built by rewriting the REAL manifest so
		// it keeps its labels, spec and size.
		if bytes.Contains(base, []byte("namespace: "+testNS)) {
			offenders[kind+" in another namespace"] = struct {
				yaml  []byte
				names string
			}{bytes.Replace(base, []byte("namespace: "+testNS), []byte("namespace: env-victim"), 1), "env-victim"}
		}
		// Same object plus a smuggled second document.
		smuggled := append(append([]byte{}, base...),
			[]byte("---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: smuggled\n  namespace: env-victim\n")...)
		offenders[kind+" carrying two documents"] = struct {
			yaml  []byte
			names string
		}{smuggled, "2 YAML documents"}
		_ = idx
	}
	// A kind that is in plurals/apiVersions but that NOTHING renders. Every
	// offender above is derived from the renderers, so the sweep could only ever
	// pin the guards for kinds that exist today — and restricting the namespace
	// fence to the CNPG API group, or to Cluster/ScheduledBackup, was green.
	// US-3.3b and US-3.3c both widen those maps.
	offenders["an unrendered but addressable kind in another namespace"] = struct {
		yaml  []byte
		names string
	}{[]byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\n  namespace: env-victim\n"), "env-victim"}
	offenders["an unrendered but addressable kind carrying two documents"] = struct {
		yaml  []byte
		names string
	}{[]byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\n  namespace: " + testNS +
		"\n---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: smuggled\n  namespace: env-victim\n"), "2 YAML documents"}

	// A cluster-scoped object that names a namespace — the third guard arm.
	offenders["a cluster-scoped object declaring a namespace"] = struct {
		yaml  []byte
		names string
	}{[]byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: " + testNS + "\n  namespace: env-somewhere\n"), "cluster-scoped"}

	if len(offenders) < 4 {
		t.Fatalf("only %d offenders derived from the real manifests", len(offenders))
	}

	for label, off := range offenders {
		for n := 1; n <= maxLen; n++ {
			for idx := 0; idx < n; idx++ {
				t.Run(fmt.Sprintf("%s/len%d/at%d", label, n, idx), func(t *testing.T) {
					manifests := make([][]byte, n)
					for i := range manifests {
						manifests[i] = filler(i)
					}
					manifests[idx] = off.yaml

					var got []capture
					srv := serverCapturing(t, 200, `{}`, &got)
					c := NewClientForTest(srv.URL, "tok", srv.Client())

					err := c.Apply(context.Background(), testNS, manifests)
					if err == nil {
						t.Fatalf("Apply accepted an offending manifest at index %d of %d", idx, n)
					}
					if !strings.Contains(err.Error(), off.names) {
						t.Fatalf("the error must name the defect at index %d: %v", idx, err)
					}
					// ABORT, do not merely refuse. Namespace-first ordering is
					// load-bearing, so continuing past a refused manifest would
					// apply everything behind it.
					if len(got) != idx {
						t.Fatalf("Apply sent %d manifests, want %d — it did not abort at the offender", len(got), idx)
					}
					for k, g := range got {
						if strings.Contains(g.body, "env-victim") || strings.Contains(g.body, "smuggled") ||
							strings.Contains(g.body, "env-somewhere") {
							t.Fatalf("the offending manifest at index %d was sent: %s", idx, g.body)
						}
						assertSSAContract(t, g, manifests[k], k)
					}
				})
			}
		}
	}

	// Positive control at every length: the real batch must apply untouched, or
	// every case above is satisfied by an Apply that refuses things.
	for n := 1; n <= maxLen; n++ {
		var got []capture
		srv := serverCapturing(t, 200, `{}`, &got)
		c := NewClientForTest(srv.URL, "tok", srv.Client())
		clean := make([][]byte, n)
		for i := range clean {
			clean[i] = filler(i)
		}
		if err := c.Apply(context.Background(), testNS, clean); err != nil {
			t.Fatalf("a legitimate %d-manifest apply was refused: %v", n, err)
		}
		if len(got) != n {
			t.Fatalf("length %d: expected %d applies, got %d", n, n, len(got))
		}
		for k, g := range got {
			assertSSAContract(t, g, clean[k], k)
		}
		srv.Close()
	}
}

const testNS = "env-9f3c1a2b"

// productionManifests is what CNPGRenderer.Converge actually applies: the
// environment's Namespace, then the service's Cluster and ScheduledBackup.
// DERIVED from the renderers, so a manifest that changes shape changes these
// fixtures too.
func productionManifests(t *testing.T) [][]byte {
	t.Helper()
	out := [][]byte{}
	tm, err := tenancy.Render(tenancy.Spec{Namespace: testNS, Cell: "cell-0"})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range tm {
		out = append(out, m.YAML)
	}
	sm, err := cnpg.New().Render(driver.Spec{
		Name: "svc_db01", Namespace: testNS, Product: "postgres",
		Shape: map[string]any{"size": "dev"}, Instances: 1, Cell: "cell-0",
		GSAEmail: "sa@steloit-dev.iam.gserviceaccount.com", WALBucket: "steloit-dev-wal-customer",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range sm {
		out = append(out, m.YAML)
	}
	return out
}

func yamlKind(t *testing.T, m []byte) string {
	t.Helper()
	var doc struct {
		Kind string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(m, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Kind == "" {
		t.Fatalf("fixture has no kind:\n%s", m)
	}
	return doc.Kind
}

// `plurals` says which kinds are addressable; `apiVersions` says under which
// group. They must name the SAME set, or the two drift and Delete either refuses
// something Apply can write or routes something under a guessed group.
//
// TestDeleteRefusesAKindItCannotAddress could not catch this: every kind it
// tries is missing from BOTH maps, so the refusal it observes comes from
// resourcePath, not from the apiVersions guard it is named for. Adding
// "NetworkPolicy" to `plurals` alone — literally what US-3.3c will do — stayed
// green. That is the round-3 lesson recurring inside the round-3 fix.
func TestPluralsAndAPIVersionsNameTheSameKinds(t *testing.T) {
	for kind := range plurals {
		if _, ok := apiVersions[kind]; !ok {
			t.Errorf("%q is in plurals but not apiVersions — Delete would refuse a kind "+
				"Apply can write, or (if the refusal is ever softened) route it under a guess", kind)
		}
	}
	for kind := range apiVersions {
		if _, ok := plurals[kind]; !ok {
			t.Errorf("%q is in apiVersions but not plurals — resourcePath cannot address it", kind)
		}
	}
}

// The apiVersions guard must fire on its own, not be answered by resourcePath.
// Driven with a kind present in `plurals` and absent from `apiVersions`, which is
// the only state in which the guard is the thing that speaks.
func TestDeleteRefusesAKindThatIsAddressableButHasNoAPIVersion(t *testing.T) {
	// NO package-state mutation. An earlier version wrote plurals["Widget"] and
	// deleted it with defer, which was safe only while nothing in this package
	// called t.Parallel() — and -race would not have warned first. It is also
	// unnecessary: Delete looks up apiVersions BEFORE resourcePath, so a kind
	// absent from both maps is answered by the guard this test names, not by the
	// path builder. TestPluralsAndAPIVersionsNameTheSameKinds covers the drift.
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c := NewClientForTest(srv.URL, "tok", srv.Client())

	err := c.Delete(context.Background(), "env-x", "Widget", "obj")
	if err == nil {
		t.Fatal("Delete routed a kind with no apiVersion — a 404 from a guessed group reads as 'already gone'")
	}
	if !strings.Contains(err.Error(), "apiVersion") {
		t.Fatalf("the error must name the missing apiVersion: %v", err)
	}
	if called {
		t.Fatal("a request was sent")
	}
}

// THE TOKEN RE-READ. GKE projected ServiceAccount tokens expire (~1h) and are
// rotated IN PLACE in the file, so caching the value at boot means every apply
// 401s after the TTL and never recovers without a restart. The code says so; no
// test said so, and deleting the re-read was a green change — an unreported
// survivor, which is indistinguishable from one nobody looked for.
func TestAuthRereadsTheRotatedServiceAccountToken(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte("first-token"), 0o600); err != nil {
		t.Fatal(err)
	}

	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClientForTest(srv.URL, "", srv.Client())
	c.tokenFile = tokenPath

	manifest := []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: " + testNS + "\n")
	if err := c.Apply(context.Background(), testNS, [][]byte{manifest}); err != nil {
		t.Fatal(err)
	}
	// Rotated in place, exactly as the kubelet does it.
	if err := os.WriteFile(tokenPath, []byte("rotated-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.Apply(context.Background(), testNS, [][]byte{manifest}); err != nil {
		t.Fatal(err)
	}

	if len(seen) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(seen))
	}
	if seen[0] != "Bearer first-token" {
		t.Fatalf("first request sent %q", seen[0])
	}
	if seen[1] != "Bearer rotated-token" {
		t.Fatalf("the rotated token was not picked up: second request sent %q — every apply "+
			"401s once the projected token expires, and never recovers without a restart", seen[1])
	}
}

// assertSSAContract pins the request shape for EVERY manifest in a batch.
//
// TestApplyUsesServerSideApplyContract asserts it on got[0] and one path on
// got[1]. Converge applies [Namespace, Cluster, ScheduledBackup], so the only
// object whose contract was pinned is the one the agent writes itself — the
// customer's database was unpinned, and six restrictions to `_mi == 0` were
// green:
//
//   - re-marshalling the body for _mi > 0 (what CNPG receives changes while the
//     driver's goldens still pass, because the driver still renders correctly)
//   - dropping force=true for _mi > 0 — the SILENT one: the Cluster and
//     ScheduledBackup stop being force-owned, so a manual `kubectl edit` is never
//     corrected on the next converge and nothing errors. Level-triggered
//     convergence survives for the Namespace only.
//   - dropping fieldManager, reverting to merge-patch, skipping auth, and
//     honouring resourcePath's error only at index 0 (fail-loud, but still holes)
func assertSSAContract(t *testing.T, g capture, sent []byte, idx int) {
	t.Helper()
	if g.method != http.MethodPatch {
		t.Errorf("manifest %d: method %s, want PATCH", idx, g.method)
	}
	if g.contentType != "application/apply-patch+yaml" {
		t.Errorf("manifest %d: content-type %q — that is not a server-side apply", idx, g.contentType)
	}
	if g.auth != "Bearer tok" {
		t.Errorf("manifest %d: authorization %q", idx, g.auth)
	}
	if !strings.Contains(g.query, "fieldManager=steloit-cell-agent") {
		t.Errorf("manifest %d: no fieldManager in %q — SSA needs an owner", idx, g.query)
	}
	if !strings.Contains(g.query, "force=true") {
		t.Errorf("manifest %d: force=true missing from %q. Without it this object is not "+
			"force-owned, so a manual kubectl edit is never corrected on the next converge — "+
			"level-triggered convergence, silently lost for this object only", idx, g.query)
	}
	if g.path == "" || !strings.HasPrefix(g.path, "/api") {
		t.Errorf("manifest %d: path %q — resourcePath's error was not honoured", idx, g.path)
	}
	if g.body != string(sent) {
		t.Errorf("manifest %d: the body is not the driver's bytes verbatim.\n got: %s\nwant: %s",
			idx, g.body, sent)
	}
}
