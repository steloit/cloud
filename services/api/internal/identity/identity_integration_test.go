package identity_test

// Full-stack integration: real Postgres (testcontainers), real migrations,
// the mounted strict server, HTTP + database-state assertions. Skips (with
// the reason) when no container runtime exists locally; CI always runs it.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/password"
	"github.com/steloit/cloud/services/api/internal/identity/policy"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/db"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
	"github.com/steloit/cloud/services/api/internal/platform/ratelimit"
)

type world struct {
	srv   *httptest.Server
	pool  *pgxpool.Pool
	svc   *identity.Service
	authz *identity.Authorizer
	hub   *events.Hub
	envs  *fakeEnvs
}

func newWorld(t *testing.T, ttl time.Duration) *world {
	t.Helper()
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("app"), tcpostgres.WithUsername("app"), tcpostgres.WithPassword("app"),
		tcpostgres.BasicWaitStrategies(),
		tcpostgres.WithSQLDriver("pgx"),
	)
	if err != nil {
		t.Skipf("container runtime unavailable (CI runs this): %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })
	_ = wait.ForLog // keep import if strategies change

	url, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(url); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	hasher := password.NewHasher(password.Params{MemoryKiB: 8192, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32})
	mgr := session.NewManager(ttl, false)
	q := store.New(pool)
	hub := events.NewHub()
	recorder := events.NewRecorder(q, hub)
	svc, err := identity.NewService(pool, hasher, mgr, ratelimit.New(100, time.Minute), recorder)
	if err != nil {
		t.Fatal(err)
	}
	matrix, err := rbac.Load()
	if err != nil {
		t.Fatal(err)
	}
	policies := policy.NewEngine(identity.NewPolicySource(q))
	authz := identity.NewAuthorizer(q, rbac.NewEvaluator(matrix, policies), recorder)
	envs := &fakeEnvs{orgs: map[string]string{}}
	mux := http.NewServeMux()
	identity.NewHandlers(svc, mgr, authz, events.NewReader(q), envs).Mount(mux)
	streamer := &events.Streamer{
		Q: q, Hub: hub, Envs: envs,
		Principal: svc.PrincipalFromRequest,
		Authorize: authz.Require,
	}
	srv := httptest.NewServer(problem.Recover(streamer.Intercept(mux)))
	t.Cleanup(srv.Close)
	return &world{srv: srv, pool: pool, svc: svc, authz: authz, hub: hub, envs: envs}
}

// fakeEnvs stands in for the T3.2 environments module: tests attach env ids
// to orgs directly.
type fakeEnvs struct{ orgs map[string]string }

func (f *fakeEnvs) OrgForEnv(_ context.Context, envID string) (string, error) {
	if org, ok := f.orgs[envID]; ok {
		return org, nil
	}
	return "", events.ErrEnvNotFound
}

func (w *world) post(t *testing.T, path, body, cookie string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, w.srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b)
}

func (w *world) get(t *testing.T, path, cookie string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, w.srv.URL+path, nil)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b)
}

func sessionCookie(resp *http.Response) string {
	for _, c := range resp.Cookies() {
		if c.Name == session.CookieName {
			return c.Name + "=" + c.Value
		}
	}
	return ""
}

const signupBody = `{"email":"asha@example.com","password":"orbit-magnet-11","name":"Asha"}`
const loginBody = `{"email":"asha@example.com","password":"orbit-magnet-11"}`

func TestAuthLifecycle(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	// --- signup: 201, cookie set, DB state correct -------------------------
	resp, body := w.post(t, "/v1/auth/signup", signupBody, "")
	if resp.StatusCode != 201 {
		t.Fatalf("signup: %d %s", resp.StatusCode, body)
	}
	ck := sessionCookie(resp)
	if ck == "" {
		t.Fatal("signup set no session cookie")
	}
	var phc, uid string
	if err := w.pool.QueryRow(ctx, "select id, password_hash from users where email='asha@example.com'").Scan(&uid, &phc); err != nil {
		t.Fatalf("user row missing: %v", err)
	}
	if !strings.HasPrefix(phc, "$argon2id$") {
		t.Fatalf("password not argon2id PHC: %s", phc[:16])
	}
	if !strings.HasPrefix(uid, "usr_") {
		t.Fatalf("user id not prefixed: %s", uid)
	}
	raw := strings.TrimPrefix(ck, session.CookieName+"=")
	var n int
	if err := w.pool.QueryRow(ctx, "select count(*) from sessions where token_hash=$1", session.HashToken(raw)).Scan(&n); err != nil || n != 1 {
		t.Fatalf("session row (hashed) missing: n=%d err=%v", n, err)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from sessions where token_hash=$1", []byte(raw)).Scan(&n); err != nil || n != 0 {
		t.Fatal("raw token stored — must be hash-only")
	}

	// --- duplicate email: 409 with reasons[] (finding: contract silent) ----
	resp, body = w.post(t, "/v1/auth/signup", signupBody, "")
	if resp.StatusCode != 409 || !strings.Contains(body, "email already registered") {
		t.Fatalf("dup signup: %d %s", resp.StatusCode, body)
	}

	// --- weak password: 422 with field error --------------------------------
	resp, body = w.post(t, "/v1/auth/signup", `{"email":"b@example.com","password":"short","name":"B"}`, "")
	if resp.StatusCode != 422 || !strings.Contains(body, "password") {
		t.Fatalf("weak password: %d %s", resp.StatusCode, body)
	}

	// --- login: 200, session ROTATED (new id), cookie differs ---------------
	resp, body = w.post(t, "/v1/auth/login", loginBody, "")
	if resp.StatusCode != 200 || !strings.Contains(body, `"status":"session"`) {
		t.Fatalf("login: %d %s", resp.StatusCode, body)
	}
	loginCk := sessionCookie(resp)
	if loginCk == "" || loginCk == ck {
		t.Fatal("login did not rotate the session cookie")
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from sessions where user_id=$1", uid).Scan(&n); err != nil || n != 2 {
		t.Fatalf("expected 2 session rows after rotation, got %d", n)
	}

	// --- failed login: 403 problem, no disclosure ---------------------------
	resp, body = w.post(t, "/v1/auth/login", `{"email":"asha@example.com","password":"wrong-password!"}`, "")
	if resp.StatusCode != 401 || !strings.Contains(body, "remediation") {
		t.Fatalf("bad password: %d %s", resp.StatusCode, body)
	}
	resp, body2 := w.post(t, "/v1/auth/login", `{"email":"nobody@example.com","password":"wrong-password!"}`, "")
	if resp.StatusCode != 401 {
		t.Fatalf("unknown email: %d", resp.StatusCode)
	}
	if body != body2 {
		t.Fatal("unknown-email and wrong-password responses differ — account disclosure")
	}

	// --- current session: 200 with current=true -----------------------------
	resp, body = w.get(t, "/v1/auth/session", loginCk)
	if resp.StatusCode != 200 || !strings.Contains(body, `"current":true`) {
		t.Fatalf("get session: %d %s", resp.StatusCode, body)
	}

	// --- unauthorized: no cookie → 403 --------------------------------------
	resp, _ = w.get(t, "/v1/auth/session", "")
	if resp.StatusCode != 401 {
		t.Fatalf("no-cookie session: %d", resp.StatusCode)
	}

	// --- logout: 204, cookie cleared, session revoked in DB -----------------
	resp, _ = w.post(t, "/v1/auth/logout", "", loginCk)
	if resp.StatusCode != 204 {
		t.Fatalf("logout: %d", resp.StatusCode)
	}
	cleared := false
	for _, c := range resp.Cookies() {
		if c.Name == session.CookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("logout did not clear the cookie")
	}
	rawLogin := strings.TrimPrefix(loginCk, session.CookieName+"=")
	var revoked bool
	if err := w.pool.QueryRow(ctx, "select revoked_at is not null from sessions where token_hash=$1", session.HashToken(rawLogin)).Scan(&revoked); err != nil || !revoked {
		t.Fatalf("session not revoked in DB: %v", err)
	}
	resp, _ = w.get(t, "/v1/auth/session", loginCk)
	if resp.StatusCode != 401 {
		t.Fatalf("revoked session still valid: %d", resp.StatusCode)
	}
}

func TestSessionExpiry(t *testing.T) {
	w := newWorld(t, time.Second)
	resp, _ := w.post(t, "/v1/auth/signup", `{"email":"exp@example.com","password":"orbit-magnet-11","name":"E"}`, "")
	if resp.StatusCode != 201 {
		t.Fatalf("signup: %d", resp.StatusCode)
	}
	ck := sessionCookie(resp)
	time.Sleep(1500 * time.Millisecond)
	resp, _ = w.get(t, "/v1/auth/session", ck)
	if resp.StatusCode != 401 {
		t.Fatalf("expired session accepted: %d", resp.StatusCode)
	}
}

func TestMalformedBody(t *testing.T) {
	w := newWorld(t, time.Hour)
	resp, body := w.post(t, "/v1/auth/signup", `{not json`, "")
	if resp.StatusCode != 422 || !strings.Contains(body, "remediation") {
		t.Fatalf("malformed body: %d %s", resp.StatusCode, body)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(body), &m)
	if m["errors"] == nil {
		t.Fatal("422 without errors[]")
	}
}
