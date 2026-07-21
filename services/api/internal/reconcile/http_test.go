package reconcile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mount(t *testing.T) (*httptest.Server, *fakeQ, *fakeTrans) {
	t.Helper()
	svc, q, tr := newFixture()
	mux := http.NewServeMux()
	NewHandlers(svc, NewAuth("s3cret", []string{"cell-0"})).Mount(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, q, tr
}

func call(t *testing.T, ts *httptest.Server, method, path, token, body string) (*http.Response, map[string]any) {
	t.Helper()
	var rd *strings.Reader
	if body == "" {
		rd = strings.NewReader("")
	} else {
		rd = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, ts.URL+path, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	var m map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return resp, m
}

func TestHTTPAuthLadder(t *testing.T) {
	ts, _, _ := mount(t)
	// no token → 401
	if r, _ := call(t, ts, "GET", "/v1/reconcile/cell-0/desired", "", ""); r.StatusCode != 401 {
		t.Fatalf("no token: want 401, got %d", r.StatusCode)
	}
	// wrong token → 401
	if r, _ := call(t, ts, "GET", "/v1/reconcile/cell-0/desired", "wrong", ""); r.StatusCode != 401 {
		t.Fatalf("wrong token: want 401, got %d", r.StatusCode)
	}
	// right token, foreign cell → 404 (never 403; no enumeration)
	if r, _ := call(t, ts, "GET", "/v1/reconcile/cell-9/desired", "s3cret", ""); r.StatusCode != 404 {
		t.Fatalf("foreign cell: want 404, got %d", r.StatusCode)
	}
	// right token, right cell → 200
	if r, _ := call(t, ts, "GET", "/v1/reconcile/cell-0/desired", "s3cret", ""); r.StatusCode != 200 {
		t.Fatalf("want 200, got %d", r.StatusCode)
	}
}

func TestHTTPUnconfiguredIs503(t *testing.T) {
	svc, _, _ := newFixture()
	mux := http.NewServeMux()
	NewHandlers(svc, NewAuth("", nil)).Mount(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	r, m := call(t, ts, "GET", "/v1/reconcile/cell-0/desired", "anything", "")
	if r.StatusCode != 503 {
		t.Fatalf("unconfigured must be 503 (visibly closed), got %d", r.StatusCode)
	}
	if m["remediation"] == "" {
		t.Fatal("problem+json must carry remediation")
	}
}

func TestHTTPDesiredShape(t *testing.T) {
	ts, _, _ := mount(t)
	r, m := call(t, ts, "GET", "/v1/reconcile/cell-0/desired?since_generation=3", "s3cret", "")
	if r.StatusCode != 200 {
		t.Fatalf("want 200, got %d", r.StatusCode)
	}
	svcs, ok := m["services"].([]any)
	if !ok || len(svcs) != 1 {
		t.Fatalf("since_generation=3 must return exactly svc_b, got %v", m["services"])
	}
	row := svcs[0].(map[string]any)
	if row["id"] != "svc_b" || row["generation"] != float64(7) {
		t.Fatalf("row shape wrong: %v", row)
	}
	if _, has := row["desired"]; !has {
		t.Fatal("the full desired doc must ride every row (level-triggered)")
	}
}

func TestHTTPBadQueryIs400(t *testing.T) {
	ts, _, _ := mount(t)
	if r, _ := call(t, ts, "GET", "/v1/reconcile/cell-0/desired?since_generation=x", "s3cret", ""); r.StatusCode != 422 {
		t.Fatalf("non-integer since_generation: want 422, got %d", r.StatusCode)
	}
	if r, _ := call(t, ts, "GET", "/v1/reconcile/cell-0/desired?limit=0", "s3cret", ""); r.StatusCode != 422 {
		t.Fatalf("limit=0: want 422, got %d", r.StatusCode)
	}
}

func TestHTTPStatusWriteback(t *testing.T) {
	ts, q, tr := mount(t)
	r, m := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_a","observed_generation":3,"status":"ready"}`)
	if r.StatusCode != 200 {
		t.Fatalf("want 200, got %d (%v)", r.StatusCode, m)
	}
	if m["status"] != "ready" || m["observed_generation"] != float64(3) {
		t.Fatalf("response shape wrong: %v", m)
	}
	if q.services["svc_a"].Status != "ready" || tr.calls != 1 {
		t.Fatal("writeback did not drive the machine exactly once")
	}
}

func TestHTTPStaleIs409WithRemediation(t *testing.T) {
	ts, _, _ := mount(t)
	r, m := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_a","observed_generation":9,"status":"ready"}`)
	if r.StatusCode != 409 {
		t.Fatalf("stale generation: want 409, got %d", r.StatusCode)
	}
	if rem, _ := m["remediation"].(string); !strings.Contains(rem, "Re-poll") {
		t.Fatalf("409 must tell the agent what to do next: %v", m)
	}
}

func TestHTTPValidation(t *testing.T) {
	ts, _, _ := mount(t)
	cases := []struct{ name, body string }{
		{"missing service_id", `{"observed_generation":1,"status":"ready"}`},
		{"missing observed_generation", `{"service_id":"svc_a","status":"ready"}`},
		{"negative generation", `{"service_id":"svc_a","observed_generation":-1}`},
		{"unknown status", `{"service_id":"svc_a","observed_generation":1,"status":"running"}`}, // ADR-024: never "running"
		{"unknown field", `{"service_id":"svc_a","observed_generation":1,"nope":true}`},
		{"garbage", `{`},
	}
	for _, c := range cases {
		if r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret", c.body); r.StatusCode != 422 {
			t.Fatalf("%s: want 422, got %d", c.name, r.StatusCode)
		}
	}
}

func TestHTTPGoneIsObservationOnly(t *testing.T) {
	ts, q, tr := mount(t)
	r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_a","observed_generation":3,"status":"gone"}`)
	if r.StatusCode != 200 {
		t.Fatalf("want 200, got %d", r.StatusCode)
	}
	if tr.calls != 0 {
		t.Fatal("gone must not drive the status machine — row removal is the deletion pipeline's job")
	}
	if q.services["svc_a"].ObservedGeneration != 3 {
		t.Fatal("gone must still record observation")
	}
}

func TestHTTPForeignServiceIs404(t *testing.T) {
	ts, _, _ := mount(t)
	r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_far","observed_generation":1,"status":"ready"}`)
	if r.StatusCode != 404 {
		t.Fatalf("foreign-cell service must be 404 (no probing), got %d", r.StatusCode)
	}
}

func TestHTTPGoneOnLiveServiceLeavesStatusUnchanged(t *testing.T) {
	ts, q, tr := mount(t)
	// svc_a is provisioning (a live, non-deleting service). An agent reporting
	// "gone" for it is a bug; the control plane must not mutate customer-visible
	// status off it (gone → observation-only).
	r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_a","observed_generation":3,"status":"gone"}`)
	if r.StatusCode != 200 {
		t.Fatalf("want 200, got %d", r.StatusCode)
	}
	if tr.calls != 0 {
		t.Fatal("gone must never drive the status machine")
	}
	if q.services["svc_a"].Status != "provisioning" {
		t.Fatalf("gone on a live service changed status to %q", q.services["svc_a"].Status)
	}
}
