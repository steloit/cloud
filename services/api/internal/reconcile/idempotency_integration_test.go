package reconcile_test

// US-3.6 / S7: idempotent mutating POSTs, against REAL Postgres. The atomic
// claim is the load-bearing part — a read-then-write would let two concurrent
// double-submits both win, which is exactly the double-provision this exists to
// prevent — so it is exercised with real concurrency on a real database.

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/idempotency"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

func TestIdempotencyReplayReturnsOriginalResponse(t *testing.T) {
	pool, q := realDB(t)
	_ = pool
	idem := idempotency.New(q, idemKEK(t))
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
	if err := idem.Complete(ctx, "usr_1", "createService", "k1", claim, 201, nil, []byte(`{"id":"svc_1"}`)); err != nil {
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
	idem := idempotency.New(q, idemKEK(t))
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
	idem := idempotency.New(q, idemKEK(t))
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
	idem := idempotency.New(q, idemKEK(t))
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
	svc := idempotency.New(q, idemKEK(t))
	ctx := context.Background()
	body := []byte(`{"name":"db"}`)

	r, claim, err := svc.Begin(ctx, "user:u1", "POST /v1/estimates", "kf", body)
	if err != nil || r != nil {
		t.Fatalf("first caller must own the key: %v %v", r, err)
	}
	// The request failed.
	if err := svc.Complete(ctx, "user:u1", "POST /v1/estimates", "kf", claim, 500, nil, []byte(`{"title":"boom"}`)); err != nil {
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
	svc := idempotency.New(q, idemKEK(t))
	ctx := context.Background()
	body := []byte(`{"name":"db"}`)

	_, claim, err := svc.Begin(ctx, "user:u2", "POST /v1/estimates", "kr", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:u2", "POST /v1/estimates", "kr", claim, 201, nil, []byte(`{"id":"est_1"}`)); err != nil {
		t.Fatal(err)
	}
	// A late non-2xx for the same key must not wipe the stored response.
	if err := svc.Complete(ctx, "user:u2", "POST /v1/estimates", "kr", claim, 500, nil, []byte(`{}`)); err != nil {
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
	svc := idempotency.New(q, idemKEK(t))
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
	svc := idempotency.New(q, idemKEK(t))
	ctx := context.Background()
	body := []byte(`{"name":"db"}`)

	_, claim, err := svc.Begin(ctx, "user:u4", "POST /v1/estimates", "kw", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:u4", "POST /v1/estimates", "kw", claim, 201, nil, []byte(`{"id":"est_9"}`)); err != nil {
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
	svc := idempotency.New(q, idemKEK(t))
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
	if err := svc.Complete(ctx, "user:f1", "POST /v1/estimates", "kfence", claimA, 500, nil, []byte(`{}`)); err != nil {
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
	if err := svc.Complete(ctx, "user:f1", "POST /v1/estimates", "kfence", claimB, 201, nil, []byte(`{"id":"real"}`)); err != nil {
		t.Fatal(err)
	}
	err = svc.Complete(ctx, "user:f1", "POST /v1/estimates", "kfence", claimA, 201, nil, []byte(`{"id":"stale"}`))
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
	svc := idempotency.New(q, idemKEK(t))
	ctx := context.Background()

	_, claim, err := svc.Begin(ctx, "user:s1", "POST /v1/estimates", "kold", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:s1", "POST /v1/estimates", "kold", claim, 201, nil, []byte(`{"id":"x"}`)); err != nil {
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
	svc := idempotency.New(q, idemKEK(t))
	ctx := context.Background()
	body := []byte(`{"name":"db"}`)

	_, claim, err := svc.Begin(ctx, "user:e1", "POST /v1/estimates", "kexp", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:e1", "POST /v1/estimates", "kexp", claim, 201, nil, []byte(`{"id":"est_1"}`)); err != nil {
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

// idemKEK reuses the package's existing test KEK constant, so the crypto path
// under test is the same one every other integration test uses.
func idemKEK(t *testing.T) secrets.KEK {
	t.Helper()
	k, err := secrets.NewEnvKEK("kek-test", testKEK)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// US-3.6a, the load-bearing guarantee: a recorded response containing
// credential material must NEVER be recoverable from the stored row.
//
// The scan checks for the raw needle AND for its base64 encodings. The sealed
// payload is JSON with a `[]byte` body, which encodes as base64 — so a scan for
// the raw bytes alone can never see the BODY, and would pass with encryption
// entirely bypassed. The webhook secret lives in the body; that is the case this
// whole ADR exists for.
func TestIdempotencyRecordedSecretIsNeverRecoverableAtRest(t *testing.T) {
	const secret = "whsec_SUPERSECRET_do_not_persist"

	for _, tc := range []struct {
		name    string
		header  http.Header
		needles []string
	}{
		{
			// The shape a REAL createWebhook produces: no Set-Cookie, so the
			// body is the only place the credential lives.
			name:    "webhook secret in the body",
			header:  http.Header{"Content-Type": {"application/json"}},
			needles: []string{secret},
		},
		{
			// The shape a REAL signup produces: the credential is in a header.
			name:    "session token in a header",
			header:  http.Header{"Set-Cookie": {"sid=SESSIONTOKEN123; HttpOnly"}},
			needles: []string{"SESSIONTOKEN123"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, q := realDB(t)
			svc := idempotency.New(q, idemKEK(t))
			ctx := context.Background()
			body := []byte(`{"url":"https://x"}`)

			_, claim, err := svc.Begin(ctx, "user:s", "POST /v1/orgs/org_1/webhooks", "kw", body)
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.Complete(ctx, "user:s", "POST /v1/orgs/org_1/webhooks", "kw", claim, 201, tc.header,
				[]byte(`{"id":"wbh_1","secret":"`+secret+`"}`)); err != nil {
				t.Fatal(err)
			}

			blob := storedRowBytes(t, pool, "kw")
			if len(tc.needles) == 0 {
				t.Fatal("no needles — this case proves nothing")
			}
			for _, needle := range tc.needles {
				assertNotRecoverable(t, blob, needle)
			}

			// And it still replays correctly.
			r, _, err := svc.Begin(ctx, "user:s", "POST /v1/orgs/org_1/webhooks", "kw", body)
			if err != nil {
				t.Fatal(err)
			}
			if r == nil || !bytes.Contains(r.Body, []byte(secret)) {
				t.Fatalf("the replay lost the secret: %+v", r)
			}
			for k, want := range tc.header {
				if got := r.Header.Get(k); got != want[0] {
					t.Fatalf("the replay lost header %s: %q want %q", k, got, want[0])
				}
			}
		})
	}
}

// storedRowBytes concatenates every column of the row that could carry
// response-derived data.
func storedRowBytes(t *testing.T, pool *pgxpool.Pool, key string) []byte {
	t.Helper()
	// A real anti-vacuity guard: the identity columns are NOT NULL, so the blob
	// is never empty even with no payload stored — checking len(blob) would
	// assert nothing. Require an actual sealed payload (16 bytes is the GCM tag
	// floor, so anything at or below it cannot be a real ciphertext).
	var ctLen int
	if err := pool.QueryRow(context.Background(),
		`SELECT coalesce(octet_length(response_ciphertext), 0) FROM idempotency_keys WHERE key = $1`, key).Scan(&ctLen); err != nil {
		t.Fatal(err)
	}
	if ctLen <= 16 {
		t.Fatalf("stored ciphertext is %d bytes — no payload was recorded, so this scan proves nothing", ctLen)
	}
	var blob []byte
	if err := pool.QueryRow(context.Background(), `
		SELECT coalesce(response_ciphertext, ''::bytea)
		     || coalesce(response_nonce, ''::bytea)
		     || coalesce(response_wrapped_dek, ''::bytea)
		     || coalesce(response_dek_nonce, ''::bytea)
		     || convert_to(coalesce(response_kek_id, ''), 'UTF8')
		     || convert_to(principal || endpoint || key || body_sha256 || claim_token, 'UTF8')
		FROM idempotency_keys WHERE key = $1`, key).Scan(&blob); err != nil {
		t.Fatal(err)
	}
	return blob
}

// assertNotRecoverable fails if the needle appears in the stored bytes either
// literally or base64-encoded. Base64 matters because the sealed payload is
// JSON with a []byte body: a bypassed seal stores the body as base64, which a
// literal scan cannot see. All three alignments are checked because the needle
// may start at any offset within the encoded body; the fuzzy edges are trimmed
// since only the interior is alignment-stable.
func assertNotRecoverable(t *testing.T, blob []byte, needle string) {
	t.Helper()
	if bytes.Contains(blob, []byte(needle)) {
		t.Fatalf("%q appears in the stored row in PLAINTEXT — a recorded response must be envelope-encrypted", needle)
	}
	// The base64 check needs a long enough needle to have a meaningful
	// interior after trimming the alignment-fuzzy edges; an empty interior
	// would match everything and assert nothing.
	const minInterior = 8
	for off := range 3 {
		enc := base64.StdEncoding.EncodeToString(append(bytes.Repeat([]byte{'x'}, off), needle...))
		// Length-check BEFORE slicing: a needle under 4 bytes would panic on
		// enc[4:len(enc)-4] before this guard could explain itself, which is
		// exactly the case the guard exists for.
		if len(enc) < 8+minInterior {
			t.Fatalf("needle %q is too short to test meaningfully (base64 is %d chars, need %d)", needle, len(enc), 8+minInterior)
		}
		interior := enc[4 : len(enc)-4]
		if bytes.Contains(blob, []byte(interior)) {
			t.Fatalf("%q is recoverable from the stored row as BASE64 (alignment %d) — the payload was not sealed", needle, off)
		}
	}
}

// The AAD binds a record to its exact (principal, endpoint, key). A row copied
// to another key must fail to open rather than decrypt into someone else's
// response — and an unreadable record is treated as ABSENT, so the caller
// simply re-claims instead of receiving a 500 or, worse, the wrong body.
func TestIdempotencyRecordCopiedToAnotherKeyDoesNotDecrypt(t *testing.T) {
	pool, q := realDB(t)
	svc := idempotency.New(q, idemKEK(t))
	ctx := context.Background()
	body := []byte(`{"a":1}`)

	_, claim, err := svc.Begin(ctx, "user:a", "POST /v1/estimates", "korig", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:a", "POST /v1/estimates", "korig", claim, 201, nil, []byte(`{"id":"est_A"}`)); err != nil {
		t.Fatal(err)
	}
	// Copy the sealed payload onto a DIFFERENT key for the same principal.
	if _, err := pool.Exec(ctx, `
		INSERT INTO idempotency_keys (principal, endpoint, key, body_sha256, claim_token, status_code,
			response_ciphertext, response_nonce, response_wrapped_dek, response_dek_nonce, response_kek_id)
		SELECT principal, endpoint, 'kstolen', body_sha256, claim_token, status_code,
			response_ciphertext, response_nonce, response_wrapped_dek, response_dek_nonce, response_kek_id
		FROM idempotency_keys WHERE key = 'korig'`); err != nil {
		t.Fatal(err)
	}

	r, claim2, err := svc.Begin(ctx, "user:a", "POST /v1/estimates", "kstolen", body)
	if err != nil {
		t.Fatalf("an unreadable record must not surface as an error: %v", err)
	}
	if r != nil && len(r.Body) > 0 {
		t.Fatalf("a record copied to another key DECRYPTED: %s — the AAD is not binding", r.Body)
	}
	if claim2 == "" {
		t.Fatal("the caller should have taken ownership of the unusable key")
	}
}

// A record sealed under a retired KEK must degrade to "expired", not to a 500.
// This is a TTL-bounded replay cache, not authoritative state: the honest
// failure is "claim it again", which the caller already handles.
func TestIdempotencyRecordUnderARetiredKEKIsTreatedAsExpired(t *testing.T) {
	_, q := realDB(t)
	ctx := context.Background()
	body := []byte(`{"a":1}`)

	old := idempotency.New(q, idemKEK(t))
	_, claim, err := old.Begin(ctx, "user:r", "POST /v1/estimates", "krot", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Complete(ctx, "user:r", "POST /v1/estimates", "krot", claim, 201, nil, []byte(`{"id":"est_R"}`)); err != nil {
		t.Fatal(err)
	}

	// The KEK rotates: a new id AND new key material, because that is what a
	// real rotation is, and it is the shape a KMS KEK (ADR-0013's named
	// successor) presents.
	//
	// What this does NOT pin, stated so nobody leans on it: `secrets.Open`'s
	// early `KEKID != kek.ID()` check. `EnvKEK.Wrap`/`Unwrap` pass the key id
	// as the GCM AAD, so `Unwrap` rejects a foreign record on its own — on the
	// id under a same-material rotation, on the cipher under this one. Deleting
	// that early check would change only the error message. Pinning it needs a
	// KEK fake whose Unwrap ignores the id; no test at this level can do it.
	rotatedKEK, err := secrets.NewEnvKEK("kek-rotated",
		base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	rotated := idempotency.New(q, rotatedKEK)
	r, claim2, err := rotated.Begin(ctx, "user:r", "POST /v1/estimates", "krot", body)
	if err != nil {
		t.Fatalf("a record under a retired KEK must not error: %v", err)
	}
	if r != nil && len(r.Body) > 0 {
		t.Fatalf("a record wrapped by a retired KEK was replayed: %s", r.Body)
	}
	if claim2 == "" {
		t.Fatal("the caller should have re-claimed the key rather than being stranded")
	}
}

// The discard's `status_code IS NOT NULL` guard: an in-flight row must survive
// even when the caller holds its exact claim token. Redundant with the
// claim_token fence today, so it is pinned rather than merely asserted in a
// comment.
func TestIdempotencyDiscardNeverRemovesAnInFlightClaim(t *testing.T) {
	pool, q := realDB(t)
	svc := idempotency.New(q, idemKEK(t))
	ctx := context.Background()

	_, claim, err := svc.Begin(ctx, "user:i", "POST /v1/estimates", "kinflight", []byte(`{"a":1}`))
	if err != nil || claim == "" {
		t.Fatalf("expected ownership: %v", err)
	}
	n, err := q.DiscardUnreadableIdempotencyKey(ctx, store.DiscardUnreadableIdempotencyKeyParams{
		Principal: "user:i", Endpoint: "POST /v1/estimates", Key: "kinflight", ClaimToken: claim,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("the discard removed %d in-flight claim(s) — a request still running lost its claim", n)
	}
	var alive int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM idempotency_keys WHERE key='kinflight'`).Scan(&alive); err != nil {
		t.Fatal(err)
	}
	if alive != 1 {
		t.Fatal("the in-flight row was deleted")
	}
}

// The discard must be FENCED to the record it actually failed to open. Without
// the fence: C reads an unreadable row; A discards it first, re-claims, runs the
// handler and records a fresh READABLE response; C's unfenced delete then wipes
// A's valid record and C re-executes — a second service and a second billing
// span, the exact double-provision every other statement here is fenced against.
func TestIdempotencyDiscardCannotWipeASuccessorsValidRecord(t *testing.T) {
	pool, q := realDB(t)
	svc := idempotency.New(q, idemKEK(t))
	ctx := context.Background()
	body := []byte(`{"a":1}`)
	const (
		principal = "user:d1"
		endpoint  = "POST /v1/estimates"
		key       = "kdiscard"
	)

	// An unreadable record exists (simulating a retired KEK), and C has already
	// read it — capturing its claim_token — but has not yet discarded it.
	_, claimC, err := svc.Begin(ctx, principal, endpoint, key, body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, principal, endpoint, key, claimC, 201, nil, []byte(`{"id":"stale"}`)); err != nil {
		t.Fatal(err)
	}
	var staleToken string
	if err := pool.QueryRow(ctx, `SELECT claim_token FROM idempotency_keys WHERE key=$1`, key).Scan(&staleToken); err != nil {
		t.Fatal(err)
	}

	// A gets there first: discards, re-claims, and records a fresh response.
	if _, err := q.DiscardUnreadableIdempotencyKey(ctx, store.DiscardUnreadableIdempotencyKeyParams{
		Principal: principal, Endpoint: endpoint, Key: key, ClaimToken: staleToken,
	}); err != nil {
		t.Fatal(err)
	}
	_, claimA, err := svc.Begin(ctx, principal, endpoint, key, body)
	if err != nil || claimA == "" {
		t.Fatalf("A should own the key after the discard: %v", err)
	}
	if err := svc.Complete(ctx, principal, endpoint, key, claimA, 201, nil, []byte(`{"id":"fresh"}`)); err != nil {
		t.Fatal(err)
	}

	// C now runs its (late) discard with the STALE token. It must match nothing.
	n, err := q.DiscardUnreadableIdempotencyKey(ctx, store.DiscardUnreadableIdempotencyKeyParams{
		Principal: principal, Endpoint: endpoint, Key: key, ClaimToken: staleToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a late discard removed %d row(s) — it wiped the successor's valid record, so the request will execute twice", n)
	}

	// A's record must still replay.
	r, claim, err := svc.Begin(ctx, principal, endpoint, key, body)
	if err != nil {
		t.Fatal(err)
	}
	if claim != "" || r == nil || string(r.Body) != `{"id":"fresh"}` {
		t.Fatalf("the successor's record did not survive: r=%+v claim=%q", r, claim)
	}
}

// The AAD binds all THREE components. Only the `key` half was pinned, so
// dropping `principal` or `endpoint` from aadFor went unnoticed — and a record
// moved across TENANTS is the higher-severity half.
func TestIdempotencyRecordCopiedAcrossPrincipalOrEndpointDoesNotDecrypt(t *testing.T) {
	for _, tc := range []struct{ name, col, newVal, beginPrincipal, beginEndpoint string }{
		{"another principal", "principal", "user:b", "user:b", "POST /v1/estimates"},
		{"another endpoint", "endpoint", "POST /v1/envs/env_2/services", "user:a", "POST /v1/envs/env_2/services"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, q := realDB(t)
			svc := idempotency.New(q, idemKEK(t))
			ctx := context.Background()
			body := []byte(`{"a":1}`)

			_, claim, err := svc.Begin(ctx, "user:a", "POST /v1/estimates", "korig", body)
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.Complete(ctx, "user:a", "POST /v1/estimates", "korig", claim, 201, nil, []byte(`{"id":"est_A"}`)); err != nil {
				t.Fatal(err)
			}
			// Copy the sealed payload onto a row with a DIFFERENT identity.
			if _, err := pool.Exec(ctx, `
				INSERT INTO idempotency_keys (principal, endpoint, key, body_sha256, claim_token, status_code,
					response_ciphertext, response_nonce, response_wrapped_dek, response_dek_nonce, response_kek_id)
				SELECT `+map[string]string{"principal": "$1", "endpoint": "principal"}[tc.col]+`,
				       `+map[string]string{"principal": "endpoint", "endpoint": "$1"}[tc.col]+`,
				       key, body_sha256, claim_token, status_code,
					response_ciphertext, response_nonce, response_wrapped_dek, response_dek_nonce, response_kek_id
				FROM idempotency_keys WHERE key = 'korig'`, tc.newVal); err != nil {
				t.Fatal(err)
			}

			r, claim2, err := svc.Begin(ctx, tc.beginPrincipal, tc.beginEndpoint, "korig", body)
			if err != nil {
				t.Fatalf("an unreadable record must not surface as an error: %v", err)
			}
			if r != nil && len(r.Body) > 0 {
				t.Fatalf("a record copied to %s DECRYPTED: %s — the AAD does not bind that component", tc.name, r.Body)
			}
			if claim2 == "" {
				t.Fatal("the caller should have taken ownership of the unusable key")
			}
		})
	}
}

// Re-claiming an expired key must ERASE the old sealed payload. Otherwise the
// re-claim resets created_at while a stale credential ciphertext rides along,
// so the sweeper never reaches it and the 24h retention promise is broken.
func TestIdempotencyReclaimErasesTheOldSealedResponse(t *testing.T) {
	pool, q := realDB(t)
	svc := idempotency.New(q, idemKEK(t))
	ctx := context.Background()
	body := []byte(`{"a":1}`)

	_, claim, err := svc.Begin(ctx, "user:w", "POST /v1/estimates", "kwipe", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:w", "POST /v1/estimates", "kwipe", claim, 201, nil, []byte(`{"id":"est_W"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE idempotency_keys SET created_at = now() - interval '25 hours' WHERE key = 'kwipe'`); err != nil {
		t.Fatal(err)
	}
	if _, claim2, err := svc.Begin(ctx, "user:w", "POST /v1/estimates", "kwipe", body); err != nil || claim2 == "" {
		t.Fatalf("the expired key should be re-claimable: claim=%q err=%v", claim2, err)
	}

	var ct, nonce, dek, dekNonce []byte
	var kekID *string
	if err := pool.QueryRow(ctx, `
		SELECT response_ciphertext, response_nonce, response_wrapped_dek, response_dek_nonce, response_kek_id
		FROM idempotency_keys WHERE key = 'kwipe'`).Scan(&ct, &nonce, &dek, &dekNonce, &kekID); err != nil {
		t.Fatal(err)
	}
	if ct != nil || nonce != nil || dek != nil || dekNonce != nil || kekID != nil {
		t.Fatal("re-claiming an expired key left the old sealed payload behind — its 24h clock is now reset and the sweeper will never reach it")
	}
}

// A row marked complete with NO payload must be discarded, not replayed as a
// status with an empty body stamped Idempotent-Replay — a lie sustained for 24h.
func TestIdempotencyCompleteRecordWithNoPayloadIsDiscarded(t *testing.T) {
	pool, q := realDB(t)
	svc := idempotency.New(q, idemKEK(t))
	ctx := context.Background()
	body := []byte(`{"a":1}`)

	_, claim, err := svc.Begin(ctx, "user:n", "POST /v1/estimates", "knop", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, "user:n", "POST /v1/estimates", "knop", claim, 201, nil, []byte(`{"id":"x"}`)); err != nil {
		t.Fatal(err)
	}
	// The shape a down/up migration cycle produces: complete, but no payload.
	if _, err := pool.Exec(ctx, `
		UPDATE idempotency_keys SET response_ciphertext=NULL, response_nonce=NULL,
			response_wrapped_dek=NULL, response_dek_nonce=NULL, response_kek_id=NULL
		WHERE key='knop'`); err != nil {
		t.Fatal(err)
	}

	r, claim2, err := svc.Begin(ctx, "user:n", "POST /v1/estimates", "knop", body)
	if err != nil {
		t.Fatal(err)
	}
	if r != nil && r.StatusCode != 0 {
		t.Fatalf("a payload-less record replayed as status %d with an empty body — that is a lie held for the TTL", r.StatusCode)
	}
	if claim2 == "" {
		t.Fatal("the caller should have re-claimed the key")
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM idempotency_keys WHERE key='knop' AND status_code IS NOT NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("the unusable record was not discarded")
	}
}
