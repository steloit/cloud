package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// fakeQ is an in-memory Querier. The reconciler's load-bearing rules —
// staleness rejection, cell fencing, idempotency — are pure logic over the
// store, so they are provable without Postgres. The SQL guard itself
// (generation >= $2) is pinned by the integration test.
type fakeQ struct {
	mu        sync.Mutex
	cells     map[string]store.Cell
	services  map[string]store.Service
	rows      []store.ListDesiredForCellRow
	heartbeat int
	observed  int
}

func (f *fakeQ) GetCell(_ context.Context, id string) (store.Cell, error) {
	if c, ok := f.cells[id]; ok {
		return c, nil
	}
	return store.Cell{}, pgx.ErrNoRows
}

func (f *fakeQ) ListDesiredForCell(_ context.Context, a store.ListDesiredForCellParams) ([]store.ListDesiredForCellRow, error) {
	var out []store.ListDesiredForCellRow
	for _, r := range f.rows {
		if r.CellID == a.CellID && r.Generation > a.Generation {
			out = append(out, r)
		}
		if int32(len(out)) >= a.Limit {
			break
		}
	}
	return out, nil
}

func (f *fakeQ) MarkObserved(_ context.Context, a store.MarkObservedParams) (store.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	svc, ok := f.services[a.ID]
	if !ok {
		return store.Service{}, pgx.ErrNoRows
	}
	// mirrors the SQL guard: WHERE generation = $2 (exact). A report for any
	// generation other than the one desired holds now is rejected.
	if svc.Generation != a.ObservedGeneration {
		return store.Service{}, pgx.ErrNoRows
	}
	f.observed++
	svc.ObservedGeneration = a.ObservedGeneration
	f.services[a.ID] = svc
	return svc, nil
}

func (f *fakeQ) TouchCellHeartbeat(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.heartbeat++
	return nil
}

func (f *fakeQ) GetService(_ context.Context, id string) (store.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.services[id]; ok {
		return s, nil
	}
	return store.Service{}, pgx.ErrNoRows
}

func (f *fakeQ) OrgForService(context.Context, string) (string, error) { return "org_1", nil }

// fakeTrans records edges and enforces the one rule the reconciler depends on:
// an edge is applied at most once per (id, from→to).
type fakeTrans struct {
	mu    sync.Mutex
	calls int
	edges []string
	q     *fakeQ
}

func (t *fakeTrans) Transition(_ context.Context, svc store.Service, to, via, _, _ string) (store.Service, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cur := t.q.services[svc.ID]
	if cur.Status == to {
		return cur, errors.New("illegal transition (already there)")
	}
	t.calls++
	t.edges = append(t.edges, cur.Status+"→"+to+"/"+via)
	cur.Status = to
	t.q.services[svc.ID] = cur
	return cur, nil
}

func newFixture() (*Service, *fakeQ, *fakeTrans) {
	q := &fakeQ{
		cells: map[string]store.Cell{"cell-0": {ID: "cell-0", Region: "us-central1", Status: "active"}},
		services: map[string]store.Service{
			"svc_a":   {ID: "svc_a", CellID: "cell-0", Status: "provisioning", Generation: 3, ObservedGeneration: 0},
			"svc_far": {ID: "svc_far", CellID: "cell-9", Status: "ready", Generation: 1},
		},
		rows: []store.ListDesiredForCellRow{
			{ID: "svc_a", CellID: "cell-0", Generation: 3, Desired: []byte(`{"product":"postgres"}`), Shape: []byte(`{}`), Product: "postgres", Status: "provisioning"},
			{ID: "svc_b", CellID: "cell-0", Generation: 7, Desired: []byte(`{"product":"web"}`), Shape: []byte(`{}`), Product: "web", Status: "ready"},
			{ID: "svc_far", CellID: "cell-9", Generation: 4, Desired: []byte(`{}`), Shape: []byte(`{}`), Product: "web", Status: "ready"},
		},
	}
	tr := &fakeTrans{q: q}
	return New(q, tr), q, tr
}

func TestDesiredScopedToCell(t *testing.T) {
	s, _, _ := newFixture()
	got, err := s.Desired(context.Background(), "cell-0", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range got {
		if d.CellID != "cell-0" {
			t.Fatalf("leaked a foreign cell's service: %s in %s", d.ID, d.CellID)
		}
	}
	if len(got) != 2 {
		t.Fatalf("want 2 services in cell-0, got %d", len(got))
	}
}

func TestSinceGenerationFiltersRows(t *testing.T) {
	s, _, _ := newFixture()
	got, err := s.Desired(context.Background(), "cell-0", 3, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "svc_b" {
		t.Fatalf("since_generation=3 should return only svc_b (gen 7); got %+v", got)
	}
}

func TestForeignCellIs404(t *testing.T) {
	s, _, _ := newFixture()
	if _, err := s.Desired(context.Background(), "cell-nope", 0, 10); !errors.Is(err, ErrUnknownCell) {
		t.Fatalf("unknown cell must be ErrUnknownCell (404), got %v", err)
	}
	// A service that exists but lives elsewhere must be indistinguishable from
	// one that does not exist — no cross-cell probing.
	_, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_far", ObservedGeneration: 1, Status: "ready"})
	if !errors.Is(err, ErrUnknownCell) {
		t.Fatalf("foreign-cell service must be ErrUnknownCell, got %v", err)
	}
}

func TestStatusWritebackAdvancesObservedGeneration(t *testing.T) {
	s, q, _ := newFixture()
	if _, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3, Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	if got := q.services["svc_a"].ObservedGeneration; got != 3 {
		t.Fatalf("observed_generation not advanced: %d", got)
	}
}

func TestAheadGenerationWritebackIsRejected(t *testing.T) {
	s, q, tr := newFixture()
	// svc_a is at generation 3; the agent reports 9 — impossible (ahead of
	// desired, a replay/bug). Rejected.
	_, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 9, Status: "ready"})
	if !errors.Is(err, ErrStaleGeneration) {
		t.Fatalf("want ErrStaleGeneration, got %v", err)
	}
	if tr.calls != 0 || q.services["svc_a"].Status != "provisioning" {
		t.Fatal("a rejected writeback must not drive the machine or mutate status")
	}
}

// The AC's literal scenario: the agent reports generation N while desired has
// moved to N+1 (it converged the OLD desired). Must be rejected, row unchanged.
func TestBehindGenerationWritebackIsRejected(t *testing.T) {
	s, q, tr := newFixture()
	// svc_a is at generation 3, observed 0. The agent converged an older gen 2
	// and reports 2 — behind the current desired. Must NOT advance or transition.
	_, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 2, Status: "ready"})
	if !errors.Is(err, ErrStaleGeneration) {
		t.Fatalf("a behind report must be rejected, got %v", err)
	}
	if tr.calls != 0 {
		t.Fatal("a behind report must not drive the status machine off stale desired")
	}
	if q.services["svc_a"].ObservedGeneration != 0 || q.services["svc_a"].Status != "provisioning" {
		t.Fatal("a behind report must leave the row unchanged")
	}
}

// A delayed OLD report cannot regress status: report gen 3/ready, then a stale
// gen 2/degraded arrives late. The exact-match guard rejects gen 2 (current is 3
// after... actually generation stays 3, observed advances). Ensure status holds.
func TestDelayedOldReportCannotRegressStatus(t *testing.T) {
	s, q, _ := newFixture()
	if _, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3, Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	// A late report on generation 2 (a slow duplicate) — rejected, cannot regress.
	_, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 2, Status: "degraded"})
	if !errors.Is(err, ErrStaleGeneration) {
		t.Fatalf("a late old-generation report must be rejected, got %v", err)
	}
	if q.services["svc_a"].Status != "ready" {
		t.Fatalf("status regressed to %q via a stale report", q.services["svc_a"].Status)
	}
}

func TestStatusWritebackIsIdempotent(t *testing.T) {
	s, q, tr := newFixture()
	rep := Report{ServiceID: "svc_a", ObservedGeneration: 3, Status: "ready"}
	if _, err := s.Writeback(context.Background(), "cell-0", rep); err != nil {
		t.Fatal(err)
	}
	// the same report again: observation is a no-op, and the edge is not re-applied
	if _, err := s.Writeback(context.Background(), "cell-0", rep); err != nil {
		t.Fatalf("repeating an identical writeback must be a no-op, got %v", err)
	}
	if tr.calls != 1 {
		t.Fatalf("status edge applied %d times, want exactly 1", tr.calls)
	}
	if q.services["svc_a"].ObservedGeneration != 3 {
		t.Fatal("observed_generation drifted on replay")
	}
}

func TestConcurrentWritebackAppliesOnce(t *testing.T) {
	s, _, tr := newFixture()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, _ = s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3, Status: "ready"})
		})
	}
	wg.Wait()
	if tr.calls != 1 {
		t.Fatalf("concurrent writebacks applied the edge %d times, want exactly 1", tr.calls)
	}
}

func TestHeartbeatFiresEvenOnRejectedReport(t *testing.T) {
	s, q, _ := newFixture()
	// The heartbeat rides the status call (§2 step 4) and must fire BEFORE the
	// generation guard: a cell reporting a stale generation is still alive, and
	// must not be counted dead just because its report was rejected.
	_, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 1})
	if !errors.Is(err, ErrStaleGeneration) {
		t.Fatalf("gen 1 vs desired 3 should be rejected, got %v", err)
	}
	if q.heartbeat != 1 {
		t.Fatalf("heartbeat must fire even on a rejected report (%d)", q.heartbeat)
	}
}

func TestHeartbeatOnCurrentGenerationObservation(t *testing.T) {
	s, q, _ := newFixture()
	if _, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3}); err != nil {
		t.Fatal(err)
	}
	if q.heartbeat != 1 {
		t.Fatalf("heartbeat not recorded on an observation-only writeback (%d)", q.heartbeat)
	}
}

func TestObservationOnlyWritebackSkipsTransition(t *testing.T) {
	s, _, tr := newFixture()
	// Current generation (3), no status → observation only, accepted, no edge.
	if _, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3}); err != nil {
		t.Fatal(err)
	}
	if tr.calls != 0 {
		t.Fatal("a report with no status must not drive the status machine")
	}
}

func TestTransitionIsRecordedAsSystemVia(t *testing.T) {
	s, _, tr := newFixture()
	if _, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3, Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	if len(tr.edges) != 1 || tr.edges[0] != "provisioning→ready/system" {
		t.Fatalf("edge provenance wrong: %v (must be via=system — the cell converged, not a person)", tr.edges)
	}
}

func TestDesiredLimitIsClamped(t *testing.T) {
	s, _, _ := newFixture()
	if _, err := s.Desired(context.Background(), "cell-0", 0, 100000); err != nil {
		t.Fatal(err)
	} // clamped to maxLimit rather than rejected — an agent must not be able to
	// ask for an unbounded page.
	if _, err := s.Desired(context.Background(), "cell-0", -5, 0); err != nil {
		t.Fatalf("negative since_generation must normalize, not error: %v", err)
	}
}

func TestDesiredCarriesFullDocumentForLevelTriggeredConverge(t *testing.T) {
	s, _, _ := newFixture()
	got, _ := s.Desired(context.Background(), "cell-0", 0, 100)
	for _, d := range got {
		if len(d.Desired) == 0 || !json.Valid(d.Desired) {
			t.Fatalf("%s carries no valid desired doc — the agent would have to diff by memory", d.ID)
		}
	}
}

// ---- auth -------------------------------------------------------------------

func req(tok string) *http.Request {
	r, _ := http.NewRequest("GET", "/", nil)
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	return r
}

func TestDesiredRequiresReconcilerToken(t *testing.T) {
	a := NewAuth("s3cret", []string{"cell-0"})
	if a.Allows(req(""), "cell-0") {
		t.Fatal("no token must not pass")
	}
	if a.Allows(req("wrong"), "cell-0") {
		t.Fatal("a wrong token must not pass")
	}
	if !a.Allows(req("s3cret"), "cell-0") {
		t.Fatal("the configured token must pass for its own cell")
	}
}

func TestTokenIsScopedToItsCells(t *testing.T) {
	a := NewAuth("s3cret", []string{"cell-0"})
	if a.Allows(req("s3cret"), "cell-9") {
		t.Fatal("a valid token must not reach a cell it does not own")
	}
	if !a.Authenticated(req("s3cret")) {
		t.Fatal("the token is still authentic — 404 not 401 is the handler's job")
	}
}

func TestUnconfiguredSecretFailsClosed(t *testing.T) {
	a := NewAuth("", []string{"cell-0"})
	if a.Enabled() {
		t.Fatal("an empty secret must leave the endpoints disabled")
	}
	if a.Allows(req("anything"), "cell-0") || a.Allows(req(""), "cell-0") {
		t.Fatal("absent config must mean CLOSED, never open")
	}
}
