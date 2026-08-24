package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeCP is a scriptable control plane.
type fakeCP struct {
	mu         sync.Mutex
	services   map[string]DesiredService // id -> desired (with observed_generation)
	desiredErr error
	reports    []Report
	reportErr  error
	polls      int

	// The ENVIRONMENT half. `envs` is what the control plane advertises;
	// `confirmed` is what the agent reported back, in order.
	envs       []DesiredEnvironmentTeardown
	confirmed  []string
	confirmErr error

	// envCellIgnored exists only so a test literal can say out loud that this
	// fake does not model cell ownership — that rule is the control plane's and
	// is tested there, against real Postgres.
	envCellIgnored bool

	// seq is the ONE ordered log both halves append to, so a test can assert
	// which happened first. Two separate counters cannot: `len(reports) > 0 &&
	// len(confirmed) > 0` is satisfied by either order, which is how an ordering
	// test came to pass with the teardown moved ABOVE the service loop.
	seq []string
}

// Desired models the real server: it returns OUTSTANDING work — services whose
// observed_generation trails generation. The agent passes since=0; the fake
// ignores it, exactly as the outstanding-work query makes the cursor moot.
func (f *fakeCP) Desired(_ context.Context, _ string, _ int64) (DesiredState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.polls++
	if f.desiredErr != nil {
		return DesiredState{}, f.desiredErr
	}
	var out []DesiredService
	for _, s := range f.services {
		if s.ObservedGeneration < s.Generation {
			out = append(out, s)
		}
	}
	return DesiredState{Services: out, Environments: append([]DesiredEnvironmentTeardown(nil), f.envs...)}, nil
}

// Report advances observed_generation for the reported generation ONLY if it
// equals the current generation (the exact-match guard) — mirroring MarkObserved.
func (f *fakeCP) applyReport(r Report) {
	s, ok := f.services[r.ServiceID]
	if !ok || s.Generation != r.ObservedGeneration {
		return // stale/mismatched: rejected, stays outstanding
	}
	s.ObservedGeneration = r.ObservedGeneration
	f.services[r.ServiceID] = s
}

// The fake stops advertising a confirmed environment, exactly as the real
// control plane does (torn_down_at is what removes it from the poll). Without
// that, a test could not tell "confirmed once" from "confirmed every tick".
func (f *fakeCP) ConfirmEnvironmentTeardown(_ context.Context, _, envID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.confirmErr != nil {
		return f.confirmErr
	}
	f.confirmed = append(f.confirmed, envID)
	f.seq = append(f.seq, "teardown:"+envID)
	kept := f.envs[:0]
	for _, e := range f.envs {
		if e.ID != envID {
			kept = append(kept, e)
		}
	}
	f.envs = kept
	return nil
}

func (f *fakeCP) Report(_ context.Context, _ string, r Report) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reportErr != nil {
		return f.reportErr
	}
	f.reports = append(f.reports, r)
	f.seq = append(f.seq, "report:"+r.ServiceID)
	f.applyReport(r)
	return nil
}

// fakeRenderer records converges and can fail a chosen service.
type fakeRenderer struct {
	mu        sync.Mutex
	converged []string
	failFor   string
	observe   string // status to report; default "ready"
}

func (r *fakeRenderer) Converge(_ context.Context, svc DesiredService) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if svc.ID == r.failFor {
		return "", errors.New("render boom")
	}
	r.converged = append(r.converged, svc.ID)
	if r.observe != "" {
		return r.observe, nil
	}
	return "ready", nil
}

func svc(id string, gen, observed int64) DesiredService {
	return DesiredService{ID: id, CellID: "cell-0", Product: "postgres", Generation: gen, ObservedGeneration: observed, Desired: map[string]any{"product": "postgres"}}
}

func cpWith(svcs ...DesiredService) *fakeCP {
	m := make(map[string]DesiredService, len(svcs))
	for _, s := range svcs {
		m[s.ID] = s
	}
	return &fakeCP{services: m}
}

// THE headline guarantee: a control-plane outage must not touch actual state.
func TestControlPlaneOutageSkipsConvergence(t *testing.T) {
	cp := &fakeCP{desiredErr: errors.New("connection refused"), services: map[string]DesiredService{}}
	r := &fakeRenderer{}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background())
	if len(r.converged) != 0 {
		t.Fatal("a failed poll must NOT converge anything — apps must keep running untouched")
	}
	if len(cp.reports) != 0 {
		t.Fatal("a failed poll must not write back")
	}
}

func TestConvergesAndReportsObservedGeneration(t *testing.T) {
	cp := cpWith(svc("svc_a", 3, 0))
	r := &fakeRenderer{}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background())
	if len(r.converged) != 1 || r.converged[0] != "svc_a" {
		t.Fatalf("expected svc_a converged, got %v", r.converged)
	}
	if len(cp.reports) != 1 || cp.reports[0].ObservedGeneration != 3 || cp.reports[0].Status != "ready" {
		t.Fatalf("writeback wrong: %+v", cp.reports)
	}
}

// Converged work stops being outstanding — the server filter, not a client
// cursor, is what prevents re-converging.
func TestConvergedWorkIsNotRepolled(t *testing.T) {
	cp := cpWith(svc("svc_a", 3, 0))
	r := &fakeRenderer{}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background()) // converges, reports 3 → observed becomes 3
	a.Tick(context.Background()) // svc_a now observed==generation, not outstanding
	if len(r.converged) != 1 {
		t.Fatalf("converged svc_a %d times; a reported service must drop out of the outstanding set", len(r.converged))
	}
}

// The regression QA sketched: a NEW service created after others have converged
// (lower per-row generation) must still be picked up. The old watermark starved it.
func TestNewServiceAfterOthersConvergedIsNotStarved(t *testing.T) {
	cp := cpWith(svc("svc_old", 5, 0))
	r := &fakeRenderer{}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background()) // svc_old (gen 5) converges → observed 5
	// A brand-new service arrives with generation 1 (per-row default) — LOWER
	// than svc_old's. A cell-wide cursor at 5 would starve it forever.
	cp.mu.Lock()
	cp.services["svc_new"] = svc("svc_new", 1, 0)
	cp.mu.Unlock()
	a.Tick(context.Background())
	if !contains(r.converged, "svc_new") {
		t.Fatal("a new service with a lower generation was starved — the cursor bug is back")
	}
}

func TestFailedConvergeIsRetried(t *testing.T) {
	cp := cpWith(svc("svc_a", 3, 0))
	r := &fakeRenderer{failFor: "svc_a"}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background())
	if len(cp.reports) != 0 {
		t.Fatal("a failed converge must not report")
	}
	// svc_a stayed outstanding (never reported), so the next tick sees it again.
	r.failFor = ""
	a.Tick(context.Background())
	if !contains(r.converged, "svc_a") || len(cp.reports) != 1 {
		t.Fatalf("failed converge was not retried: converged=%v reports=%v", r.converged, cp.reports)
	}
}

func TestFailedWritebackIsRetried(t *testing.T) {
	cp := cpWith(svc("svc_a", 3, 0))
	cp.reportErr = errors.New("mismatch, re-poll")
	r := &fakeRenderer{}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background()) // converge ok, report rejected → still outstanding
	cp.reportErr = nil
	a.Tick(context.Background())
	if len(cp.reports) < 1 || cp.services["svc_a"].ObservedGeneration != 3 {
		t.Fatalf("writeback was not retried to completion: reports=%+v observed=%d", cp.reports, cp.services["svc_a"].ObservedGeneration)
	}
}

func TestBatchIsolationOneFailureDoesNotBlockOthers(t *testing.T) {
	cp := cpWith(svc("svc_a", 3, 0), svc("svc_b", 4, 0))
	r := &fakeRenderer{failFor: "svc_a"}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background())
	if !contains(r.converged, "svc_b") || contains(r.converged, "svc_a") {
		t.Fatalf("batch isolation broken: %v", r.converged)
	}
	if len(cp.reports) != 1 || cp.reports[0].ServiceID != "svc_b" {
		t.Fatalf("svc_b should have reported: %+v", cp.reports)
	}
	// svc_a stayed outstanding; once it stops failing it converges. It was never lost.
	r.failFor = ""
	a.Tick(context.Background())
	if !contains(r.converged, "svc_a") {
		t.Fatal("svc_a was permanently skipped after a failure")
	}
}

// End-to-end through the real Tick and HTTP client: the agent polls generation
// 1, but desired has bumped to 2 by the time it reports, so the report 409s; the
// NEXT tick polls generation 2 and succeeds. The 409 path is genuinely driven.
func TestAgentRecoversFromGenerationMismatch(t *testing.T) {
	var (
		mu         sync.Mutex
		pollGen    int64 = 1 // what /desired serves
		currentGen int64 = 2 // what /status accepts (desired has already moved)
		conflicts  int
		accepted   []int64
	)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/reconcile/{cell}/desired", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		g := pollGen
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"services": []DesiredService{{ID: "svc_a", CellID: "cell-0", Product: "postgres", Generation: g, Desired: map[string]any{}}},
		})
	})
	mux.HandleFunc("POST /v1/reconcile/{cell}/status", func(w http.ResponseWriter, r *http.Request) {
		var rep Report
		_ = json.NewDecoder(r.Body).Decode(&rep)
		mu.Lock()
		defer mu.Unlock()
		if rep.ObservedGeneration != currentGen {
			conflicts++
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"remediation": "re-poll"})
			return
		}
		accepted = append(accepted, rep.ObservedGeneration)
		_ = json.NewEncoder(w).Encode(map[string]any{"service_id": rep.ServiceID, "status": rep.Status, "observed_generation": rep.ObservedGeneration})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	a := New("cell-0", NewHTTPControlPlane(ts.URL, "tok"), &countingRenderer{}, quietLog())

	// Tick 1: polls gen 1, reports gen 1, /status holds gen 2 → 409.
	a.Tick(context.Background())
	mu.Lock()
	if conflicts != 1 {
		mu.Unlock()
		t.Fatalf("tick 1 must produce exactly one 409 (the mismatch path); got %d", conflicts)
	}
	if len(accepted) != 0 {
		mu.Unlock()
		t.Fatalf("nothing should be accepted after a mismatch; got %v", accepted)
	}
	// desired settles: the poll now serves gen 2 too.
	pollGen = 2
	mu.Unlock()

	// Tick 2: polls gen 2, reports gen 2 → accepted.
	a.Tick(context.Background())
	mu.Lock()
	defer mu.Unlock()
	if len(accepted) != 1 || accepted[0] != 2 {
		t.Fatalf("tick 2 must converge the current generation; accepted=%v", accepted)
	}
}

func TestConcurrentTicksAreRaceFree(t *testing.T) {
	cp := cpWith(svc("svc_a", 1, 0), svc("svc_b", 1, 0))
	r := &fakeRenderer{}
	a := New("cell-0", cp, r, quietLog())
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { a.Tick(context.Background()) })
	}
	wg.Wait() // -race proves the agent holds no unguarded shared state
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func TestAckRendererDeletingConvergesToGone(t *testing.T) {
	r := NewAckRenderer(quietLog())
	got, err := r.Converge(context.Background(),
		DesiredService{ID: "svc_d", Product: "postgres", Desired: map[string]any{"deleting": true}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "gone" {
		t.Fatalf("a deleting desired must converge to gone, got %q", got)
	}
}

func TestAckRendererLiveConvergesToReady(t *testing.T) {
	r := NewAckRenderer(quietLog())
	got, _ := r.Converge(context.Background(),
		DesiredService{ID: "svc_l", Product: "postgres", Desired: map[string]any{}})
	if got != "ready" {
		t.Fatalf("a live desired must converge to ready, got %q", got)
	}
}

func TestAckRendererDeletingStatusConvergesToGone(t *testing.T) {
	r := NewAckRenderer(quietLog())
	// A service whose STATUS is deleting (cancel-the-create) converges to gone,
	// even before deletes write a desired.deleting flag — otherwise it would
	// report ready and hot-loop on the illegal deleting→ready edge.
	got, err := r.Converge(context.Background(),
		DesiredService{ID: "svc_c", Product: "postgres", Status: "deleting", Desired: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "gone" {
		t.Fatalf("a deleting service must converge to gone, got %q", got)
	}
}

// A renderer that CAN tear environments down. Separate from fakeRenderer so the
// "renderer does not implement it" case stays reachable — if every fake had the
// method, nothing would exercise the skip.
type envRenderer struct {
	fakeRenderer
	mu       sync.Mutex
	tornDown []string
	failFor  string
}

func (r *envRenderer) TeardownEnvironment(_ context.Context, ns string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failFor == ns {
		return errors.New("boom")
	}
	r.tornDown = append(r.tornDown, ns)
	return nil
}

// THE TICK TEARS DOWN THE ENVIRONMENTS THE CONTROL PLANE ADVERTISES, and
// confirms each one so it stops being advertised.
func TestTickTearsDownAdvertisedEnvironments(t *testing.T) {
	cp := &fakeCP{
		services: map[string]DesiredService{},
		envs: []DesiredEnvironmentTeardown{
			{ID: "env_a", Namespace: "env-a"},
			{ID: "env_b", Namespace: "env-b"},
		},
		envCellIgnored: true,
	}
	r := &envRenderer{}
	a := New("cell-0", cp, r, quietLog())

	a.Tick(context.Background())

	r.mu.Lock()
	torn := append([]string(nil), r.tornDown...)
	r.mu.Unlock()
	if len(torn) != 2 {
		t.Fatalf("tore down %v, want both namespaces", torn)
	}
	cp.mu.Lock()
	confirmed := append([]string(nil), cp.confirmed...)
	remaining := len(cp.envs)
	cp.mu.Unlock()
	if len(confirmed) != 2 {
		t.Fatalf("confirmed %v, want both environments", confirmed)
	}
	if remaining != 0 {
		t.Fatalf("%d environments still advertised after confirmation", remaining)
	}

	// A second tick must do nothing — the confirmation is what makes the work
	// stop, and re-tearing down a namespace every tick would be a hot loop.
	a.Tick(context.Background())
	r.mu.Lock()
	again := len(r.tornDown)
	r.mu.Unlock()
	if again != 2 {
		t.Fatalf("the second tick tore down %d more environments — a confirmed teardown must "+
			"stop being advertised", again-2)
	}
}

// A TEARDOWN THAT FAILS IS NOT CONFIRMED, so the environment stays outstanding
// and the next tick retries it. Confirming a teardown that did not happen is the
// one thing that loses the namespace forever.
func TestAFailedEnvironmentTeardownIsNotConfirmed(t *testing.T) {
	cp := &fakeCP{
		services: map[string]DesiredService{},
		envs: []DesiredEnvironmentTeardown{
			{ID: "env_bad", Namespace: "env-bad"},
			{ID: "env_ok", Namespace: "env-ok"},
		},
		envCellIgnored: true,
	}
	r := &envRenderer{failFor: "env-bad"}
	a := New("cell-0", cp, r, quietLog())

	a.Tick(context.Background())

	cp.mu.Lock()
	confirmed := append([]string(nil), cp.confirmed...)
	cp.mu.Unlock()
	if len(confirmed) != 1 || confirmed[0] != "env_ok" {
		t.Fatalf("confirmed %v — a failed teardown must not be confirmed, and one failure must "+
			"not stop the others", confirmed)
	}
}

// A CONFIRMATION THAT FAILS LEAVES THE NAMESPACE GONE AND THE ROW OUTSTANDING.
// The next tick re-advertises it, so TeardownEnvironment must be idempotent —
// this is the case that makes that requirement real rather than decorative.
func TestAFailedConfirmationRetriesTheWholeTeardown(t *testing.T) {
	cp := &fakeCP{
		services:       map[string]DesiredService{},
		envs:           []DesiredEnvironmentTeardown{{ID: "env_x", Namespace: "env-x"}},
		confirmErr:     errors.New("control plane down"),
		envCellIgnored: true,
	}
	r := &envRenderer{}
	a := New("cell-0", cp, r, quietLog())

	a.Tick(context.Background())
	cp.mu.Lock()
	confirmedFirst := len(cp.confirmed)
	stillAdvertised := len(cp.envs)
	cp.mu.Unlock()
	if confirmedFirst != 0 {
		t.Fatal("a failed confirmation was recorded as one")
	}
	if stillAdvertised != 1 {
		t.Fatal("the environment stopped being advertised despite an unconfirmed teardown")
	}

	// The control plane comes back; the next tick re-runs the teardown and
	// confirms it.
	cp.mu.Lock()
	cp.confirmErr = nil
	cp.mu.Unlock()
	a.Tick(context.Background())

	r.mu.Lock()
	torn := len(r.tornDown)
	r.mu.Unlock()
	if torn != 2 {
		t.Fatalf("the namespace was torn down %d times; the retry must re-run it (idempotently), "+
			"because the agent cannot know the first delete landed", torn)
	}
	cp.mu.Lock()
	confirmed := append([]string(nil), cp.confirmed...)
	cp.mu.Unlock()
	if len(confirmed) != 1 || confirmed[0] != "env_x" {
		t.Fatalf("confirmed %v after the retry, want [env_x]", confirmed)
	}
}

// A RENDERER THAT CANNOT TEAR ENVIRONMENTS DOWN DOES NOT SILENTLY DROP THE WORK.
// The alpha AckRenderer owns no environment-scoped objects; the loop must not
// confirm a teardown it never performed, or the namespace leaks with the control
// plane believing it is gone.
func TestARendererThatCannotTearDownNeverConfirms(t *testing.T) {
	cp := &fakeCP{
		services:       map[string]DesiredService{},
		envs:           []DesiredEnvironmentTeardown{{ID: "env_y", Namespace: "env-y"}},
		envCellIgnored: true,
	}
	a := New("cell-0", cp, &fakeRenderer{}, quietLog()) // no TeardownEnvironment

	a.Tick(context.Background())

	cp.mu.Lock()
	confirmed := len(cp.confirmed)
	advertised := len(cp.envs)
	cp.mu.Unlock()
	if confirmed != 0 {
		t.Fatal("a renderer that cannot tear down confirmed a teardown — the namespace leaks " +
			"and the control plane stops asking")
	}
	if advertised != 1 {
		t.Fatal("the environment stopped being advertised")
	}
}

// SERVICES ARE CONVERGED BEFORE ENVIRONMENTS ARE TORN DOWN.
//
// The real protection is server-side (an environment is not advertised until
// every service in it is gone), but a tick that removed a namespace before
// converging the services in it would be deleting a workload it was in the
// middle of managing.
//
// ASSERTED ON A SEQUENCE, not on two counters. The first version of this test
// checked `len(reports) > 0 && len(confirmed) > 0`, which is true in EITHER
// order — moving tearDownEnvironments above the service loop left it, and the
// whole package, green.
func TestEnvironmentsAreTornDownAfterServicesConverge(t *testing.T) {
	cp := &fakeCP{
		services: map[string]DesiredService{
			"svc_1": {ID: "svc_1", Generation: 1, ObservedGeneration: 0, Desired: map[string]any{}},
			"svc_2": {ID: "svc_2", Generation: 1, ObservedGeneration: 0, Desired: map[string]any{}},
		},
		envs: []DesiredEnvironmentTeardown{
			{ID: "env_z", Namespace: "env-z"},
			{ID: "env_w", Namespace: "env-w"},
		},
		envCellIgnored: true,
	}
	r := &envRenderer{}
	a := New("cell-0", cp, r, quietLog())

	a.Tick(context.Background())

	cp.mu.Lock()
	seq := append([]string(nil), cp.seq...)
	cp.mu.Unlock()

	if len(seq) != 4 {
		t.Fatalf("sequence = %v, want two reports and two teardowns", seq)
	}
	// EVERY report must precede EVERY teardown.
	lastReport, firstTeardown := -1, len(seq)
	for i, e := range seq {
		if strings.HasPrefix(e, "report:") && i > lastReport {
			lastReport = i
		}
		if strings.HasPrefix(e, "teardown:") && i < firstTeardown {
			firstTeardown = i
		}
	}
	if lastReport > firstTeardown {
		t.Fatalf("a namespace was torn down before every service was converged: %v\n"+
			"the tick would be deleting a workload it is in the middle of managing", seq)
	}
}
