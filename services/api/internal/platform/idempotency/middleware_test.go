package idempotency

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"gopkg.in/yaml.v3"

	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// TestRouteTableMatchesOpenAPI ties idempotentRoutes to the contract. openapi.yaml
// is the authority (docs/product/08-api); if an operation gains or loses the
// `idempotencyKey` parameter there and this table is not updated, the header is
// silently unenforced (or wrongly rejected) — so drift fails here rather than in
// production.
func TestRouteTableMatchesOpenAPI(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	specPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..",
		"docs", "product", "08-api", "openapi.yaml")
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	// Loosely typed on purpose: a path item mixes operations with a path-level
	// `parameters` sequence, and operations are written in both block and flow
	// style. Only real HTTP methods are inspected.
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	methods := map[string]bool{"get": true, "put": true, "post": true, "delete": true, "patch": true}

	var want []string
	for path, item := range doc.Paths {
		for method, opAny := range item {
			if !methods[method] {
				continue
			}
			op, ok := opAny.(map[string]any)
			if !ok {
				continue
			}
			// Merge PATH-ITEM-level parameters: openapi allows the parameter
			// to be declared once for every method on a path (the spec already
			// does this for `env`), and a walk that reads only operation-level
			// parameters would be blind to that declaration.
			// COPY before appending: `params` comes from the decoded
			// document, so appending in place could write into a sibling
			// operation's backing array.
			opParams, _ := op["parameters"].([]any)
			params := append([]any(nil), opParams...)
			if itemParams, ok := item["parameters"].([]any); ok {
				params = append(params, itemParams...)
			}
			for _, pAny := range params {
				pm, ok := pAny.(map[string]any)
				if !ok {
					continue
				}
				ref, _ := pm["$ref"].(string)
				if !strings.HasSuffix(ref, "/idempotencyKey") {
					continue
				}
				// openapi `{env}` is also stdlib-mux wildcard syntax.
				// Declaring the parameter at BOTH levels is legal, so dedupe
				// rather than counting the route twice.
				route := strings.ToUpper(method) + " /v1" + path
				if !slices.Contains(want, route) {
					want = append(want, route)
				}
			}
		}
	}
	if len(want) == 0 {
		t.Fatal("parsed zero idempotent operations from openapi.yaml — the test is not actually reading the contract")
	}
	// Every operation the contract declares must be accounted for: either
	// ENFORCED here, or explicitly DEFERRED with a recorded reason. An
	// operation that is in neither list would silently accept the header while
	// nothing dedupes it.
	got := append(append([]string(nil), idempotentRoutes...), deferredRoutes...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the route tables have drifted from openapi.yaml\n enforced+deferred: %v\n declared in spec: %v", got, want)
	}
	// The two lists must be disjoint — a route cannot be both enforced and
	// deferred, and an accidental duplicate would mask a missing entry above.
	for _, e := range idempotentRoutes {
		for _, d := range deferredRoutes {
			if e == d {
				t.Fatalf("%q is both enforced and deferred", e)
			}
		}
	}
}

// Deferred routes DECLARE the parameter in openapi.yaml but cannot be enforced
// yet (US-3.6a). The header must PASS THROUGH: refusing would turn a
// spec-declared optional header into a hard failure and break signup for any
// client that sets it globally. This test documents the gap in executable form
// — when US-3.6a lands it should start failing.
func TestDeferredRoutesPassTheHeaderThroughUnenforced(t *testing.T) {
	n := 0
	h, fs := newHarnessWithStore(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	for _, path := range []string{"/v1/auth/signup", "/v1/orgs/org_1/webhooks"} {
		first := post(t, h, path, "k1", `{}`)
		if first.Code != 201 {
			t.Fatalf("%s: a key must not break a declared-but-unenforced route, got %d", path, first.Code)
		}
		second := post(t, h, path, "k1", `{}`)
		if second.Header().Get("Idempotent-Replay") == "true" {
			t.Fatalf("%s is marked as replayed, but its response carries credential material that is not stored — the marker would be a lie", path)
		}
	}
	if n != 4 {
		t.Fatalf("deferred routes must execute normally; handler ran %d times, want 4", n)
	}
	// And nothing may be PERSISTED for a route we told the client we do not
	// handle — storing a response we will never replay is the worst of both.
	fs.mu.Lock()
	stored := len(fs.rows)
	fs.mu.Unlock()
	if stored != 0 {
		t.Fatalf("a deferred route persisted %d idempotency row(s)", stored)
	}
}

// --- middleware behavior, proved over real HTTP ---

// fakeStore is an in-memory stand-in for the SQL layer. The SQL semantics
// themselves (atomic claim, 24h window) are proved against real Postgres in
// internal/reconcile/idempotency_integration_test.go; these tests prove the HTTP
// contract on top.
type fakeStore struct {
	mu   sync.Mutex
	rows map[string]*fakeRow
}
type fakeRow struct {
	fingerprint string
	token       string
	status      int
	body        []byte
	done        bool
}

func newHarness(t *testing.T, h http.Handler) http.Handler {
	t.Helper()
	wrapped, _ := newHarnessWithStore(t, h)
	return wrapped
}

// newHarnessWithStore also exposes the store, for tests that must assert what
// was (or was not) persisted.
func newHarnessWithStore(t *testing.T, h http.Handler) (http.Handler, *fakeStore) {
	t.Helper()
	fs := &fakeStore{rows: map[string]*fakeRow{}}
	return Middleware(newTestService(fs), nil)(h), fs
}

func post(t *testing.T, h http.Handler, path, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	r.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// The handler must run exactly once no matter how many times a key is replayed,
// and the replay must be the ORIGINAL bytes.
func TestReplayReturnsOriginalBytesAndRunsHandlerOnce(t *testing.T) {
	calls := 0
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"svc_1"}`))
	}))

	first := post(t, h, "/v1/envs/env_1/services", "k1", `{"name":"db"}`)
	if first.Code != 201 || first.Body.String() != `{"id":"svc_1"}` {
		t.Fatalf("first response wrong: %d %s", first.Code, first.Body.String())
	}
	if first.Header().Get("Idempotent-Replay") != "" {
		t.Fatal("the FIRST response must not be marked as a replay")
	}

	second := post(t, h, "/v1/envs/env_1/services", "k1", `{"name":"db"}`)
	if calls != 1 {
		t.Fatalf("handler ran %d times — a replay must never re-execute (it would double-provision)", calls)
	}
	if second.Code != 201 || second.Body.String() != `{"id":"svc_1"}` {
		t.Fatalf("replay is not the original response: %d %s", second.Code, second.Body.String())
	}
	if second.Header().Get("Idempotent-Replay") != "true" {
		t.Fatal("a replay must be marked Idempotent-Replay: true (S7)")
	}
}

func TestSameKeyDifferentBodyIs409WithRemediation(t *testing.T) {
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"svc_1"}`))
	}))
	post(t, h, "/v1/envs/env_1/services", "k1", `{"name":"db"}`)
	got := post(t, h, "/v1/envs/env_1/services", "k1", `{"name":"OTHER"}`)
	if got.Code != http.StatusConflict {
		t.Fatalf("same key + different body must be 409, got %d", got.Code)
	}
	if !strings.Contains(got.Body.String(), "remediation") {
		t.Fatalf("every problem+json carries remediation: %s", got.Body.String())
	}
}

// The dedupe scope is the CONCRETE path. Reusing one key against a different env
// must NOT replay the first env's service — that would hand a caller a resource
// in an environment they did not ask for.
func TestKeyIsScopedToTheConcretePathNotTheRoutePattern(t *testing.T) {
	n := 0
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"env":"` + r.URL.Path + `"}`))
	}))
	a := post(t, h, "/v1/envs/env_A/services", "same", `{"name":"db"}`)
	b := post(t, h, "/v1/envs/env_B/services", "same", `{"name":"db"}`)
	if n != 2 {
		t.Fatalf("the same key on a DIFFERENT env must execute, not replay (handler ran %d times)", n)
	}
	if a.Body.String() == b.Body.String() {
		t.Fatalf("env_B received env_A's response: %s", b.Body.String())
	}
	if b.Header().Get("Idempotent-Replay") == "true" {
		t.Fatal("a different resource must never be answered as a replay")
	}
}

// A failure must leave the key claimable: otherwise a client that hit a 500 can
// never retry and its state is stranded (US-3.6).
func TestFailedResponseIsNotRecordedSoRetryIsPossible(t *testing.T) {
	n := 0
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"title":"boom"}`))
			return
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"svc_1"}`))
	}))
	if got := post(t, h, "/v1/estimates", "k1", `{"a":1}`); got.Code != 500 {
		t.Fatalf("expected the handler's 500, got %d", got.Code)
	}
	retry := post(t, h, "/v1/estimates", "k1", `{"a":1}`)
	if retry.Code != 201 {
		t.Fatalf("a retry after a failure must execute, got %d (the client is stranded)", retry.Code)
	}
	if n != 2 {
		t.Fatalf("handler ran %d times, want 2", n)
	}
}

func TestNoKeyPassesThroughUntouched(t *testing.T) {
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"svc_1"}`))
	}))
	got := post(t, h, "/v1/envs/env_1/services", "", `{"name":"db"}`)
	if got.Code != 201 || got.Header().Get("Idempotent-Replay") != "" {
		t.Fatalf("a request without a key must be untouched: %d", got.Code)
	}
}

// A key on an operation the contract does not declare idempotent must be
// REFUSED, not ignored: silently accepting it tells the client a retry is safe
// when nothing dedupes it.
func TestKeyOnUndeclaredRouteIsRefused(t *testing.T) {
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	got := post(t, h, "/v1/envs/env_1/services/svc_1/restart", "k1", `{}`)
	if got.Code != 422 {
		t.Fatalf("a key on an undeclared operation must be refused, got %d", got.Code)
	}
}

func TestOverlongKeyIsRefused(t *testing.T) {
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run for an invalid key")
	}))
	if got := post(t, h, "/v1/estimates", strings.Repeat("x", 256), `{}`); got.Code != 422 {
		t.Fatalf("a >255-char key must be refused (S7), got %d", got.Code)
	}
}

// The handler must receive the body unchanged — the middleware reads it to
// fingerprint the request, so a missing rewind would hand every handler an
// empty body.
func TestBodyIsRewoundForTheHandler(t *testing.T) {
	var seen string
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		seen = string(b)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{}`))
	}))
	post(t, h, "/v1/estimates", "k1", `{"name":"db"}`)
	if seen != `{"name":"db"}` {
		t.Fatalf("handler saw %q — the body was not rewound", seen)
	}
}

// An in-flight duplicate (claimed, not yet completed) must be refused rather
// than executed a second time.
func TestInFlightDuplicateIsRefusedNotExecuted(t *testing.T) {
	fs := &fakeStore{rows: map[string]*fakeRow{}}
	svc := newTestService(fs)
	n := 0
	h := Middleware(svc, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		// Never completes: simulates the first request still running.
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"svc_1"}`))
	}))
	// Claim the key out-of-band without completing it.
	// Use the SAME derivation the middleware does, rather than a hand-written
	// string that silently stops matching when the derivation changes.
	if _, _, err := svc.Begin(context.Background(), "anon:10.0.0.1", endpointKey(http.MethodPost, "/v1/estimates"), "k1", []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	got := post(t, h, "/v1/estimates", "k1", `{"a":1}`)
	if got.Code != http.StatusConflict {
		t.Fatalf("an in-flight duplicate must be refused (409), got %d", got.Code)
	}
	if n != 0 {
		t.Fatal("an in-flight duplicate must NOT execute the handler — that is the double-provision this prevents")
	}
}

// --- fake store: SQL semantics are proved against real Postgres elsewhere ---

func newTestService(fs *fakeStore) *Service { return New(fs) }

func (f *fakeStore) key(principal, endpoint, k string) string {
	return principal + "\x00" + endpoint + "\x00" + k
}

func (f *fakeStore) ClaimIdempotencyKey(_ context.Context, a store.ClaimIdempotencyKeyParams) (store.IdempotencyKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.key(a.Principal, a.Endpoint, a.Key)
	if _, exists := f.rows[id]; exists {
		return store.IdempotencyKey{}, pgx.ErrNoRows // someone already holds it
	}
	f.rows[id] = &fakeRow{fingerprint: a.BodySha256, token: a.ClaimToken}
	return store.IdempotencyKey{Principal: a.Principal, Endpoint: a.Endpoint, Key: a.Key, BodySha256: a.BodySha256, ClaimToken: a.ClaimToken}, nil
}

func (f *fakeStore) GetIdempotencyKey(_ context.Context, a store.GetIdempotencyKeyParams) (store.IdempotencyKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[f.key(a.Principal, a.Endpoint, a.Key)]
	if !ok {
		return store.IdempotencyKey{}, pgx.ErrNoRows
	}
	out := store.IdempotencyKey{Principal: a.Principal, Endpoint: a.Endpoint, Key: a.Key, BodySha256: row.fingerprint}
	if row.done {
		out.StatusCode = pgtype.Int4{Int32: int32(row.status), Valid: true}
		out.Response = row.body
	}
	return out, nil
}

func (f *fakeStore) SaveIdempotentResponse(ctx context.Context, a store.SaveIdempotentResponseParams) (int64, error) {
	// A real driver fails on a cancelled context; the fake must too, or the
	// client-disconnect test proves nothing.
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[f.key(a.Principal, a.Endpoint, a.Key)]
	if !ok {
		return 0, nil
	}
	if row.token != a.ClaimToken {
		return 0, nil // fenced: not the current owner
	}
	row.status, row.body, row.done = int(a.StatusCode.Int32), a.Response, true
	return 1, nil
}

func (f *fakeStore) ReleaseIdempotencyKey(ctx context.Context, a store.ReleaseIdempotencyKeyParams) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.key(a.Principal, a.Endpoint, a.Key)
	if row, ok := f.rows[id]; ok && !row.done && row.token == a.ClaimToken {
		delete(f.rows, id)
		return 1, nil
	}
	return 0, nil
}

func (f *fakeStore) SweepExpiredIdempotencyKeys(context.Context) (int64, error) { return 0, nil }

// --- gaps QA measured as unpinned ---

type stubPrincipal struct{ p session.Principal }

func (s *stubPrincipal) PrincipalFromRequest(context.Context, *http.Request) (session.Principal, bool) {
	return s.p, s.p.UserID != "" || s.p.OrgID != ""
}

// The whole point of scoping by principal is that one caller's key can never
// replay another caller's response. Every other test passes a nil resolver, so
// without this the user:/orgkey:/anon: branches never execute.
func TestKeyIsScopedToTheAuthenticatedPrincipal(t *testing.T) {
	fs := &fakeStore{rows: map[string]*fakeRow{}}
	svc := newTestService(fs)
	n := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"n":` + strconv.Itoa(n) + `}`))
	})
	send := func(p session.Principal) *httptest.ResponseRecorder {
		h := Middleware(svc, &stubPrincipal{p: p})(handler)
		r := httptest.NewRequest(http.MethodPost, "/v1/estimates", strings.NewReader(`{"a":1}`))
		r.Header.Set("Idempotency-Key", "shared-key")
		r.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	u1 := send(session.Principal{Kind: "session", UserID: "usr_1"})
	u2 := send(session.Principal{Kind: "session", UserID: "usr_2"})
	// TWO org keys, not one: with a single org-key principal any bucket
	// distinct from the users' would pass, so a collapsed or constant
	// `orgkey:` bucket would go unnoticed. Org keys are the machine callers —
	// the population most likely to retry — so a shared bucket is a
	// cross-tenant replay.
	k1 := send(session.Principal{Kind: "token", TokenID: "tok_1", OrgID: "org_1"})
	k2 := send(session.Principal{Kind: "token", TokenID: "tok_2", OrgID: "org_1"})
	if n != 4 {
		t.Fatalf("four DIFFERENT principals sharing a key must each execute; handler ran %d times", n)
	}
	for name, resp := range map[string]*httptest.ResponseRecorder{"user2": u2, "orgkey1": k1, "orgkey2": k2} {
		if resp.Header().Get("Idempotent-Replay") == "true" {
			t.Fatalf("%s received another principal's response as a replay — cross-caller leak", name)
		}
	}
	if u1.Body.String() == u2.Body.String() {
		t.Fatalf("usr_2 got usr_1's response: %s", u2.Body.String())
	}
	if k1.Body.String() == k2.Body.String() {
		t.Fatalf("tok_2 got tok_1's response: %s — two org keys share a dedupe bucket", k2.Body.String())
	}

	// Each principal repeating its own key must replay — including an org key,
	// which pins that `orgkey:` is stable and not merely distinct.
	for name, p := range map[string]session.Principal{
		"user":   {Kind: "session", UserID: "usr_1"},
		"orgkey": {Kind: "token", TokenID: "tok_1", OrgID: "org_1"},
	} {
		again := send(p)
		if again.Header().Get("Idempotent-Replay") != "true" {
			t.Fatalf("%s repeating its own key must replay, got %d %s", name, again.Code, again.Body.String())
		}
	}
	if n != 4 {
		t.Fatalf("a replay must not re-execute; handler ran %d times", n)
	}
}

// A panic must RELEASE the claim. problem.Recover sits above this middleware,
// so the panic unwinds straight past it — without a deferred completion the
// client is refused "still in progress" for the whole abandonment window on a
// request that definitively failed.
func TestAPanicReleasesTheClaimSoTheRetryCanRun(t *testing.T) {
	fs := &fakeStore{rows: map[string]*fakeRow{}}
	n := 0
	h := Middleware(newTestService(fs), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			panic("boom")
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"est_1"}`))
	}))

	func() {
		defer func() { _ = recover() }() // stand in for problem.Recover above us
		post(t, h, "/v1/estimates", "k1", `{"a":1}`)
	}()

	retry := post(t, h, "/v1/estimates", "k1", `{"a":1}`)
	if retry.Code != 201 {
		t.Fatalf("a retry after a panicking handler must execute, got %d — the panic stranded the claim", retry.Code)
	}
	if n != 2 {
		t.Fatalf("handler ran %d times, want 2", n)
	}
}

// r.Context() is cancelled the instant the client disconnects — which is the
// exact scenario S7 exists for. Completing on it would abort the write, leave
// the claim unrecorded, and let the retry re-execute.
func TestCompleteSurvivesAClientDisconnect(t *testing.T) {
	fs := &fakeStore{rows: map[string]*fakeRow{}}
	n := 0
	h := Middleware(newTestService(fs), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"est_1"}`))
	}))

	// A request whose context is already cancelled: the client hung up.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodPost, "/v1/estimates", strings.NewReader(`{"a":1}`)).WithContext(ctx)
	r.Header.Set("Idempotency-Key", "k1")
	r.RemoteAddr = "10.0.0.1:1234"
	h.ServeHTTP(httptest.NewRecorder(), r)

	retry := post(t, h, "/v1/estimates", "k1", `{"a":1}`)
	if retry.Header().Get("Idempotent-Replay") != "true" {
		t.Fatalf("the response was not recorded after the client disconnected (status %d) — the retry re-executes and double-provisions", retry.Code)
	}
	if n != 1 {
		t.Fatalf("handler ran %d times; a disconnect must not cost a second execution", n)
	}
}

// Concurrency at the HTTP layer, not just the service layer.
func TestConcurrentHTTPDoubleSubmitExecutesOnce(t *testing.T) {
	fs := &fakeStore{rows: map[string]*fakeRow{}}
	var mu sync.Mutex
	n := 0
	h := Middleware(newTestService(fs), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"est_1"}`))
	}))

	var wg sync.WaitGroup
	codes := make([]int, 12)
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = post(t, h, "/v1/estimates", "krace", `{"a":1}`).Code
		}()
	}
	wg.Wait()
	if n != 1 {
		t.Fatalf("12 concurrent submits executed the handler %d times — each extra one is a double-provision", n)
	}
	for i, c := range codes {
		if c != 201 && c != http.StatusConflict {
			t.Fatalf("request %d got %d; every non-owner must be a replay or an in-flight 409, never a 500", i, c)
		}
	}
	// With a slow handler every loser lands in-flight, so the REPLAY path is
	// never exercised above. One more submit after the winner has completed
	// pins that a retry actually receives the original response.
	after := post(t, h, "/v1/estimates", "krace", `{"a":1}`)
	if after.Header().Get("Idempotent-Replay") != "true" || after.Body.String() != `{"id":"est_1"}` {
		t.Fatalf("a retry after the winner completed did not replay: %d %s", after.Code, after.Body.String())
	}
	if n != 1 {
		t.Fatalf("the replay re-executed the handler (%d runs)", n)
	}
}

// spyWriter records whether the wrapper actually reached through. Asserting
// that `w.(http.Flusher)` merely SUCCEEDS proves nothing — the wrapper declares
// the method, so the assertion is true by construction whether or not the call
// forwards.
type spyWriter struct {
	http.ResponseWriter
	flushes int
}

func (s *spyWriter) Flush() { s.flushes++ }

type plainWriter struct{ http.ResponseWriter } // deliberately NOT a Hijacker

func TestRecorderForwardsFlushToTheUnderlyingWriter(t *testing.T) {
	spy := &spyWriter{ResponseWriter: httptest.NewRecorder()}
	h := Middleware(newTestService(&fakeStore{rows: map[string]*fakeRow{}}), nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("the recorder stripped http.Flusher")
			}
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{}`))
			f.Flush()
			// ResponseController is the modern path and must work too.
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Fatalf("ResponseController.Flush did not reach the writer: %v", err)
			}
		}))
	r := httptest.NewRequest(http.MethodPost, "/v1/estimates", strings.NewReader(`{"a":1}`))
	r.Header.Set("Idempotency-Key", "k1")
	r.RemoteAddr = "10.0.0.1:1234"
	h.ServeHTTP(spy, r)

	if spy.flushes != 2 {
		t.Fatalf("the underlying writer saw %d flushes, want 2 — the wrapper declares Flush but does not forward it", spy.flushes)
	}
}

// A capability must be advertised only when it EXISTS. Declaring Hijack
// unconditionally makes `w.(http.Hijacker)` succeed over a writer that cannot
// hijack, so a handler takes the hijack branch instead of its fallback and
// fails at runtime — worse than stripping.
func TestRecorderDoesNotClaimHijackerItDoesNotHave(t *testing.T) {
	for _, tc := range []struct {
		name string
		w    http.ResponseWriter
		want bool
	}{
		{"underlying cannot hijack", &plainWriter{ResponseWriter: httptest.NewRecorder()}, false},
		{"underlying can hijack", &hijackableWriter{ResponseWriter: httptest.NewRecorder()}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got bool
			h := Middleware(newTestService(&fakeStore{rows: map[string]*fakeRow{}}), nil)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var hj http.Hijacker
					hj, got = w.(http.Hijacker)
					if got {
						_, _, _ = hj.Hijack()
					}
					w.WriteHeader(201)
					_, _ = w.Write([]byte(`{}`))
				}))
			r := httptest.NewRequest(http.MethodPost, "/v1/estimates", strings.NewReader(`{"a":1}`))
			r.Header.Set("Idempotency-Key", "k1")
			r.RemoteAddr = "10.0.0.1:1234"
			h.ServeHTTP(tc.w, r)
			if got != tc.want {
				t.Fatalf("handler saw Hijacker=%v, want %v", got, tc.want)
			}
			// Advertising the capability is not enough — the call must REACH
			// the underlying writer, exactly as the Flush test requires.
			if hw, ok := tc.w.(*hijackableWriter); ok && !hw.hijacked {
				t.Fatal("the handler hijacked but the call never reached the underlying writer")
			}
		})
	}
}

type hijackableWriter struct {
	http.ResponseWriter
	hijacked bool
}

func (h *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil
}

// BOTH directions of the endpoint key are pinned, because each half-fix
// reintroduces the other's bug and this line has already oscillated once.
//
// SPLITTING: two spellings the ROUTER treats as identical must share a bucket,
// or a retry that re-encodes an escape executes the same key twice.
func TestEndpointKeyDoesNotSplitEquivalentSpellings(t *testing.T) {
	for _, tc := range []struct{ name, first, second string }{
		{"underscore escaped", "/v1/envs/env_1/services", "/v1/envs/env%5F1/services"},
		{"literal escaped", "/v1/estimates", "/v1/%65stimates"},
		{"hex case differs", "/v1/envs/a%2Fb/services", "/v1/envs/a%2fb/services"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := 0
			h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n++
				w.WriteHeader(201)
				_, _ = w.Write([]byte(`{"id":"est_1"}`))
			}))
			post(t, h, tc.first, "k1", `{"a":1}`)
			second := post(t, h, tc.second, "k1", `{"a":1}`)
			if n != 1 {
				t.Fatalf("%s: an equivalent spelling escaped the dedupe bucket (handler ran %d times) — a retry that re-encodes double-provisions", tc.name, n)
			}
			if second.Header().Get("Idempotent-Replay") != "true" {
				t.Fatalf("%s: the equivalent spelling was not treated as the same endpoint: %d", tc.name, second.Code)
			}
		})
	}
}

// COLLAPSING: the endpoint key must alias NO MORE than the router does. Decoding the path
// first (path.Clean on r.URL.Path) collapses `%2F` and `%2E` spellings that
// address DIFFERENT resources into one bucket — which would replay one env's
// service to another. That is the cross-resource leak this scoping exists to
// prevent, so it is pinned in the collapsing direction.
func TestEndpointKeyDoesNotCollapseDistinctEncodedPaths(t *testing.T) {
	n := 0
	var seen []string
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		seen = append(seen, r.URL.EscapedPath())
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"n":"` + r.URL.EscapedPath() + `"}`))
	}))
	// Two spellings that DECODE to the same string but address different envs.
	a := post(t, h, "/v1/envs/a%2Fb/services", "same", `{"x":1}`)
	b := post(t, h, "/v1/envs/a%2F%2E%2Fb/services", "same", `{"x":1}`)
	if n != 2 {
		t.Fatalf("two distinct encoded paths collapsed into one dedupe bucket (handler ran %d times, saw %v) — one env would receive another's response", n, seen)
	}
	if b.Header().Get("Idempotent-Replay") == "true" || a.Body.String() == b.Body.String() {
		t.Fatalf("a different resource was answered as a replay: %s", b.Body.String())
	}
}

// The problem body must carry a NON-EMPTY remediation. Asserting on the string
// "remediation" only matches the JSON key, which problem.Write always emits.
func TestConflictCarriesNonEmptyRemediation(t *testing.T) {
	h := newHarness(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"est_1"}`))
	}))
	post(t, h, "/v1/estimates", "k1", `{"a":1}`)
	got := post(t, h, "/v1/estimates", "k1", `{"a":2}`)

	var p struct {
		Remediation string `json:"remediation"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &p); err != nil {
		t.Fatalf("the 409 is not problem+json: %s", got.Body.String())
	}
	if strings.TrimSpace(p.Remediation) == "" {
		t.Fatalf("remediation is present but EMPTY — the contract is that it tells the caller what to do: %s", got.Body.String())
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("content-type is %q, want application/problem+json", ct)
	}
}

// The body cap: a middleware that BUFFERS what it reads is a memory amplifier
// if the read is unbounded, and it sits outside every handler-level limit.
func TestOversizedBodyIsRefusedAndTheHandlerNeverRuns(t *testing.T) {
	ran := false
	h, fs := newHarnessWithStore(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ran = true
		w.WriteHeader(201)
	}))
	got := post(t, h, "/v1/estimates", "k1", strings.Repeat("x", 2<<20))
	if ran {
		t.Fatal("an oversized body reached the handler — the cap did not apply")
	}
	// 422 explicitly, not merely "not 201": when US-3.6a revisits the catalog
	// and a 413 becomes available, that change must be a conscious edit here
	// rather than a silent behaviour shift.
	if got.Code != 422 {
		t.Fatalf("an oversized body must be refused with 422 today, got %d", got.Code)
	}
	fs.mu.Lock()
	claimed := len(fs.rows)
	fs.mu.Unlock()
	if claimed != 0 {
		t.Fatalf("a refused request claimed %d key(s); nothing should be claimed before the body is accepted", claimed)
	}
}

// errStore drives Begin's races: Claim always loses, and Get answers from a
// scripted sequence.
type errStore struct {
	gets []error
	i    int
}

func (e *errStore) ClaimIdempotencyKey(context.Context, store.ClaimIdempotencyKeyParams) (store.IdempotencyKey, error) {
	return store.IdempotencyKey{}, pgx.ErrNoRows
}
func (e *errStore) GetIdempotencyKey(_ context.Context, a store.GetIdempotencyKeyParams) (store.IdempotencyKey, error) {
	err := pgx.ErrNoRows
	if e.i < len(e.gets) {
		err = e.gets[e.i]
	}
	e.i++
	if err != nil {
		return store.IdempotencyKey{}, err
	}
	return store.IdempotencyKey{Principal: a.Principal, Endpoint: a.Endpoint, Key: a.Key,
		BodySha256: fingerprintOf([]byte(`{"a":1}`))}, nil
}
func (e *errStore) SaveIdempotentResponse(context.Context, store.SaveIdempotentResponseParams) (int64, error) {
	return 1, nil
}
func (e *errStore) ReleaseIdempotencyKey(context.Context, store.ReleaseIdempotencyKeyParams) (int64, error) {
	return 1, nil
}
func (e *errStore) SweepExpiredIdempotencyKeys(context.Context) (int64, error) { return 0, nil }

// A claim can fail because someone holds the key, and that holder can RELEASE
// it (a failed request) before the follow-up read runs. That legal sequence
// must not surface to the client as a 500.
func TestReleaseRacingARetryDoesNotError(t *testing.T) {
	svc := New(&errStore{gets: []error{pgx.ErrNoRows, nil}})
	replay, claim, err := svc.Begin(context.Background(), "user:1", "POST /v1/estimates", "k", []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("a retry racing a release must not error: %v", err)
	}
	// The second attempt's Get finds a live row with a matching fingerprint, so
	// the caller must observe an IN-FLIGHT claim (StatusCode 0) and must not be
	// handed ownership — asserting merely "not both empty" is unfalsifiable,
	// since Begin has no error-free return where both are empty.
	if claim != "" {
		t.Fatalf("ownership was granted while another request holds the key (claim %q)", claim)
	}
	if replay == nil || replay.StatusCode != 0 {
		t.Fatalf("want an in-flight replay, got %+v", replay)
	}
}

// If the key is released under us TWICE, Begin must report in-flight rather
// than hand out ownership — a pathological loop must never double-provision.
func TestRepeatedReleaseUnderUsNeverGrantsOwnership(t *testing.T) {
	svc := New(&errStore{gets: []error{pgx.ErrNoRows, pgx.ErrNoRows}})
	replay, claim, err := svc.Begin(context.Background(), "user:1", "POST /v1/estimates", "k", []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if claim != "" {
		t.Fatalf("ownership was granted after repeated contention (claim %q) — that is a second execution", claim)
	}
	if replay == nil || replay.StatusCode != 0 {
		t.Fatalf("want an in-flight replay, got %+v", replay)
	}
}

// sweepStore counts sweeps and can fail the first one.
type sweepStore struct {
	mu     sync.Mutex
	n      int
	failed bool
}

func (s *sweepStore) ClaimIdempotencyKey(context.Context, store.ClaimIdempotencyKeyParams) (store.IdempotencyKey, error) {
	return store.IdempotencyKey{}, pgx.ErrNoRows
}
func (s *sweepStore) GetIdempotencyKey(context.Context, store.GetIdempotencyKeyParams) (store.IdempotencyKey, error) {
	return store.IdempotencyKey{}, pgx.ErrNoRows
}
func (s *sweepStore) SaveIdempotentResponse(context.Context, store.SaveIdempotentResponseParams) (int64, error) {
	return 1, nil
}
func (s *sweepStore) ReleaseIdempotencyKey(context.Context, store.ReleaseIdempotencyKeyParams) (int64, error) {
	return 1, nil
}
func (s *sweepStore) SweepExpiredIdempotencyKeys(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	if s.n == 1 {
		s.failed = true
		return 0, errors.New("transient")
	}
	return 1, nil
}
func (s *sweepStore) count() int { s.mu.Lock(); defer s.mu.Unlock(); return s.n }

// The sweeper must run once at STARTUP: a ticker does not fire immediately, so
// a service that redeploys more often than its interval would otherwise never
// sweep at all. The interval here is an HOUR precisely so a tick cannot explain
// the sweep — only the startup call can.
func TestSweeperSweepsAtStartup(t *testing.T) {
	st := &sweepStore{}
	svc := New(st)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunSweeper(ctx, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil))); close(done) }()

	deadline := time.After(2 * time.Second)
	for st.count() < 1 {
		select {
		case <-deadline:
			t.Fatal("no sweep ran within 2s on an hourly interval — the startup sweep is missing, so a service redeploying faster than its interval never sweeps")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunSweeper did not return when its context was cancelled")
	}
}

// A failed sweep must not end the loop, and cancellation must stop it.
func TestSweeperSurvivesAFailedSweepAndStops(t *testing.T) {
	st := &sweepStore{}
	svc := New(st)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunSweeper(ctx, time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for st.count() < 3 {
		select {
		case <-deadline:
			t.Fatalf("only %d sweeps ran; the loop did not continue after the first failed", st.count())
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if !st.failed {
		t.Fatal("the failing-sweep path never executed")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunSweeper did not return when its context was cancelled")
	}
}
