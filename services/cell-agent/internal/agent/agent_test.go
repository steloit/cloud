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
	desired    map[int64][]DesiredService // keyed by the sinceGeneration asked
	desiredErr error
	reports    []Report
	reportErr  error
	polls      int
}

func (f *fakeCP) Desired(_ context.Context, _ string, since int64) ([]DesiredService, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.polls++
	if f.desiredErr != nil {
		return nil, f.desiredErr
	}
	var out []DesiredService
	for _, batch := range f.desired {
		for _, s := range batch {
			if s.Generation > since {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

func (f *fakeCP) Report(_ context.Context, _ string, r Report) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reportErr != nil {
		return f.reportErr
	}
	f.reports = append(f.reports, r)
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

func svc(id string, gen int64) DesiredService {
	return DesiredService{ID: id, CellID: "cell-0", Product: "postgres", Generation: gen, Desired: map[string]any{"product": "postgres"}}
}

// THE headline guarantee: a control-plane outage must not touch actual state.
func TestControlPlaneOutageSkipsConvergence(t *testing.T) {
	cp := &fakeCP{desiredErr: errors.New("connection refused")}
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
	cp := &fakeCP{desired: map[int64][]DesiredService{0: {svc("svc_a", 3)}}}
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

func TestWatermarkAdvancesSoConvergedWorkIsNotRepolled(t *testing.T) {
	cp := &fakeCP{desired: map[int64][]DesiredService{0: {svc("svc_a", 3)}}}
	r := &fakeRenderer{}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background())
	a.Tick(context.Background()) // second pass: watermark is now 3, svc_a (gen 3) is not > 3
	if len(r.converged) != 1 {
		t.Fatalf("converged svc_a %d times; watermark should have filtered it after the first", len(r.converged))
	}
}

func TestFailedConvergeIsRetriedNotSkipped(t *testing.T) {
	cp := &fakeCP{desired: map[int64][]DesiredService{0: {svc("svc_a", 3)}}}
	r := &fakeRenderer{failFor: "svc_a"}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background())
	if len(cp.reports) != 0 {
		t.Fatal("a failed converge must not report")
	}
	// watermark did not advance, so the next pass sees svc_a again
	r.failFor = ""
	a.Tick(context.Background())
	if len(r.converged) != 1 || len(cp.reports) != 1 {
		t.Fatalf("failed converge was not retried: converged=%v reports=%v", r.converged, cp.reports)
	}
}

func TestFailedWritebackDoesNotAdvanceWatermark(t *testing.T) {
	cp := &fakeCP{desired: map[int64][]DesiredService{0: {svc("svc_a", 3)}}, reportErr: errors.New("stale, re-poll")}
	r := &fakeRenderer{}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background())
	// converge happened but the report was rejected; watermark must stay so the
	// next pass reports again (converge is idempotent, so re-running is safe).
	cp.reportErr = nil
	a.Tick(context.Background())
	if len(cp.reports) != 1 {
		t.Fatalf("writeback was not retried after failure: %+v", cp.reports)
	}
}

func TestBatchIsolationOneFailureDoesNotBlockOthers(t *testing.T) {
	cp := &fakeCP{desired: map[int64][]DesiredService{0: {svc("svc_a", 3), svc("svc_b", 4)}}}
	r := &fakeRenderer{failFor: "svc_a"}
	a := New("cell-0", cp, r, quietLog())
	a.Tick(context.Background())
	// svc_b must converge and report even though svc_a failed
	if len(r.converged) != 1 || r.converged[0] != "svc_b" {
		t.Fatalf("batch isolation broken: %v", r.converged)
	}
	if len(cp.reports) != 1 || cp.reports[0].ServiceID != "svc_b" {
		t.Fatalf("svc_b should have reported: %+v", cp.reports)
	}
	// AND the watermark must NOT advance past the failed svc_a (gen 3), or svc_a
	// would be lost. It should still be < 4.
	a.Tick(context.Background())
	// svc_a (gen 3) is still re-polled because watermark stayed at... let's just
	// assert svc_a eventually gets a chance: clear the failure and tick.
	r.failFor = ""
	a.Tick(context.Background())
	converged := map[string]bool{}
	for _, id := range r.converged {
		converged[id] = true
	}
	if !converged["svc_a"] {
		t.Fatal("svc_a was permanently skipped after a failure — watermark advanced past it")
	}
}
