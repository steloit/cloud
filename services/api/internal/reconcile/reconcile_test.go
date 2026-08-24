package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/provisioning"
)

// fakeQ is an in-memory Querier. The reconciler's load-bearing rules —
// staleness rejection, cell fencing, idempotency — are pure logic over the
// store, so they are provable without Postgres. The SQL guard itself
// (generation = $2, exact match) is pinned by the integration test.
type fakeQ struct {
	mu        sync.Mutex
	cells     map[string]store.Cell
	services  map[string]store.Service
	rows      []store.ListDesiredForCellRow
	heartbeat int
	observed  int

	// afterRead runs at the end of GetService, OUTSIDE q.mu, and is nil in every
	// test but the concurrency one. It exists because giving q.services one owner
	// (Q10) also serialised GetService against Transition, which made the
	// concurrent test mostly stop exercising the window it is named for: the
	// second goroutine now reads status="ready" and Writeback's own
	// `rep.Status != svc.Status` pre-check filters it before Transition is
	// reached. Measured, at CI's plain `go test ./...`: removing the fixture's
	// exactly-once guard went undetected 97 runs in 100.
	//
	// So the test releases all N callers only once all N have READ, which forces
	// every one of them past the pre-check and into Transition. Deterministic
	// instead of luck-dependent. Must run outside the lock — blocking while
	// holding q.mu would deadlock on the first caller.
	afterRead func()

	// The ENVIRONMENT half (US-3.3b). `envs` is what the teardown poll returns;
	// `tornDown` records confirmations. `envCell` maps env id -> cell so
	// GetEnvironmentForCell can enforce the not-your-cell 404.
	envs     []store.ListEnvironmentTeardownsForCellRow
	envCell  map[string]string
	envState map[string]struct{ scheduled, torn bool }
	tornDown []string
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
	svc, ok := f.services[id]
	f.mu.Unlock()
	if f.afterRead != nil {
		f.afterRead() // outside the lock, deliberately — see the field comment
	}
	if !ok {
		return store.Service{}, pgx.ErrNoRows
	}
	return svc, nil
}

func (f *fakeQ) OrgForService(context.Context, string) (string, error) { return "org_1", nil }

// fakeTrans records edges and enforces the one rule the reconciler depends on:
// an edge is applied at most once per (id, from→to).
//
// It has NO mutex of its own, deliberately. Transition read-modify-writes
// q.services, so that map must have exactly ONE owner; two mutexes each
// guarding half the accesses exclude nothing (Q10/O14 — the detector caught
// Transition writing q.services while GetService/MarkObserved read it under
// q.mu). Everything fakeTrans mutates — calls, edges, and q.services — is
// therefore guarded by q.mu. No fakeQ *method* is called from here, so taking
// q.mu directly cannot deadlock against one that also takes it.
func (f *fakeQ) ListEnvironmentTeardownsForCell(_ context.Context, arg store.ListEnvironmentTeardownsForCellParams) ([]store.ListEnvironmentTeardownsForCellRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.ListEnvironmentTeardownsForCellRow
	for _, e := range f.envs {
		if f.envCell[e.ID] == arg.CellID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeQ) GetEnvironmentForCell(_ context.Context, arg store.GetEnvironmentForCellParams) (store.GetEnvironmentForCellRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Not-in-this-cell is indistinguishable from does-not-exist, which is the
	// point: a reconciler token must not learn that an environment exists
	// elsewhere.
	if c, ok := f.envCell[arg.ID]; !ok || c != arg.CellID {
		return store.GetEnvironmentForCellRow{}, pgx.ErrNoRows
	}
	return store.GetEnvironmentForCellRow{ID: arg.ID}, nil
}

func (f *fakeQ) MarkEnvironmentTornDown(_ context.Context, id string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Mirrors the SQL guard exactly: scheduled and not already torn down. The
	// fake must not be more permissive than the store on the guard that makes a
	// replay a no-op.
	st, ok := f.envState[id]
	if !ok || !st.scheduled || st.torn {
		return 0, nil
	}
	st.torn = true
	f.envState[id] = st
	f.tornDown = append(f.tornDown, id)
	// A torn-down environment stops being advertised.
	kept := f.envs[:0]
	for _, e := range f.envs {
		if e.ID != id {
			kept = append(kept, e)
		}
	}
	f.envs = kept
	return 1, nil
}

type fakeTrans struct {
	calls  int
	edges  []string
	q      *fakeQ
	failTo string // if set, Transition to this status returns an error (illegal edge)
}

// The fake delegates to the REAL mapping rather than reimplementing it. A fake
// status machine here would be a second copy of ADR-024 that drifts from the
// first — the exact thing the Transitioner interface exists to prevent.
func (t *fakeTrans) ObservedStatus(from, observed string) provisioning.Observation {
	return provisioning.ObservedStatus(from, observed)
}

func (t *fakeTrans) Transition(_ context.Context, svc store.Service, to, via, _, _ string) (store.Service, error) {
	t.q.mu.Lock()
	defer t.q.mu.Unlock()
	if t.failTo != "" && to == t.failTo {
		return store.Service{}, errors.New("illegal edge")
	}
	cur := t.q.services[svc.ID]
	if cur.Status == to {
		return cur, errors.New("illegal transition (already there)")
	}
	// The FROM-GUARD, which the real SetServiceStatus enforces atomically with
	// `WHERE status = $2` (ErrNoRows → the "state changed concurrently" 409).
	// Without it this fake is MORE PERMISSIVE than the store on exactly the
	// property the concurrency tests measure: a caller holding a stale read
	// could re-apply its edge on top of a newer status and walk the row
	// backwards. That is not reachable in production, so a test that observes it
	// here is measuring the fake, not the system.
	if cur.Status != svc.Status {
		return store.Service{}, errors.New("illegal transition (state changed concurrently)")
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
	for _, d := range got.Services {
		if d.CellID != "cell-0" {
			t.Fatalf("leaked a foreign cell's service: %s in %s", d.ID, d.CellID)
		}
	}
	if len(got.Services) != 2 {
		t.Fatalf("want 2 services in cell-0, got %d", len(got.Services))
	}
}

func TestSinceGenerationFiltersRows(t *testing.T) {
	s, _, _ := newFixture()
	got, err := s.Desired(context.Background(), "cell-0", 3, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Services) != 1 || got.Services[0].ID != "svc_b" {
		t.Fatalf("since_generation=3 should return only svc_b (gen 7); got %+v", got.Services)
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

// Exactly one of N concurrent identical writebacks applies the status edge.
//
// DETERMINISTIC, not opportunistic. An N-way barrier inside GetService holds
// every caller until all N have read the row, so all N observe
// status="provisioning", all N clear Writeback's `rep.Status != svc.Status`
// pre-check, and all N reach Transition. Exactly-once is then a property of the
// FROM-guard rather than of whichever goroutine happened to be scheduled first.
//
// Without the barrier this test degraded into near-uselessness after Q10 gave
// q.services a single owner: at CI's plain `go test ./...`, deleting the
// fixture's exactly-once guard went undetected in 97 runs out of 100. With it,
// deleting that guard yields calls==8 on every run.
//
// Errors are counted, not discarded. The sibling test in this package
// (TestIdempotencyConcurrentDoubleSubmitHasOneWinner) warns about exactly this:
// "1 owner + 1 replay + 10 errors would otherwise pass". Asserting only on
// tr.calls would let 1 winner + 7 arbitrary failures look identical to 1 winner
// + 7 clean conflict losers.
func TestConcurrentWritebackAppliesOnce(t *testing.T) {
	const n = 8
	s, q, tr := newFixture()

	// The barrier. Fails loudly rather than hanging for the package timeout if
	// fewer than n callers ever arrive.
	var arrived atomic.Int64
	release := make(chan struct{})
	stuck := make(chan struct{})
	q.afterRead = func() {
		if arrived.Add(1) == n {
			close(release)
		}
		select {
		case <-release:
		case <-stuck:
			t.Errorf("only %d of %d callers reached GetService — the barrier never released", arrived.Load(), n)
		}
	}

	var mu sync.Mutex
	winners, losers := 0, 0
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			_, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3, Status: "ready"})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				winners++
			} else {
				losers++
			}
		})
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		close(stuck)
		<-done
		t.Fatal("concurrent writeback deadlocked")
	}

	if tr.calls != 1 {
		t.Fatalf("concurrent writebacks applied the edge %d times, want exactly 1", tr.calls)
	}
	if winners != 1 || losers != n-1 {
		t.Fatalf("got %d winners and %d losers, want exactly 1 and %d — a run where six callers "+
			"errored for unrelated reasons would satisfy an assertion on tr.calls alone", winners, losers, n-1)
	}
	// The row the callers were fighting over ends in the state exactly one of
	// them drove it to. Asserting the edge count without this leaves open that
	// the edge applied once and then something undid it.
	if got := q.services["svc_a"]; got.Status != "ready" || got.ObservedGeneration != 3 {
		t.Fatalf("after %d concurrent writebacks the row is status=%q observed=%d, want ready/3",
			n, got.Status, got.ObservedGeneration)
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
	// svc_a is `provisioning`, so an observation-only ack does NOT finish the
	// generation — the cell said nothing about status and the row is still
	// mid-apply. The heartbeat must fire anyway: it runs FIRST precisely so that
	// an agent which is alive but reporting something the machine will not
	// accept still counts as seen (see Writeback's ordering note).
	_, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3})
	if !errors.Is(err, ErrNotConverged) {
		t.Fatalf("err = %v, want ErrNotConverged", err)
	}
	if q.heartbeat != 1 {
		t.Fatalf("heartbeat not recorded on an observation-only writeback (%d)", q.heartbeat)
	}
}

// AN ACK ON A SETTLED ROW FINISHES THE GENERATION; ON AN UNSETTLED ONE IT DOES
// NOT. "I applied this generation, no status to report" asserts nothing about
// where the row rests, so the row's own status decides whether anything is left
// to watch. `degraded` BILLS — parking one there unwatched bills indefinitely.
func TestAnAckFinishesAGenerationOnlyOnASettledRow(t *testing.T) {
	for _, tc := range []struct {
		status    string
		converged bool
	}{
		{"ready", true}, {"failed", true}, {"provisioning", false}, {"degraded", false},
	} {
		s, q, tr := newFixture()
		q.services["svc_a"] = store.Service{
			ID: "svc_a", CellID: "cell-0", Status: tc.status, Generation: 3, ObservedGeneration: 2,
		}
		_, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3})
		if tr.calls != 0 {
			t.Errorf("%s: an ack drove the status machine", tc.status)
		}
		advanced := q.services["svc_a"].ObservedGeneration == 3
		if advanced != tc.converged {
			t.Errorf("%s: observed_generation advanced=%v, want %v (err=%v)",
				tc.status, advanced, tc.converged, err)
		}
		if tc.converged && err != nil {
			t.Errorf("%s: %v", tc.status, err)
		}
		if !tc.converged && !errors.Is(err, ErrNotConverged) {
			t.Errorf("%s: err = %v, want ErrNotConverged — the row must stay outstanding", tc.status, err)
		}
	}
}

func TestObservationOnlyWritebackSkipsTransition(t *testing.T) {
	s, _, tr := newFixture()
	// Current generation (3), no status → observation only, no edge. svc_a is
	// `provisioning`, so it does not converge (see the ack test above); what is
	// asserted here is that it never touches the status machine either way.
	if _, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3}); !errors.Is(err, ErrNotConverged) {
		t.Fatalf("err = %v, want ErrNotConverged", err)
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
	for _, d := range got.Services {
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

// The stranding regression both reviewers flagged: if the status edge fails,
// observed_generation must NOT advance, so the row stays outstanding and the
// next tick retries it. The reverse order lost the edge forever.
func TestFailedTransitionDoesNotAdvanceObserved(t *testing.T) {
	s, q, tr := newFixture()
	tr.failTo = "ready" // simulate an illegal edge / mid-request failure
	_, err := s.Writeback(context.Background(), "cell-0", Report{ServiceID: "svc_a", ObservedGeneration: 3, Status: "ready"})
	if err == nil {
		t.Fatal("a failing transition must surface an error")
	}
	if q.services["svc_a"].ObservedGeneration != 0 {
		t.Fatalf("observed_generation advanced to %d despite a failed transition — the row is stranded",
			q.services["svc_a"].ObservedGeneration)
	}
	if q.services["svc_a"].Status != "provisioning" {
		t.Fatal("status changed despite a failed transition")
	}
}

// A quiescent cell (no outstanding work) still heartbeats via the poll, so a
// health check does not call a healthy cell dead.
func TestPollTouchesHeartbeat(t *testing.T) {
	s, q, _ := newFixture()
	before := q.heartbeat
	if _, err := s.Desired(context.Background(), "cell-0", 0, 100); err != nil {
		t.Fatal(err)
	}
	if q.heartbeat != before+1 {
		t.Fatalf("the poll must touch the heartbeat (quiescent-cell liveness); count %d→%d", before, q.heartbeat)
	}
}

// THE WRITEBACK MUST CONSULT THE MACHINE, AND MUST HONOUR "not converged".
//
// The type stops the convergence signal being DISCARDED (there is no second
// return value to drop with `_`), but nothing in the compiler forces a caller to
// ASK. This pins the call site: it is the representation that decides, and the
// one US-3.3a round 12 got wrong.
//
// A cluster that broke while READY: the agent reports `failed`, which ADR-024
// does not allow from `ready`, so the raw report would 409 every tick. The
// mapped answer is `degraded` — and it is NOT converged, because `degraded`
// still bills and `degraded → failed` is the only edge that closes the span.
func TestWritebackMapsTheReportThroughTheStatusMachine(t *testing.T) {
	s, q, _ := newFixture()
	q.services["svc_r"] = store.Service{
		ID: "svc_r", CellID: "cell-0", Status: "ready", Generation: 5, ObservedGeneration: 4,
	}
	_, err := s.Writeback(context.Background(), "cell-0", Report{
		ServiceID: "svc_r", ObservedGeneration: 5, Status: "failed",
	})
	if !errors.Is(err, ErrNotConverged) {
		t.Fatalf("err = %v, want ErrNotConverged", err)
	}
	if got := q.services["svc_r"].Status; got != "degraded" {
		t.Errorf("a READY service reported failed ended as %q, want degraded — the raw report was "+
			"used instead of the mapped one", got)
	}
	if got := q.services["svc_r"].ObservedGeneration; got == 5 {
		t.Fatal("observed_generation advanced: the row leaves the outstanding set at `degraded`, " +
			"which BILLS, and nothing observes the cluster again")
	}

	// Still broken on the next tick -> `failed`, which is the hop that closes the
	// metering span. This is the leg that was unreachable when the first hop was
	// marked converged.
	if _, err := s.Writeback(context.Background(), "cell-0", Report{
		ServiceID: "svc_r", ObservedGeneration: 5, Status: "failed",
	}); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if got := q.services["svc_r"].Status; got != "failed" {
		t.Errorf("after the second tick status = %q, want failed — the span never closes", got)
	}
	if got := q.services["svc_r"].ObservedGeneration; got != 5 {
		t.Errorf("observed_generation = %d, want 5 once converged", got)
	}
}

// ...and a transient blip takes the other branch of the same two-hop path.
func TestARecoveredBlipReturnsToReadyWithoutPassingThroughFailed(t *testing.T) {
	s, q, _ := newFixture()
	q.services["svc_t"] = store.Service{
		ID: "svc_t", CellID: "cell-0", Status: "ready", Generation: 2, ObservedGeneration: 1,
	}
	if _, err := s.Writeback(context.Background(), "cell-0", Report{
		ServiceID: "svc_t", ObservedGeneration: 2, Status: "failed",
	}); !errors.Is(err, ErrNotConverged) {
		t.Fatalf("first tick err = %v, want ErrNotConverged", err)
	}
	if _, err := s.Writeback(context.Background(), "cell-0", Report{
		ServiceID: "svc_t", ObservedGeneration: 2, Status: "ready",
	}); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if got := q.services["svc_t"].Status; got != "ready" {
		t.Errorf("a recovered blip ended at %q, want ready", got)
	}
	if got := q.services["svc_t"].ObservedGeneration; got != 2 {
		t.Errorf("observed_generation = %d, want 2", got)
	}
}

// AND AN UNSETTLED HOP MUST NOT ADVANCE OBSERVATION. `failed` + a healthy
// cluster routes through `provisioning` and needs a second tick; advancing here
// strands the row at `provisioning` forever, because ListDesiredForCell selects
// on observed_generation < generation.
func TestWritebackLeavesAnUnconvergedRowOutstanding(t *testing.T) {
	s, q, _ := newFixture()
	q.services["svc_f"] = store.Service{
		ID: "svc_f", CellID: "cell-0", Status: "failed", Generation: 9, ObservedGeneration: 8,
	}
	_, err := s.Writeback(context.Background(), "cell-0", Report{
		ServiceID: "svc_f", ObservedGeneration: 9, Status: "ready",
	})
	if !errors.Is(err, ErrNotConverged) {
		t.Fatalf("err = %v, want ErrNotConverged — the hop is not finished", err)
	}
	if got := q.services["svc_f"].Status; got != "provisioning" {
		t.Errorf("status = %q, want provisioning — the legal edge should still have been taken", got)
	}
	if got := q.services["svc_f"].ObservedGeneration; got == 9 {
		t.Fatal("observed_generation ADVANCED on an unconverged hop: the row leaves the " +
			"outstanding set at `provisioning` and never reaches ready")
	}

	// The next tick finishes it, which is what makes leaving it outstanding correct.
	if _, err := s.Writeback(context.Background(), "cell-0", Report{
		ServiceID: "svc_f", ObservedGeneration: 9, Status: "ready",
	}); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if got := q.services["svc_f"].Status; got != "ready" {
		t.Errorf("after the second tick status = %q, want ready", got)
	}
	if got := q.services["svc_f"].ObservedGeneration; got != 9 {
		t.Errorf("observed_generation = %d, want 9 once converged", got)
	}
}

// A HELD ROW IS NEITHER RESUMED NOR FINISHED BY A CONTRADICTING REPORT.
//
// Not resumed: `suspended → ready` is a legal edge, so a converging agent that
// sees a healthy cluster would otherwise silently un-suspend the service and
// restart its metering span.
//
// And not FINISHED, which is the half that costs a teardown. `DeleteService`
// bumps the generation and only then transitions to `deleting`, so a deleting
// row is outstanding by construction — that is what redelivers the
// `deleting:true` desired doc. One `ready` report from a cell that has torn
// nothing down would advance observed_generation, drop the row out of
// ListDesiredForCell, and leave the cluster running forever while DeleteService
// answers "deletion already in progress".
func TestAHeldRowIsNeitherResumedNorFinishedByAContradictingReport(t *testing.T) {
	for _, from := range []string{"suspended", "deleting"} {
		for _, observed := range []string{"ready", "provisioning", "degraded", "failed"} {
			s, q, tr := newFixture()
			q.services["svc_h"] = store.Service{
				ID: "svc_h", CellID: "cell-0", Status: from, Generation: 4, ObservedGeneration: 3,
			}
			_, err := s.Writeback(context.Background(), "cell-0", Report{
				ServiceID: "svc_h", ObservedGeneration: 4, Status: observed,
			})
			if !errors.Is(err, ErrNotConverged) {
				t.Errorf("%s+%s: err = %v, want ErrNotConverged — the hold was never observed to "+
					"take effect, so the row must stay outstanding", from, observed, err)
			}
			if got := q.services["svc_h"].Status; got != from {
				t.Errorf("%s+%s: an observation moved a held service to %q", from, observed, got)
			}
			if got := q.services["svc_h"].ObservedGeneration; got == 4 {
				t.Errorf("%s+%s: observed_generation advanced — the row leaves the outstanding "+
					"set, so the desired doc that carries the hold is never redelivered",
					from, observed)
			}
			if tr.calls != 0 {
				t.Errorf("%s+%s: a held row drove the status machine", from, observed)
			}
		}
	}
}

// ...and `gone` IS the evidence that finishes it: the hold took effect.
func TestAHeldRowIsFinishedByAConfirmedTeardown(t *testing.T) {
	for _, from := range []string{"suspended", "deleting"} {
		s, q, tr := newFixture()
		q.services["svc_g"] = store.Service{
			ID: "svc_g", CellID: "cell-0", Status: from, Generation: 4, ObservedGeneration: 3,
		}
		if _, err := s.Writeback(context.Background(), "cell-0", Report{
			ServiceID: "svc_g", ObservedGeneration: 4, Status: "gone",
		}); err != nil {
			t.Errorf("%s+gone: %v", from, err)
		}
		if got := q.services["svc_g"].Status; got != from {
			t.Errorf("%s+gone: status moved to %q — `gone` is never a status edge", from, got)
		}
		if got := q.services["svc_g"].ObservedGeneration; got != 4 {
			t.Errorf("%s+gone: observed_generation = %d, want 4 — a confirmed teardown finishes "+
				"the generation, or the agent re-issues Delete every tick forever", from, got)
		}
		if tr.calls != 0 {
			t.Errorf("%s+gone: drove the status machine", from)
		}
	}
}

// AN UNCONVERGED HOP IS A 409, NOT A 500.
//
// It is a NORMAL event — the machine took a legal edge and needs one more
// converge. Without an arm in writeErr it fell to problem.Internal: a 500 the
// OpenAPI contract does not declare for this route, an ERROR boundary log, and a
// remediation telling the operator to "contact support with the event id" — an
// id that is never minted. Every failed-service recovery in the fleet would emit
// a control-plane 5xx.
func TestAnUnconvergedHopIsAConflictNotAnInternalError(t *testing.T) {
	s, q, _ := newFixture()
	q.services["svc_w"] = store.Service{
		ID: "svc_w", CellID: "cell-0", Status: "failed", Generation: 4, ObservedGeneration: 3,
	}
	h := &Handlers{svc: s}
	rec := httptest.NewRecorder()
	h.writeErr(rec, httptest.NewRequest(http.MethodPost, "/v1/reconcile/cell-0/status", nil),
		fmt.Errorf("%w: svc_w needs another hop from failed", ErrNotConverged))

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409. A 500 here is an undeclared response, an ERROR log and a "+
			"remediation naming an event id that is never minted — for an expected event.", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Errorf("content-type = %q, want application/problem+json", ct)
	}
	body := rec.Body.String()
	if strings.Contains(body, "contact support") {
		t.Errorf("the remediation still tells the operator to contact support: %s", body)
	}
	if !strings.Contains(body, "remediation") || strings.Contains(body, `"remediation":""`) {
		t.Errorf("no remediation on the refusal: %s", body)
	}
}

// A REPORT THE MACHINE CANNOT PLACE LEAVES THE ROW OUTSTANDING.
//
// `provisioning` + `degraded` has no legal edge (ADR-024 allows
// provisioning → {ready, failed, deleting}). It used to answer "no change,
// converged": observed_generation advanced, the row left the outstanding set at
// `provisioning`, and the agent never converged it again — no error, nothing
// visible, the mirror image of the 409 loop this task exists to remove.
//
// The mid-apply pair is the same shape from the other side: `ready` +
// `provisioning` is what a cell reports during an in-place upgrade, and settling
// it declares a generation done in the middle of the apply.
func TestAReportTheMachineCannotPlaceLeavesTheRowOutstanding(t *testing.T) {
	for _, tc := range []struct{ from, observed string }{
		{"provisioning", "degraded"},
		{"failed", "degraded"},
		{"ready", "provisioning"},
		{"degraded", "provisioning"},
	} {
		s, q, tr := newFixture()
		q.services["svc_u"] = store.Service{
			ID: "svc_u", CellID: "cell-0", Status: tc.from, Generation: 5, ObservedGeneration: 4,
		}
		_, err := s.Writeback(context.Background(), "cell-0", Report{
			ServiceID: "svc_u", ObservedGeneration: 5, Status: tc.observed,
		})
		if !errors.Is(err, ErrNotConverged) {
			t.Errorf("%s+%s: err = %v, want ErrNotConverged", tc.from, tc.observed, err)
		}
		if got := q.services["svc_u"].Status; got != tc.from {
			t.Errorf("%s+%s: status moved to %q — there is no legal edge, so nothing may be invented",
				tc.from, tc.observed, got)
		}
		if got := q.services["svc_u"].ObservedGeneration; got == 5 {
			t.Errorf("%s+%s: observed_generation advanced — the row left the outstanding set at a "+
				"status the cell never reported and will never be reconciled again",
				tc.from, tc.observed)
		}
		if tr.calls != 0 {
			t.Errorf("%s+%s: an unplaceable report reached the status machine", tc.from, tc.observed)
		}
		// The error names both sides: it is the only trace this report leaves,
		// and it is what the agent logs and the operator reads in the 409 body.
		if !strings.Contains(err.Error(), tc.observed) || !strings.Contains(err.Error(), tc.from) {
			t.Errorf("%s+%s: the refusal names neither side: %v", tc.from, tc.observed, err)
		}
	}
}

// A ONE-HOP LANDING ON `degraded` IS THE SAME BILLING BUG AS THE TWO-HOP ONE.
//
// `ready` + `degraded` IS a legal edge, so it took the CanTransition arm and
// converged — the row left the outstanding set resting on `degraded`, which
// BILLS, with `degraded → failed` (the only edge that emits a metering `close`)
// permanently unreachable because nothing observes the cluster again.
//
// That is exactly the defect review found on `ready` + `failed`, which was fixed
// for that hop ALONE. Convergence is decided by the destination now, so both are
// covered by one rule.
func TestAServiceReportedDegradedStaysOutstanding(t *testing.T) {
	s, q, _ := newFixture()
	q.services["svc_d"] = store.Service{
		ID: "svc_d", CellID: "cell-0", Status: "ready", Generation: 6, ObservedGeneration: 5,
	}
	_, err := s.Writeback(context.Background(), "cell-0", Report{
		ServiceID: "svc_d", ObservedGeneration: 6, Status: "degraded",
	})
	if !errors.Is(err, ErrNotConverged) {
		t.Fatalf("err = %v, want ErrNotConverged", err)
	}
	if got := q.services["svc_d"].Status; got != "degraded" {
		t.Errorf("status = %q, want degraded — the legal edge should still have been taken", got)
	}
	if got := q.services["svc_d"].ObservedGeneration; got == 6 {
		t.Fatal("observed_generation advanced at `degraded`: the row rests on a BILLING state " +
			"that nothing will observe again, so it bills indefinitely")
	}
	// The next tick resolves it either way, which is what makes staying outstanding correct.
	if _, err := s.Writeback(context.Background(), "cell-0", Report{
		ServiceID: "svc_d", ObservedGeneration: 6, Status: "failed",
	}); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if got := q.services["svc_d"].Status; got != "failed" {
		t.Errorf("after the second tick status = %q, want failed — the span never closes", got)
	}
	if got := q.services["svc_d"].ObservedGeneration; got != 6 {
		t.Errorf("observed_generation = %d, want 6 once converged", got)
	}
}

// CONCURRENCY CANNOT PARK A ROW ON AN UNSETTLED STATUS.
//
// The pre-existing concurrency test covers the CONVERGED provisioning → ready
// path, where the edge and the advance happen together. Here the first hop is
// deliberately unconverged, so they come apart.
//
// IT USES THE FIXTURE'S BARRIER, and that is not decoration. Without it the test
// does not reliably race: q.mu serialises GetService against Transition, so most
// callers read the already-advanced status and never enter the dangerous window
// at all — measured at 20 runs under -race with the fakeTrans FROM-guard
// deleted, 17 went undetected. The barrier releases all n callers only once all
// n have READ, so every one of them holds from="failed" and enters Transition,
// which is the interleaving that makes the guard load-bearing.
func TestConcurrentReportsNeverAdvanceObservationOnAnUnsettledRow(t *testing.T) {
	const n = 8
	s, q, _ := newFixture()
	q.services["svc_c"] = store.Service{
		ID: "svc_c", CellID: "cell-0", Status: "failed", Generation: 3, ObservedGeneration: 2,
	}

	var arrived atomic.Int64
	release := make(chan struct{})
	stuck := make(chan struct{})
	t.Cleanup(func() { close(stuck) })
	q.afterRead = func() {
		if arrived.Add(1) == n {
			close(release)
		}
		select {
		case <-release:
		case <-stuck:
			t.Errorf("only %d of %d callers reached GetService — the barrier never released",
				arrived.Load(), n)
		}
	}

	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = s.Writeback(context.Background(), "cell-0", Report{
				ServiceID: "svc_c", ObservedGeneration: 3, Status: "ready",
			})
		}(i)
	}
	wg.Wait()

	if got := arrived.Load(); got != n {
		t.Fatalf("only %d of %d callers read — the interleaving under test never happened", got, n)
	}
	final := q.services["svc_c"].Status
	advanced := q.services["svc_c"].ObservedGeneration == 3

	// Every caller holds from="failed", so every edge taken is failed →
	// provisioning, and `provisioning` is unsettled: nothing may finish here.
	if final != "provisioning" {
		t.Fatalf("status = %q, want provisioning — all %d callers read `failed`, and the only "+
			"edge from there for a healthy report is failed → provisioning", final, n)
	}
	if advanced {
		t.Fatal("observed_generation advanced while the row rests at `provisioning` — the row " +
			"leaves the outstanding set mid-apply and never reaches ready")
	}
	// Exactly one caller may take the edge; the rest lose the FROM-guard race,
	// which is a different and correct refusal. Nobody may report success.
	notConverged, conflicts := 0, 0
	for i, err := range errs {
		switch {
		case err == nil:
			t.Fatalf("caller %d reported success while the row is still outstanding", i)
		case errors.Is(err, ErrNotConverged):
			notConverged++
		case isConflict(err):
			conflicts++
		default:
			t.Fatalf("caller %d got an unexpected error: %v", i, err)
		}
	}
	if notConverged != 1 {
		t.Errorf("%d callers took the edge and saw ErrNotConverged, want exactly 1 "+
			"(the other %d must lose the FROM-guard race)", notConverged, n-1)
	}
	if notConverged+conflicts != n {
		t.Errorf("accounted for %d of %d callers", notConverged+conflicts, n)
	}
}

// A STALE READ MAY NOT WALK A FINISHED ROW BACKWARDS — driven deterministically,
// because hoping for the interleaving does not work.
//
// The barrier above forces every caller to read the SAME status, so all of them
// compute the same destination and fakeTrans's "already there" arm rejects the
// duplicates: that version is blind to the FROM-guard. An unsynchronised
// free-for-all is barely better — q.mu serialises GetService against Transition,
// so with the guard deleted a 40-attempt racing version detected it in only
// 1 run in 10.
//
// So this stages the exact window instead. Caller A reads `failed` and is HELD.
// B then runs to completion (failed → provisioning, unconverged), then C
// (provisioning → ready, converged — observation advances). Only then is A
// released, still holding its stale `failed` read and still heading for
// `provisioning`.
//
// The store refuses A: SetServiceStatus is `WHERE id = $1 AND status = $2`, so a
// caller whose from no longer matches writes nothing. Without that guard A walks
// a FINISHED row back to `provisioning` while observed_generation stays advanced
// — the row is out of the outstanding set, resting mid-apply, and nothing will
// ever converge it again.
func TestAStaleReadCannotWalkAFinishedRowBackwards(t *testing.T) {
	s, q, _ := newFixture()
	q.services["svc_x"] = store.Service{
		ID: "svc_x", CellID: "cell-0", Status: "failed", Generation: 3, ObservedGeneration: 2,
	}

	var held atomic.Bool
	hasRead := make(chan struct{})
	release := make(chan struct{})
	q.afterRead = func() {
		if held.CompareAndSwap(false, true) {
			close(hasRead) // A has its stale read of `failed`
			<-release
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = s.Writeback(context.Background(), "cell-0", Report{
			ServiceID: "svc_x", ObservedGeneration: 3, Status: "ready",
		})
	}()
	<-hasRead

	// B: failed → provisioning, and it must NOT finish the generation.
	if _, err := s.Writeback(context.Background(), "cell-0", Report{
		ServiceID: "svc_x", ObservedGeneration: 3, Status: "ready",
	}); !errors.Is(err, ErrNotConverged) {
		t.Fatalf("B: err = %v, want ErrNotConverged", err)
	}
	// C: provisioning → ready, which finishes it.
	if _, err := s.Writeback(context.Background(), "cell-0", Report{
		ServiceID: "svc_x", ObservedGeneration: 3, Status: "ready",
	}); err != nil {
		t.Fatalf("C: %v", err)
	}
	if got := q.services["svc_x"].ObservedGeneration; got != 3 {
		t.Fatalf("C did not finish the generation (observed = %d)", got)
	}

	close(release)
	wg.Wait()

	if got := q.services["svc_x"].Status; got != "ready" {
		t.Errorf("a stale read walked the row from `ready` back to %q while "+
			"observed_generation stayed advanced — the row is out of the outstanding set, "+
			"resting mid-apply, and nothing will converge it again", got)
	}
	if got := q.services["svc_x"].ObservedGeneration; got != 3 {
		t.Errorf("observed_generation = %d, want 3", got)
	}
}

// A losing caller hits Transition's from-guard, which is a different — and
// correct — refusal from "this hop did not finish".
func isConflict(err error) bool {
	return strings.Contains(err.Error(), "illegal") || strings.Contains(err.Error(), "concurrent") ||
		strings.Contains(err.Error(), "transition")
}
