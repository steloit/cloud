package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
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
}

// Desired models the real server: it returns OUTSTANDING work — services whose
// observed_generation trails generation. The agent passes since=0; the fake
// ignores it, exactly as the outstanding-work query makes the cursor moot.
func (f *fakeCP) Desired(_ context.Context, _ string, _ int64) ([]DesiredService, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.polls++
	if f.desiredErr != nil {
		return nil, f.desiredErr
	}
	var out []DesiredService
	for _, s := range f.services {
		if s.ObservedGeneration < s.Generation {
			out = append(out, s)
		}
	}
	return out, nil
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

func (f *fakeCP) Report(_ context.Context, _ string, r Report) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reportErr != nil {
		return f.reportErr
	}
	f.reports = append(f.reports, r)
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

// A behind report (agent converged gen N, desired has since bumped to N+1) is
// rejected by the exact-match guard the fake mirrors, so it stays outstanding.
func TestBehindReportIsRejectedAndStaysOutstanding(t *testing.T) {
	cp := cpWith(svc("svc_a", 2, 0)) // desired already at gen 2
	r := &fakeRenderer{}
	a := New("cell-0", cp, r, quietLog())
	// Force the agent to report gen 1 by handing the renderer a stale view:
	// simplest is to drive applyReport directly for the behind case.
	cp.applyReport(Report{ServiceID: "svc_a", ObservedGeneration: 1, Status: "ready"})
	if cp.services["svc_a"].ObservedGeneration != 0 {
		t.Fatal("a behind report (gen 1 vs desired 2) must be rejected, observed unchanged")
	}
	// the real current-generation report is accepted
	cp.applyReport(Report{ServiceID: "svc_a", ObservedGeneration: 2, Status: "ready"})
	if cp.services["svc_a"].ObservedGeneration != 2 {
		t.Fatal("the current-generation report must be accepted")
	}
	_ = a
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
