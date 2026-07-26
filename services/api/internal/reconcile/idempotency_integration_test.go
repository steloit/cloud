package reconcile_test

// US-3.6 / S7: idempotent mutating POSTs, against REAL Postgres. The atomic
// claim is the load-bearing part — a read-then-write would let two concurrent
// double-submits both win, which is exactly the double-provision this exists to
// prevent — so it is exercised with real concurrency on a real database.

import (
	"context"
	"sync"
	"testing"

	"github.com/steloit/cloud/services/api/internal/platform/idempotency"
)

func TestIdempotencyReplayReturnsOriginalResponse(t *testing.T) {
	pool, q := realDB(t)
	_ = pool
	idem := idempotency.New(q)
	ctx := context.Background()
	body := []byte(`{"name":"db","product":"postgres"}`)

	// first request owns the key
	replay, claim, err := idem.Begin(ctx, "usr_1", "createService", "k1", body)
	if err != nil {
		t.Fatal(err)
	}
	if replay != nil {
		t.Fatal("the first request must own the key, not replay")
	}
	if err := idem.Complete(ctx, "usr_1", "createService", "k1", claim, 201, []byte(`{"id":"svc_1"}`)); err != nil {
		t.Fatal(err)
	}

	// the retry replays the ORIGINAL response verbatim
	replay, _, err = idem.Begin(ctx, "usr_1", "createService", "k1", body)
	if err != nil {
		t.Fatal(err)
	}
	if replay == nil {
		t.Fatal("a repeat with the same key must replay, not re-execute (it would burn a second estimate)")
	}
	// BYTE-exact: S7 promises the original response, and the response column is
	// bytea precisely so a replay is not a re-serialization of it.
	if replay.StatusCode != 201 || string(replay.Body) != `{"id":"svc_1"}` {
		t.Fatalf("replay is not the original response verbatim: %d %s", replay.StatusCode, replay.Body)
	}
}

func TestIdempotencySameKeyDifferentBodyIsRefused(t *testing.T) {
	pool, q := realDB(t)
	_ = pool
	idem := idempotency.New(q)
	ctx := context.Background()
	if _, _, err := idem.Begin(ctx, "usr_1", "createService", "k2", []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	_, _, err := idem.Begin(ctx, "usr_1", "createService", "k2", []byte(`{"a":2}`))
	if err == nil {
		t.Fatal("the same key with a DIFFERENT body must be refused — replaying would answer the wrong question")
	}
	if !errorsIs(err, idempotency.ErrBodyMismatch) {
		t.Fatalf("want ErrBodyMismatch, got %v", err)
	}
}

// The key is scoped to (principal, endpoint, key): the same key from another
// caller, or on another endpoint, is NOT a replay.
func TestIdempotencyKeyIsScopedToPrincipalAndEndpoint(t *testing.T) {
	pool, q := realDB(t)
	_ = pool
	idem := idempotency.New(q)
	ctx := context.Background()
	body := []byte(`{"x":1}`)
	if _, _, err := idem.Begin(ctx, "usr_1", "createService", "shared", body); err != nil {
		t.Fatal(err)
	}
	// different principal — must own it, not replay
	r, _, err := idem.Begin(ctx, "usr_2", "createService", "shared", body)
	if err != nil || r != nil {
		t.Fatalf("another principal's identical key must not replay: r=%v err=%v", r, err)
	}
	// different endpoint — same
	r, _, err = idem.Begin(ctx, "usr_1", "createEstimate", "shared", body)
	if err != nil || r != nil {
		t.Fatalf("the same key on another endpoint must not replay: r=%v err=%v", r, err)
	}
}

// THE guarantee: a concurrent double-submit resolves to exactly ONE owner.
// A read-then-write implementation passes every test above and fails this one.
func TestIdempotencyConcurrentDoubleSubmitHasOneWinner(t *testing.T) {
	pool, q := realDB(t)
	_ = pool
	idem := idempotency.New(q)
	body := []byte(`{"race":true}`)

	var mu sync.Mutex
	owners, replays, errs := 0, 0, 0
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			r, _, err := idem.Begin(context.Background(), "usr_race", "createService", "krace", body)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				// Counted, not swallowed: 1 owner + 1 replay + 10 errors would
				// otherwise pass while ten callers got a 500.
				errs++
				return
			}
			if r == nil {
				owners++
			} else {
				replays++
			}
		})
	}
	wg.Wait()
	if owners != 1 {
		t.Fatalf("concurrent double-submit produced %d owners (want exactly 1) — a second owner double-provisions and burns a second estimate", owners)
	}
	if replays == 0 {
		t.Fatal("the losing requests must observe a replay, not silently proceed")
	}
	if errs != 0 {
		t.Fatalf("%d of 12 concurrent callers got an ERROR; a double-submit must resolve to owner-or-replay, never a failure", errs)
	}
	if owners+replays != 12 {
		t.Fatalf("only %d of 12 callers were accounted for", owners+replays)
	}
}

func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// A FAILED request must release its claim, or the client is stranded on a key
// it can never reuse. Proved against real SQL: the fake store in the middleware
// tests cannot show that the DELETE's `status_code IS NULL` guard is right.
func TestIdempotencyFailureReleasesTheClaim(t *testing.T) {
	_, q := realDB(t)
	svc := idempotency.New(q)
	ctx := context.Background()
	body := []byte(`{"name":"db"}`)

	r, claim, err := svc.Begin(ctx, "user:u1", "POST /v1/estimates", "kf", body)
	if err != nil || r != nil {
		t.Fatalf("first caller must own the key: %v %v", r, err)
	}
	// The request failed.
	if err := svc.Complete(ctx, "user:u1", "POST /v1/estimates", "kf", claim, 500, []byte(`{"title":"boom"}`)); err != nil {
		t.Fatal(err)
	}
	// The retry must OWN it again, not be told a request is in flight.
	r, _, err = svc.Begin(ctx, "user:u1", "POST /v1/estimates", "kf", body)
	if err != nil {
		t.Fatal(err)
	}
	if r != nil {
		t.Fatalf("a retry after a failure was refused (%+v) — the failed attempt stranded the client", r)
	}
}

// A release must NEVER destroy a recorded response: the DELETE is guarded on
// `status_code IS NULL`, so a completed key survives and keeps replaying.
func TestIdempotencyReleaseCannotDestroyARecordedResponse(t *testing.T) {
	_, q := realDB(t)
	svc := idempotency.New(q)
	ctx := context.Background()
	body := []byte(`{"name":"db"}`)

	_, claim, err := svc.Begin(ctx, "user:u2", "POST /v1/estimates", "kr", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:u2", "POST /v1/estimates", "kr", claim, 201, []byte(`{"id":"est_1"}`)); err != nil {
		t.Fatal(err)
	}
	// A late non-2xx for the same key must not wipe the stored response.
	if err := svc.Complete(ctx, "user:u2", "POST /v1/estimates", "kr", claim, 500, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	r, _, err := svc.Begin(ctx, "user:u2", "POST /v1/estimates", "kr", body)
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || r.StatusCode != 201 {
		t.Fatalf("the recorded response was destroyed by a release: %+v", r)
	}
}

// An ABANDONED claim (the process died mid-request, so no response was ever
// recorded) must become re-claimable. Holding it for the full 24h replay window
// would strand the client on a key nothing will ever complete.
func TestIdempotencyAbandonedClaimBecomesReclaimable(t *testing.T) {
	pool, q := realDB(t)
	svc := idempotency.New(q)
	ctx := context.Background()
	body := []byte(`{"name":"db"}`)

	if _, _, err := svc.Begin(ctx, "user:u3", "POST /v1/estimates", "ka", body); err != nil {
		t.Fatal(err)
	}
	// Still in flight → a duplicate must be refused, not executed.
	r, _, err := svc.Begin(ctx, "user:u3", "POST /v1/estimates", "ka", body)
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || r.StatusCode != 0 {
		t.Fatalf("a concurrent duplicate must see an in-flight claim, got %+v", r)
	}

	// Age the claim past the abandonment window (the process that held it died).
	if _, err := pool.Exec(ctx,
		`UPDATE idempotency_keys SET created_at = now() - interval '10 minutes' WHERE key = 'ka'`); err != nil {
		t.Fatal(err)
	}
	r, _, err = svc.Begin(ctx, "user:u3", "POST /v1/estimates", "ka", body)
	if err != nil {
		t.Fatal(err)
	}
	if r != nil {
		t.Fatalf("an abandoned claim was never released (%+v) — the key is stranded", r)
	}
}

// The 24h REPLAY window is not shortened by the abandonment window: a COMPLETED
// key older than 5 minutes must still replay.
func TestIdempotencyCompletedKeyStillReplaysAfterTheAbandonmentWindow(t *testing.T) {
	pool, q := realDB(t)
	svc := idempotency.New(q)
	ctx := context.Background()
	body := []byte(`{"name":"db"}`)

	_, claim, err := svc.Begin(ctx, "user:u4", "POST /v1/estimates", "kw", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:u4", "POST /v1/estimates", "kw", claim, 201, []byte(`{"id":"est_9"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE idempotency_keys SET created_at = now() - interval '3 hours' WHERE key = 'kw'`); err != nil {
		t.Fatal(err)
	}
	r, _, err := svc.Begin(ctx, "user:u4", "POST /v1/estimates", "kw", body)
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || r.StatusCode != 201 {
		t.Fatalf("a completed key stopped replaying inside the 24h window: %+v", r)
	}
}

// The owner FENCE. A request that outran the abandonment window and returns
// late must not be able to release or overwrite the claim of the successor that
// legitimately took over — that would let a third request execute concurrently
// with the successor, which is the double-provision this whole layer prevents.
func TestIdempotencyLateLoserCannotReleaseTheCurrentOwnersClaim(t *testing.T) {
	pool, q := realDB(t)
	svc := idempotency.New(q)
	ctx := context.Background()
	body := []byte(`{"name":"db"}`)

	// A claims.
	_, claimA, err := svc.Begin(ctx, "user:f1", "POST /v1/estimates", "kfence", body)
	if err != nil {
		t.Fatal(err)
	}
	// A stalls past the abandonment window; B takes the claim over.
	if _, err := pool.Exec(ctx,
		`UPDATE idempotency_keys SET created_at = now() - interval '10 minutes' WHERE key = 'kfence'`); err != nil {
		t.Fatal(err)
	}
	_, claimB, err := svc.Begin(ctx, "user:f1", "POST /v1/estimates", "kfence", body)
	if err != nil {
		t.Fatal(err)
	}
	if claimB == "" || claimB == claimA {
		t.Fatalf("B did not take over the abandoned claim (A=%q B=%q)", claimA, claimB)
	}

	// A finally returns, having failed. Its release must NOT touch B's claim.
	if err := svc.Complete(ctx, "user:f1", "POST /v1/estimates", "kfence", claimA, 500, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	r, claimC, err := svc.Begin(ctx, "user:f1", "POST /v1/estimates", "kfence", body)
	if err != nil {
		t.Fatal(err)
	}
	if claimC != "" || r == nil || r.StatusCode != 0 {
		t.Fatalf("a late loser released the CURRENT owner's claim — a third request can now run alongside B (r=%+v claim=%q)", r, claimC)
	}

	// B records its response FIRST, then A returns late with its own success.
	// The order matters: with A first and B second, last-write-wins produces
	// the right answer whether or not the fence exists, and the fence would go
	// unpinned. B-then-A is the only ordering where the stale write can win.
	if err := svc.Complete(ctx, "user:f1", "POST /v1/estimates", "kfence", claimB, 201, []byte(`{"id":"real"}`)); err != nil {
		t.Fatal(err)
	}
	err = svc.Complete(ctx, "user:f1", "POST /v1/estimates", "kfence", claimA, 201, []byte(`{"id":"stale"}`))
	if !errorsIs(err, idempotency.ErrLostClaim) {
		t.Fatalf("a fenced-out save must report losing the claim, got %v", err)
	}
	r, _, err = svc.Begin(ctx, "user:f1", "POST /v1/estimates", "kfence", body)
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || string(r.Body) != `{"id":"real"}` {
		t.Fatalf("the stale owner's response won: %+v", r)
	}
}

// The 24h window is a promise: without a sweep the table grows without bound
// and every recorded response body is retained indefinitely.
func TestIdempotencySweepRemovesExpiredEntriesOnly(t *testing.T) {
	pool, q := realDB(t)
	svc := idempotency.New(q)
	ctx := context.Background()

	_, claim, err := svc.Begin(ctx, "user:s1", "POST /v1/estimates", "kold", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:s1", "POST /v1/estimates", "kold", claim, 201, []byte(`{"id":"x"}`)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Begin(ctx, "user:s1", "POST /v1/estimates", "kfresh", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE idempotency_keys SET created_at = now() - interval '25 hours' WHERE key = 'kold'`); err != nil {
		t.Fatal(err)
	}

	n, err := q.SweepExpiredIdempotencyKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("sweep removed %d rows, want exactly 1 (the expired one)", n)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM idempotency_keys WHERE key = 'kfresh'`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatal("the sweep deleted an entry that is still inside its 24h window")
	}
}

// "Entries expire after 24h" is an acceptance criterion, and it was only
// half-pinned: both the claim's 24h branch and Get's 24h filter could be
// removed with nothing failing. After the window the key must be OWNABLE again
// — not a replay, and not an error.
func TestIdempotencyKeyIsReusableAfterTheReplayWindow(t *testing.T) {
	pool, q := realDB(t)
	svc := idempotency.New(q)
	ctx := context.Background()
	body := []byte(`{"name":"db"}`)

	_, claim, err := svc.Begin(ctx, "user:e1", "POST /v1/estimates", "kexp", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:e1", "POST /v1/estimates", "kexp", claim, 201, []byte(`{"id":"est_1"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE idempotency_keys SET created_at = now() - interval '25 hours' WHERE key = 'kexp'`); err != nil {
		t.Fatal(err)
	}

	r, claim2, err := svc.Begin(ctx, "user:e1", "POST /v1/estimates", "kexp", body)
	if err != nil {
		t.Fatalf("a key past its window must be re-claimable, not an error: %v", err)
	}
	if r != nil {
		t.Fatalf("a key past its 24h window still replayed: %+v — the window is a promise, not a suggestion", r)
	}
	if claim2 == "" {
		t.Fatal("the caller did not take ownership of the expired key")
	}
}
