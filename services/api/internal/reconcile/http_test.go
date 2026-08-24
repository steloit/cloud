package reconcile

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/steloit/cloud/services/api/internal/identity/store"
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

// `gone` NEVER DRIVES THE STATUS MACHINE — and on a live service it does not
// finish the generation either.
//
// svc_a is `provisioning`: a live, non-deleting service. An agent reporting
// `gone` for it means the workload VANISHED while desired still wants it alive.
// The status must not move (row removal is the deletion pipeline's job, US-3.5),
// but the row must STAY OUTSTANDING so the next converge re-creates it.
//
// This used to answer 200 and advance observed_generation, which dropped the row
// out of ListDesiredForCell permanently: the customer kept seeing `provisioning`
// for a service that did not exist, and nothing would ever put it back.
func TestHTTPGoneOnALiveServiceIs409AndLeavesTheRowOutstanding(t *testing.T) {
	ts, q, tr := mount(t)
	before := q.services["svc_a"].ObservedGeneration
	r, m := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_a","observed_generation":3,"status":"gone"}`)
	if r.StatusCode != 409 {
		t.Fatalf("want 409, got %d", r.StatusCode)
	}
	if tr.calls != 0 {
		t.Fatal("gone must never drive the status machine — row removal is the deletion pipeline's job")
	}
	if got := q.services["svc_a"].Status; got != "provisioning" {
		t.Fatalf("gone on a live service changed status to %q", got)
	}
	if got := q.services["svc_a"].ObservedGeneration; got != before {
		t.Fatalf("observed_generation advanced %d→%d: the row left the outstanding set and "+
			"nothing will ever re-create the vanished workload", before, got)
	}
	if rem, _ := m["remediation"].(string); !strings.Contains(rem, "Re-poll") {
		t.Fatalf("no actionable remediation on the refusal: %v", m)
	}
}

// ...but the teardown we ASKED for does finish, or a deleting row would stay
// outstanding forever and the agent would re-issue Delete every tick.
func TestHTTPGoneOnADeletingServiceConverges(t *testing.T) {
	ts, q, tr := mount(t)
	q.services["svc_a"] = store.Service{
		ID: "svc_a", CellID: "cell-0", Status: "deleting", Generation: 3, ObservedGeneration: 2,
	}
	r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_a","observed_generation":3,"status":"gone"}`)
	if r.StatusCode != 200 {
		t.Fatalf("want 200, got %d", r.StatusCode)
	}
	if tr.calls != 0 {
		t.Fatal("gone must never drive the status machine")
	}
	if got := q.services["svc_a"].Status; got != "deleting" {
		t.Fatalf("status moved to %q — `deleting` is terminal", got)
	}
	if got := q.services["svc_a"].ObservedGeneration; got != 3 {
		t.Fatalf("observed_generation = %d, want 3 — a completed teardown must finish the generation", got)
	}
}

// AN UNCONVERGED HOP IS A 409 ON THE ROUTE, not just in writeErr.
//
// The unit test below calls writeErr directly with a hand-built error, which
// cannot see routing, the auth gate, or the error Writeback actually produces —
// a handler that answered 200 for ErrNotConverged would leave it green. This
// drives the real path: a `failed` service reporting a healthy cluster routes
// through `provisioning` (ADR-024 has no failed → ready) and needs a second tick.
func TestHTTPUnconvergedHopIs409AndLeavesTheRowOutstanding(t *testing.T) {
	ts, q, _ := mount(t)
	q.services["svc_f"] = store.Service{
		ID: "svc_f", CellID: "cell-0", Status: "failed", Generation: 4, ObservedGeneration: 3,
	}
	r, m := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_f","observed_generation":4,"status":"ready"}`)
	if r.StatusCode != 409 {
		t.Fatalf("want 409, got %d — a 500 here is an undeclared response for an expected event", r.StatusCode)
	}
	if got := q.services["svc_f"].Status; got != "provisioning" {
		t.Fatalf("status = %q, want provisioning — the legal edge should still have been taken", got)
	}
	if got := q.services["svc_f"].ObservedGeneration; got == 4 {
		t.Fatal("observed_generation advanced on an unconverged hop: the row leaves the " +
			"outstanding set at `provisioning` and never reaches ready")
	}
	if rem, _ := m["remediation"].(string); !strings.Contains(rem, "Re-poll") || strings.Contains(rem, "contact support") {
		t.Fatalf("remediation is not the re-poll the agent needs: %v", m)
	}

	// The second tick finishes it, which is what makes leaving it outstanding correct.
	if r2, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_f","observed_generation":4,"status":"ready"}`); r2.StatusCode != 200 {
		t.Fatalf("second tick: want 200, got %d", r2.StatusCode)
	}
	if got := q.services["svc_f"].Status; got != "ready" {
		t.Fatalf("after the second tick status = %q, want ready", got)
	}
}

// A CELL CANNOT SUSPEND OR DELETE A SERVICE — enforced on the ROUTE the
// reconciler token actually reaches, not only in the mapping.
//
// `statusVocab` admits both because it mirrors the customer-facing ServiceStatus
// enum. Left ungated, CanTransition accepts both straight from `ready` and one
// POST bricks the service permanently: the edge lands, but no `deleting:true`
// desired doc is produced and no teardown runs, `deleting` has no outgoing edge,
// and DeleteService then answers "deletion already in progress" forever — with
// the metering span closed and the workload still running. The reconciler secret
// is one shared value across a configured cell list.
func TestHTTPACellCannotSuspendOrDeleteAService(t *testing.T) {
	for _, status := range []string{"deleting", "suspended"} {
		ts, q, tr := mount(t)
		q.services["svc_r"] = store.Service{
			ID: "svc_r", CellID: "cell-0", Status: "ready", Generation: 2, ObservedGeneration: 1,
		}
		r, m := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
			`{"service_id":"svc_r","observed_generation":2,"status":"`+status+`"}`)
		if r.StatusCode != 422 {
			t.Errorf("%s: want 422, got %d — a cell reports what it observes, it does not "+
				"issue lifecycle commands", status, r.StatusCode)
		}
		if tr.calls != 0 {
			t.Errorf("%s: the report reached the status machine", status)
		}
		if got := q.services["svc_r"].Status; got != "ready" {
			t.Errorf("%s: a cell moved a ready service to %q", status, got)
		}
		// And the quieter attack: it must not advance observation either, or the
		// row silently stops being reconciled.
		if got := q.services["svc_r"].ObservedGeneration; got != 1 {
			t.Errorf("%s: observed_generation advanced to %d on a refused report", status, got)
		}
		// Assert on m["errors"], NEVER on the whole body: every problem+json
		// carries `"status": 422`, so `strings.Contains(fmt.Sprint(m), "status")`
		// — which is what this used to be — cannot fail. Measured: blanking the
		// FieldError's Field survived it.
		errs, _ := m["errors"].([]any)
		if len(errs) == 0 {
			t.Fatalf("%s: no field errors on the refusal: %v", status, m)
		}
		var detail string
		for _, e := range errs {
			fe, _ := e.(map[string]any)
			if f, _ := fe["field"].(string); f == "status" {
				detail, _ = fe["detail"].(string)
			}
		}
		if detail == "" {
			t.Errorf("%s: the refusal does not name `status` as the offending field: %v", status, m)
		}
		// The SPECIFIC message is the entire justification for statusVocab being
		// wider than the contract enum, so it is asserted, not assumed. Measured:
		// replacing it with the generic vocabulary message survived.
		if !strings.Contains(detail, "lifecycle state the control plane sets") {
			t.Errorf("%s: the refusal is generic (%q) — a cell is told the value is not in the "+
				"ADR-024 vocabulary, which is true of neither value", status, detail)
		}
		if !strings.Contains(detail, status) {
			t.Errorf("%s: the refusal does not quote the offending value: %q", status, detail)
		}
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

// The /desired payload MUST carry per-service "status" — the AckRenderer's
// delete-hot-loop fix branches on it. If the field drifts or loses its json tag,
// every other test stays green while the deleting→ready 409 loop silently
// returns; this pins the wire seam. (fakeQ's rows carry status "provisioning".)
func TestHTTPDesiredCarriesStatus(t *testing.T) {
	ts, _, _ := mount(t)
	_, m := call(t, ts, "GET", "/v1/reconcile/cell-0/desired", "s3cret", "")
	svcs, ok := m["services"].([]any)
	if !ok || len(svcs) == 0 {
		t.Fatalf("no services in payload: %v", m["services"])
	}
	row := svcs[0].(map[string]any)
	if _, has := row["status"]; !has {
		t.Fatalf("the /desired payload must carry per-service status (the renderer branches on it): %v", row)
	}
	if row["status"] == "" {
		t.Fatal("status present but empty — the renderer cannot distinguish a deleting service")
	}
}

// AN OMITTED STATUS IS NOT `gone`, ON THE ROUTE.
//
// The handler used to normalise the wire's `gone` into "" and US-3.3h stopped
// it, because the two mean opposite things: "" is the observation-only ack and
// finishes the generation on a settled row, while `gone` on a live row means the
// workload VANISHED and must keep the row outstanding so the agent re-creates it.
//
// Only the gone→"" direction was pinned. Measured: adding the REVERSE
// (`if b.Status == "" { b.Status = "gone" }` just before Writeback) survived the
// entire reconcile suite — and this PR's own spec text newly claims an omitted
// status "finishes the generation only when the row already rests on a settled
// status". Both directions are driven here, through the real route.
func TestHTTPAnOmittedStatusIsNotGone(t *testing.T) {
	// The ack: settled row, nothing reported → finished.
	ts, q, tr := mount(t)
	q.services["svc_s"] = store.Service{
		ID: "svc_s", CellID: "cell-0", Status: "ready", Generation: 5, ObservedGeneration: 4,
	}
	r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_s","observed_generation":5}`)
	if r.StatusCode != 200 {
		t.Fatalf("an observation-only ack on a settled row: want 200, got %d", r.StatusCode)
	}
	if got := q.services["svc_s"].ObservedGeneration; got != 5 {
		t.Errorf("observed_generation = %d, want 5 — the ack must finish the generation", got)
	}
	if got := q.services["svc_s"].Status; got != "ready" {
		t.Errorf("the ack moved status to %q", got)
	}
	if tr.calls != 0 {
		t.Error("an ack drove the status machine")
	}

	// `gone` on the SAME row: the workload vanished, so it must NOT finish.
	ts2, q2, _ := mount(t)
	q2.services["svc_s"] = store.Service{
		ID: "svc_s", CellID: "cell-0", Status: "ready", Generation: 5, ObservedGeneration: 4,
	}
	r2, _ := call(t, ts2, "POST", "/v1/reconcile/cell-0/status", "s3cret",
		`{"service_id":"svc_s","observed_generation":5,"status":"gone"}`)
	if r2.StatusCode != 409 {
		t.Fatalf("`gone` on a live service: want 409, got %d — it is being treated as an ack, "+
			"so a vanished workload leaves the outstanding set and nothing re-creates it",
			r2.StatusCode)
	}
	if got := q2.services["svc_s"].ObservedGeneration; got == 5 {
		t.Error("`gone` on a live service advanced observed_generation")
	}
}

// mountEnv is mount(t) with an environment awaiting teardown in cell-0.
func mountEnv(t *testing.T) (*httptest.Server, *fakeQ) {
	t.Helper()
	ts, q, _ := mount(t)
	q.envCell = map[string]string{"env_a": "cell-0", "env_far": "cell-9", "env_unsched": "cell-0"}
	q.envState = map[string]struct{ scheduled, torn bool }{
		"env_a":       {scheduled: true},
		"env_unsched": {},
	}
	q.envs = []store.ListEnvironmentTeardownsForCellRow{{ID: "env_a"}}
	return ts, q
}

// THE POLL CARRIES ENVIRONMENTS, and both keys are always present.
//
// A nil slice marshals to `null`, which a generated client decodes as absent —
// indistinguishable from "this control plane is too old to send it". The
// contract marks both required precisely to remove that ambiguity.
func TestHTTPDesiredCarriesEnvironmentsAlways(t *testing.T) {
	ts, _ := mountEnv(t)
	_, m := call(t, ts, "GET", "/v1/reconcile/cell-0/desired", "s3cret", "")
	envs, ok := m["environments"].([]any)
	if !ok {
		t.Fatalf("no `environments` key in the poll (or it is null): %v", m)
	}
	if len(envs) != 1 {
		t.Fatalf("want one environment awaiting teardown, got %v", envs)
	}
	e, _ := envs[0].(map[string]any)
	if e["id"] != "env_a" {
		t.Errorf("environment id = %v", e["id"])
	}
	if e["namespace"] != "env-a" {
		t.Errorf("namespace = %v, want env-a (the control plane resolves it, ADR-0012)", e["namespace"])
	}

	// And when there is nothing to tear down, the key is an empty ARRAY, not null.
	ts2, q2 := mountEnv(t)
	q2.envs = nil
	_, m2 := call(t, ts2, "GET", "/v1/reconcile/cell-0/desired", "s3cret", "")
	raw, err := json.Marshal(m2["environments"])
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "[]" {
		t.Errorf("`environments` with no work marshalled as %s, want [] — null reads as "+
			"'this server does not support teardowns'", raw)
	}
}

func TestHTTPEnvironmentTeardownConfirms(t *testing.T) {
	ts, q := mountEnv(t)
	r, m := call(t, ts, "POST", "/v1/reconcile/cell-0/environments/env_a/teardown", "s3cret",
		`{"observed":"gone"}`)
	if r.StatusCode != 200 {
		t.Fatalf("want 200, got %d: %v", r.StatusCode, m)
	}
	if len(q.tornDown) != 1 || q.tornDown[0] != "env_a" {
		t.Fatalf("torn down %v, want [env_a]", q.tornDown)
	}
	// A replay is a 409, not a second teardown.
	r2, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/environments/env_a/teardown", "s3cret",
		`{"observed":"gone"}`)
	if r2.StatusCode != 409 {
		t.Fatalf("replay: want 409, got %d", r2.StatusCode)
	}
	if len(q.tornDown) != 1 {
		t.Fatalf("the replay tore down again: %v", q.tornDown)
	}
}

// AN ENVIRONMENT ON ANOTHER CELL IS 404 — never 403, and never a distinct
// "exists but not yours", or a reconciler token could enumerate them.
func TestHTTPEnvironmentTeardownDoesNotLeakOtherCells(t *testing.T) {
	ts, q := mountEnv(t)
	r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/environments/env_far/teardown", "s3cret",
		`{"observed":"gone"}`)
	if r.StatusCode != 404 {
		t.Fatalf("want 404 for an environment on cell-9, got %d", r.StatusCode)
	}
	// Byte-identical to a genuinely unknown environment: the two must not be
	// distinguishable from the outside.
	_, mFar := call(t, ts, "POST", "/v1/reconcile/cell-0/environments/env_far/teardown", "s3cret",
		`{"observed":"gone"}`)
	_, mNope := call(t, ts, "POST", "/v1/reconcile/cell-0/environments/env_nope/teardown", "s3cret",
		`{"observed":"gone"}`)
	if fmt.Sprint(mFar) != fmt.Sprint(mNope) {
		t.Errorf("a foreign environment and a nonexistent one answer differently:\n far: %v\n none: %v",
			mFar, mNope)
	}
	if len(q.tornDown) != 0 {
		t.Fatalf("a foreign environment was torn down: %v", q.tornDown)
	}
}

// AN ENVIRONMENT NOBODY SCHEDULED CANNOT BE TORN DOWN by a report. The
// confirmation records a teardown; it must not be able to invent one.
func TestHTTPAnUnscheduledEnvironmentCannotBeConfirmed(t *testing.T) {
	ts, q := mountEnv(t)
	r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/environments/env_unsched/teardown", "s3cret",
		`{"observed":"gone"}`)
	if r.StatusCode != 409 {
		t.Fatalf("want 409 for an environment with no scheduled deletion, got %d", r.StatusCode)
	}
	if len(q.tornDown) != 0 {
		t.Fatalf("an unscheduled environment was marked torn down: %v", q.tornDown)
	}
}

// `observed` IS AN ENUM OF ONE. A teardown that did not finish is reported by
// NOT calling this, so anything other than `gone` is a malformed report — and
// must not be read as a confirmation.
func TestHTTPEnvironmentTeardownRefusesAnythingButGone(t *testing.T) {
	for _, body := range []string{
		`{"observed":"ready"}`, `{"observed":""}`, `{}`, `{"observed":"gone","extra":1}`, `not json`,
	} {
		ts, q := mountEnv(t)
		r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/environments/env_a/teardown", "s3cret", body)
		if r.StatusCode != 422 {
			t.Errorf("body %s: want 422, got %d", body, r.StatusCode)
		}
		if len(q.tornDown) != 0 {
			t.Errorf("body %s: marked torn down anyway: %v", body, q.tornDown)
		}
	}
}

// The teardown route sits behind the SAME auth ladder as the rest of the plane.
func TestHTTPEnvironmentTeardownAuthLadder(t *testing.T) {
	ts, _ := mountEnv(t)
	if r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/environments/env_a/teardown", "",
		`{"observed":"gone"}`); r.StatusCode != 401 {
		t.Errorf("no token: want 401, got %d", r.StatusCode)
	}
	if r, _ := call(t, ts, "POST", "/v1/reconcile/cell-0/environments/env_a/teardown", "wrong",
		`{"observed":"gone"}`); r.StatusCode != 401 {
		t.Errorf("bad token: want 401, got %d", r.StatusCode)
	}
	if r, _ := call(t, ts, "POST", "/v1/reconcile/cell-9/environments/env_a/teardown", "s3cret",
		`{"observed":"gone"}`); r.StatusCode != 404 {
		t.Errorf("a cell this token does not own: want 404, got %d", r.StatusCode)
	}
}
