package kube

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		buf := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			_, _ = r.Body.Read(buf)
		}
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
