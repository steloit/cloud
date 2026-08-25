package identity_test

// Full-stack integration: real Postgres (testcontainers), real migrations,
// the mounted strict server, HTTP + database-state assertions. Skips (with
// the reason) when no container runtime exists locally; CI always runs it.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/httpapi"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/password"
	"github.com/steloit/cloud/services/api/internal/identity/policy"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/metering"
	"github.com/steloit/cloud/services/api/internal/notify"
	"github.com/steloit/cloud/services/api/internal/platform/db"
	"github.com/steloit/cloud/services/api/internal/platform/idempotency"
	"github.com/steloit/cloud/services/api/internal/platform/ratelimit"
	"github.com/steloit/cloud/services/api/internal/platform/testenv"
	"github.com/steloit/cloud/services/api/internal/provisioning"
	"github.com/steloit/cloud/services/api/internal/secrets"
	"github.com/steloit/cloud/services/api/internal/subscription"
)

// testKEK is a fixed 32-byte key (base64) — test worlds only.
const testKEK = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

type world struct {
	subs  *subscription.Service
	srv   *httptest.Server
	pool  *pgxpool.Pool
	svc   *identity.Service
	authz *identity.Authorizer
	hub   *events.Hub
	rec   *events.Recorder
	prov  *provisioning.Service
	vault *secrets.Vault
	kek   secrets.KEK
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
		testenv.SkipOrFail(t, err) // skip locally, FAIL in CI — see the package doc
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })
	_ = wait.ForLog // keep import if strategies change

	url, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	// O34: prove this port is served by THIS container's Postgres before anything
	// uses it. Under colima the VM picks the host port and nothing reserves it, so
	// a long-lived macOS listener can already own it — measured: rapportd holding
	// *:54167 since Aug 17 answered one of our containers with `01 00 00 00`.
	testenv.RequirePostgresPeer(t, url)
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
	kek, err := secrets.NewEnvKEK("test-v1", testKEK)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := billing.Load()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := identity.NewService(pool, hasher, mgr, ratelimit.New(100, time.Minute), recorder, kek, plans)
	if err != nil {
		t.Fatal(err)
	}
	matrix, err := rbac.Load()
	if err != nil {
		t.Fatal(err)
	}
	policies := policy.NewEngine(identity.NewPolicySource(q))
	authz := identity.NewAuthorizer(q, rbac.NewEvaluator(matrix, policies), recorder)
	vault := secrets.NewVault(q, kek)
	prov := provisioning.NewService(pool, recorder, vault, metering.NewEmitter(q), plans)
	mux := http.NewServeMux()
	subs := subscription.NewService(q, recorder)
	idHandlers := identity.NewHandlers(svc, mgr, authz, events.NewReader(q), prov, metering.NewEmitter(q), notify.NewRouter(q, kek), subs)
	idHandlers.Mount(mux, &testAPI{
		Handlers:  idHandlers,
		Handlers2: provisioning.NewHandlers(prov, authz, q, svc, estimates.NewService(q)),
	})
	streamer := &events.Streamer{
		Q: q, Hub: hub, Envs: prov,
		Principal: svc.PrincipalFromRequest,
		Authorize: authz.Require,
	}
	// The SAME chain main.go builds — including the S7 idempotency layer — so a
	// middleware that is mounted in production is exercised here, and one that
	// is dropped from production fails here.
	srv := httptest.NewServer(httpapi.Chain(mux,
		idempotency.Middleware(idempotency.New(q, kek), svc),
		streamer.Intercept))
	t.Cleanup(srv.Close)
	return &world{subs: subs, srv: srv, pool: pool, svc: svc, authz: authz, hub: hub, rec: recorder, prov: prov, vault: vault, kek: kek}
}

// testAPI composes the module handler sets exactly like the composition root.
type testAPI struct {
	*identity.Handlers
	Handlers2 *provisioning.Handlers
}

func (s *testAPI) CreateDashboard(ctx context.Context, r gen.CreateDashboardRequestObject) (gen.CreateDashboardResponseObject, error) {
	return s.Handlers2.CreateDashboard(ctx, r)
}
func (s *testAPI) ListDashboards(ctx context.Context, r gen.ListDashboardsRequestObject) (gen.ListDashboardsResponseObject, error) {
	return s.Handlers2.ListDashboards(ctx, r)
}
func (s *testAPI) GetDashboard(ctx context.Context, r gen.GetDashboardRequestObject) (gen.GetDashboardResponseObject, error) {
	return s.Handlers2.GetDashboard(ctx, r)
}
func (s *testAPI) UpdateDashboard(ctx context.Context, r gen.UpdateDashboardRequestObject) (gen.UpdateDashboardResponseObject, error) {
	return s.Handlers2.UpdateDashboard(ctx, r)
}
func (s *testAPI) DeleteDashboard(ctx context.Context, r gen.DeleteDashboardRequestObject) (gen.DeleteDashboardResponseObject, error) {
	return s.Handlers2.DeleteDashboard(ctx, r)
}
func (s *testAPI) AddWidget(ctx context.Context, r gen.AddWidgetRequestObject) (gen.AddWidgetResponseObject, error) {
	return s.Handlers2.AddWidget(ctx, r)
}
func (s *testAPI) ForkDashboard(ctx context.Context, r gen.ForkDashboardRequestObject) (gen.ForkDashboardResponseObject, error) {
	return s.Handlers2.ForkDashboard(ctx, r)
}
func (s *testAPI) DuplicateDashboard(ctx context.Context, r gen.DuplicateDashboardRequestObject) (gen.DuplicateDashboardResponseObject, error) {
	return s.Handlers2.DuplicateDashboard(ctx, r)
}
func (s *testAPI) DeleteWidget(ctx context.Context, r gen.DeleteWidgetRequestObject) (gen.DeleteWidgetResponseObject, error) {
	return s.Handlers2.DeleteWidget(ctx, r)
}
func (s *testAPI) CaptureTemplate(ctx context.Context, r gen.CaptureTemplateRequestObject) (gen.CaptureTemplateResponseObject, error) {
	return s.Handlers2.CaptureTemplate(ctx, r)
}
func (s *testAPI) ListTemplates(ctx context.Context, r gen.ListTemplatesRequestObject) (gen.ListTemplatesResponseObject, error) {
	return s.Handlers2.ListTemplates(ctx, r)
}
func (s *testAPI) GetTemplate(ctx context.Context, r gen.GetTemplateRequestObject) (gen.GetTemplateResponseObject, error) {
	return s.Handlers2.GetTemplate(ctx, r)
}
func (s *testAPI) UpdateTemplate(ctx context.Context, r gen.UpdateTemplateRequestObject) (gen.UpdateTemplateResponseObject, error) {
	return s.Handlers2.UpdateTemplate(ctx, r)
}
func (s *testAPI) DeleteTemplate(ctx context.Context, r gen.DeleteTemplateRequestObject) (gen.DeleteTemplateResponseObject, error) {
	return s.Handlers2.DeleteTemplate(ctx, r)
}
func (s *testAPI) RefreshTemplate(ctx context.Context, r gen.RefreshTemplateRequestObject) (gen.RefreshTemplateResponseObject, error) {
	return s.Handlers2.RefreshTemplate(ctx, r)
}
func (s *testAPI) CreateProject(ctx context.Context, r gen.CreateProjectRequestObject) (gen.CreateProjectResponseObject, error) {
	return s.Handlers2.CreateProject(ctx, r)
}
func (s *testAPI) ListProjects(ctx context.Context, r gen.ListProjectsRequestObject) (gen.ListProjectsResponseObject, error) {
	return s.Handlers2.ListProjects(ctx, r)
}
func (s *testAPI) GetProject(ctx context.Context, r gen.GetProjectRequestObject) (gen.GetProjectResponseObject, error) {
	return s.Handlers2.GetProject(ctx, r)
}
func (s *testAPI) UpdateProject(ctx context.Context, r gen.UpdateProjectRequestObject) (gen.UpdateProjectResponseObject, error) {
	return s.Handlers2.UpdateProject(ctx, r)
}
func (s *testAPI) DeleteProject(ctx context.Context, r gen.DeleteProjectRequestObject) (gen.DeleteProjectResponseObject, error) {
	return s.Handlers2.DeleteProject(ctx, r)
}
func (s *testAPI) CreateEnvironment(ctx context.Context, r gen.CreateEnvironmentRequestObject) (gen.CreateEnvironmentResponseObject, error) {
	return s.Handlers2.CreateEnvironment(ctx, r)
}
func (s *testAPI) ListEnvironments(ctx context.Context, r gen.ListEnvironmentsRequestObject) (gen.ListEnvironmentsResponseObject, error) {
	return s.Handlers2.ListEnvironments(ctx, r)
}
func (s *testAPI) CreateEstimate(ctx context.Context, r gen.CreateEstimateRequestObject) (gen.CreateEstimateResponseObject, error) {
	return s.Handlers2.CreateEstimate(ctx, r)
}
func (s *testAPI) CreateService(ctx context.Context, r gen.CreateServiceRequestObject) (gen.CreateServiceResponseObject, error) {
	return s.Handlers2.CreateService(ctx, r)
}
func (s *testAPI) ListServices(ctx context.Context, r gen.ListServicesRequestObject) (gen.ListServicesResponseObject, error) {
	return s.Handlers2.ListServices(ctx, r)
}
func (s *testAPI) GetService(ctx context.Context, r gen.GetServiceRequestObject) (gen.GetServiceResponseObject, error) {
	return s.Handlers2.GetService(ctx, r)
}
func (s *testAPI) UpdateService(ctx context.Context, r gen.UpdateServiceRequestObject) (gen.UpdateServiceResponseObject, error) {
	return s.Handlers2.UpdateService(ctx, r)
}
func (s *testAPI) DeleteService(ctx context.Context, r gen.DeleteServiceRequestObject) (gen.DeleteServiceResponseObject, error) {
	return s.Handlers2.DeleteService(ctx, r)
}
func (s *testAPI) CreateBinding(ctx context.Context, r gen.CreateBindingRequestObject) (gen.CreateBindingResponseObject, error) {
	return s.Handlers2.CreateBinding(ctx, r)
}
func (s *testAPI) ListBindings(ctx context.Context, r gen.ListBindingsRequestObject) (gen.ListBindingsResponseObject, error) {
	return s.Handlers2.ListBindings(ctx, r)
}
func (s *testAPI) DeleteBinding(ctx context.Context, r gen.DeleteBindingRequestObject) (gen.DeleteBindingResponseObject, error) {
	return s.Handlers2.DeleteBinding(ctx, r)
}
func (s *testAPI) CreateDeployment(ctx context.Context, r gen.CreateDeploymentRequestObject) (gen.CreateDeploymentResponseObject, error) {
	return s.Handlers2.CreateDeployment(ctx, r)
}
func (s *testAPI) ListDeployments(ctx context.Context, r gen.ListDeploymentsRequestObject) (gen.ListDeploymentsResponseObject, error) {
	return s.Handlers2.ListDeployments(ctx, r)
}
func (s *testAPI) RollbackDeployment(ctx context.Context, r gen.RollbackDeploymentRequestObject) (gen.RollbackDeploymentResponseObject, error) {
	return s.Handlers2.RollbackDeployment(ctx, r)
}
func (s *testAPI) RenameEnvironment(ctx context.Context, r gen.RenameEnvironmentRequestObject) (gen.RenameEnvironmentResponseObject, error) {
	return s.Handlers2.RenameEnvironment(ctx, r)
}
func (s *testAPI) DeleteEnvironment(ctx context.Context, r gen.DeleteEnvironmentRequestObject) (gen.DeleteEnvironmentResponseObject, error) {
	return s.Handlers2.DeleteEnvironment(ctx, r)
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

// US-3.6 / S7, end-to-end through the REAL composed server (httpapi.Chain, real
// Postgres, real handlers). The unit tests prove the middleware; this proves it
// is MOUNTED — a chain that drops the layer fails here, and only here.
//
// createEstimate is the route the task exists to protect: estimates are
// one-shot (F2), so a client that times out and retries must replay rather than
// burn a second one.
func TestIdempotentEstimateReplaysInsteadOfBurningASecond(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	resp, _ := w.post(t, "/v1/auth/signup", signupBody, "")
	ck := sessionCookie(resp)
	if ck == "" {
		t.Fatal("signup did not set a session cookie")
	}

	estimate := func(key, body string) (*http.Response, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, w.srv.URL+"/v1/estimates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", ck)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		return r, string(b)
	}

	const reqBody = `{"services":[{"product":"postgres","intent":"database"}]}`
	first, body1 := estimate("est-key-1", reqBody)
	if first.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", first.StatusCode, body1)
	}
	if first.Header.Get("Idempotent-Replay") != "" {
		t.Fatal("the first request must not be marked a replay")
	}

	// The client's connection dropped and it retried with the same key.
	second, body2 := estimate("est-key-1", reqBody)
	if second.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("the retry was NOT deduped — the idempotency layer is not mounted in the served chain (status %d, body %s)",
			second.StatusCode, body2)
	}
	if second.StatusCode != first.StatusCode || body2 != body1 {
		t.Fatalf("replay is not the original response VERBATIM:\n first: %d %s\nsecond: %d %s",
			first.StatusCode, body1, second.StatusCode, body2)
	}

	// The decisive check: exactly ONE estimate was created, not two.
	var n int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM estimates`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("a double-submitted estimate created %d estimates, want exactly 1 (F2: an estimate is one-shot)", n)
	}
}

// The e2e twin of the principal-scoping unit test: two REAL users behind the
// same IP sharing a key must not share a result. Every other e2e uses one user,
// so a single bucket would satisfy them all and a regression in pre-strict
// principal resolution would go unnoticed.
func TestTwoUsersSharingAKeyFromOneIPDoNotShareAnEstimate(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ckA, _ := w.signupUser(t, "share-a@example.com")
	ckB, _ := w.signupUser(t, "share-b@example.com")

	estimate := func(ck string) (*http.Response, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, w.srv.URL+"/v1/estimates",
			strings.NewReader(`{"services":[{"product":"postgres","intent":"database"}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", ck)
		req.Header.Set("Idempotency-Key", "shared-across-users")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		return r, string(b)
	}

	a, bodyA := estimate(ckA)
	b, bodyB := estimate(ckB)
	if a.StatusCode != 200 || b.StatusCode != 200 {
		t.Fatalf("both users must be served: %d / %d", a.StatusCode, b.StatusCode)
	}
	if b.Header.Get("Idempotent-Replay") == "true" {
		t.Fatal("user B received user A's response as a replay — cross-user leak")
	}
	if bodyA == bodyB {
		t.Fatalf("both users got the same estimate: %s", bodyB)
	}
	var n int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM estimates`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("two users sharing a key produced %d estimates, want 2", n)
	}
}

// The same key with a DIFFERENT body must be refused end-to-end, with a
// problem+json carrying remediation (the error contract, not just a status).
func TestIdempotentReuseWithDifferentBodyIsRefusedEndToEnd(t *testing.T) {
	w := newWorld(t, time.Hour)
	resp, _ := w.post(t, "/v1/auth/signup", signupBody, "")
	ck := sessionCookie(resp)

	estimate := func(key, body string) (*http.Response, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, w.srv.URL+"/v1/estimates", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", ck)
		req.Header.Set("Idempotency-Key", key)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		return r, string(b)
	}
	estimate("k-mismatch", `{"services":[{"product":"postgres","intent":"database"}]}`)
	got, body := estimate("k-mismatch", `{"services":[{"product":"valkey","intent":"cache"}]}`)
	if got.StatusCode != http.StatusConflict {
		t.Fatalf("same key + different body must be 409, got %d %s", got.StatusCode, body)
	}
	if !strings.Contains(body, "remediation") {
		t.Fatalf("problem+json must carry remediation: %s", body)
	}
}

// US-3.6a: signup is now ENFORCED, and the replay must carry the SAME session
// cookie the original issued — without it the client holds no session while
// believing it does. End-to-end through the real chain and real Postgres.
func TestIdempotentSignupReplaysTheSameSessionCookie(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	send := func() (*http.Response, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, w.srv.URL+"/v1/auth/signup", strings.NewReader(signupBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "signup-k1")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		return r, string(b)
	}

	first, body1 := send()
	if first.StatusCode != 201 {
		t.Fatalf("signup: %d %s", first.StatusCode, body1)
	}
	ck1 := sessionCookie(first)
	if ck1 == "" {
		t.Fatal("signup did not set a session cookie")
	}

	// Scan BEFORE the replay assertions: the record already exists, and a
	// bypassed seal must be caught here rather than masked by a later
	// replay failure. ADR-0013's widening, against the ACTUAL minted token.
	assertCredentialNotRecoverable(t, w.pool, strings.TrimPrefix(ck1, session.CookieName+"="))

	second, body2 := send()
	if second.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("signup was not deduped: %d %s", second.StatusCode, body2)
	}
	if body2 != body1 {
		t.Fatalf("replay body differs:\n%s\n%s", body1, body2)
	}
	ck2 := sessionCookie(second)
	if ck2 != ck1 {
		t.Fatalf("the replay did not carry the original session cookie: %q vs %q — the client would believe it is signed in while holding nothing", ck2, ck1)
	}
	// Compare the RAW header, not just name=value: sessionCookie() drops
	// HttpOnly/Secure/SameSite, so a replay that lost those attributes would
	// pass a name=value check while handing the client a weaker cookie.
	if !slices.Equal(second.Header.Values("Set-Cookie"), first.Header.Values("Set-Cookie")) {
		t.Fatalf("Set-Cookie differs in attributes or multiplicity:\n first: %v\nsecond: %v",
			first.Header.Values("Set-Cookie"), second.Header.Values("Set-Cookie"))
	}
	// Content-Type must survive too. The replay path no longer hardcodes it, so
	// it depends on the recorded headers; the second assertion stops this going
	// vacuous if the fixture ever stops setting one.
	if !strings.Contains(first.Header.Get("Content-Type"), "json") {
		t.Fatalf("the fixture proves nothing: original Content-Type is %q", first.Header.Get("Content-Type"))
	}
	if second.Header.Get("Content-Type") != first.Header.Get("Content-Type") {
		t.Fatalf("replay Content-Type %q != original %q", second.Header.Get("Content-Type"), first.Header.Get("Content-Type"))
	}

	// Exactly one account, and the replayed cookie is a WORKING session.
	var n int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE email = 'asha@example.com'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("a double-submitted signup produced %d accounts, want 1", n)
	}
	resp, body := w.get(t, "/v1/auth/session", ck2)
	if resp.StatusCode != 200 {
		t.Fatalf("the replayed session is not usable: %d %s", resp.StatusCode, body)
	}
}

// The route this task exists to protect, end-to-end. F2 makes an estimate
// ONE-SHOT: a client that times out mid-createService and retries must replay,
// not burn the estimate a second time — which would fail the retry outright and
// leave the caller unable to provision what they already paid to price.
func TestReplayedCreateServiceDoesNotBurnASecondEstimate(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "svc-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"svcco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	orgRow, err := w.svc.GetOrg(ctx, org.Id)
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}

	const shape = `{"size":"dev","storage_gb":10}`
	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":`+shape+`}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", resp.StatusCode, body)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)

	createBody := `{"name":"db","product":"postgres","estimate_id":"` + est.Id + `","shape":` + shape + `}`
	send := func(key string) (*http.Response, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, w.srv.URL+"/v1/envs/"+env.ID+"/services",
			strings.NewReader(createBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", ownerCk)
		req.Header.Set("Idempotency-Key", key)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		return r, string(b)
	}

	first, body1 := send("svc-key-1")
	if first.StatusCode != 201 {
		t.Fatalf("createService: %d %s", first.StatusCode, body1)
	}

	// The client's connection dropped; it retries with the same key. WITHOUT
	// dedupe this second call re-consumes the one-shot estimate and fails.
	second, body2 := send("svc-key-1")
	if second.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("the retry was not deduped: %d %s", second.StatusCode, body2)
	}
	if second.StatusCode != 201 || body2 != body1 {
		t.Fatalf("replay is not the original response:\n first: %d %s\nsecond: %d %s",
			first.StatusCode, body1, second.StatusCode, body2)
	}

	// Exactly ONE service, and nothing billed — provisioning has not reached
	// ready, and D10 opens a span only at the ready edge.
	var services int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM services WHERE env_id=$1`, env.ID).Scan(&services); err != nil {
		t.Fatal(err)
	}
	if services != 1 {
		t.Fatalf("a double-submitted createService produced %d services, want exactly 1", services)
	}
}

// The reveal-once route this whole ADR exists for, end-to-end. A replay must
// return the SAME secret (it is the same response to the same request), while
// reveal-once still holds outside the replay window: fetching the webhook later
// must not expose it.
func TestIdempotentCreateWebhookReplaysTheRevealOnceSecret(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, _ := w.signupUser(t, "wh-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"whco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	send := func() (*http.Response, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, w.srv.URL+"/v1/orgs/"+org.Id+"/webhooks",
			strings.NewReader(`{"url":"https://example.com/hook","events":["service.ready"]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", ownerCk)
		req.Header.Set("Idempotency-Key", "wh-k1")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		return r, string(b)
	}

	first, body1 := send()
	if first.StatusCode != 201 {
		t.Fatalf("createWebhook: %d %s", first.StatusCode, body1)
	}
	var created struct{ Secret, Id string }
	if err := json.Unmarshal([]byte(body1), &created); err != nil || created.Secret == "" {
		t.Fatalf("no secret in the 201 — this test would prove nothing: %s", body1)
	}

	// Scan BEFORE the replay assertions, for the same reason as signup.
	assertCredentialNotRecoverable(t, w.pool, created.Secret)

	second, body2 := send()
	if second.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("createWebhook was not deduped: %d %s", second.StatusCode, body2)
	}
	if second.StatusCode != 201 || body2 != body1 {
		t.Fatalf("replay is not the original response:\n%s\n%s", body1, body2)
	}

	var n int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM webhooks WHERE org_id=$1`, org.Id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("a double-submitted createWebhook produced %d webhooks, want 1", n)
	}

	// Reveal-once still holds: listing must not expose the secret.
	resp, listBody := w.get(t, "/v1/orgs/"+org.Id+"/webhooks", ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("listWebhooks: %d %s", resp.StatusCode, listBody)
	}
	if strings.Contains(listBody, created.Secret) {
		t.Fatal("the signing secret is exposed outside its reveal-once response — replay must not weaken reveal-once")
	}
}

// assertCredentialNotRecoverable scans EVERY idempotency row for a credential,
// literally and base64-encoded. Base64 matters because the sealed payload is
// JSON with a []byte body: a bypassed seal stores the body base64, which a
// literal scan cannot see.
//
// This is the assertion that ties ADR-0013's "the raw session token lives
// envelope-encrypted in idempotency_keys" to a REAL minted credential. Every
// other test on this path uses a fabricated stand-in.
func assertCredentialNotRecoverable(t *testing.T, pool *pgxpool.Pool, raw string) {
	t.Helper()
	if len(raw) < 16 {
		t.Fatalf("fixture credential is only %d chars — too short to test meaningfully", len(raw))
	}
	var blob []byte
	if err := pool.QueryRow(context.Background(), `
		SELECT coalesce(string_agg(
			coalesce(response_ciphertext, ''::bytea)
			 || coalesce(response_nonce, ''::bytea)
			 || coalesce(response_wrapped_dek, ''::bytea)
			 || coalesce(response_dek_nonce, ''::bytea), ''::bytea), ''::bytea)
		FROM idempotency_keys`).Scan(&blob); err != nil {
		t.Fatal(err)
	}
	if len(blob) <= 16 {
		t.Fatalf("no sealed payload stored (%d bytes) — this scan proves nothing", len(blob))
	}
	if strings.Contains(string(blob), raw) {
		t.Fatal("the REAL credential is recoverable from idempotency_keys in PLAINTEXT")
	}
	const minInterior = 8
	for off := range 3 {
		enc := base64.StdEncoding.EncodeToString(append(bytes.Repeat([]byte{'x'}, off), raw...))
		if len(enc) < 8+minInterior {
			t.Fatalf("credential too short for the base64 check (%d chars)", len(enc))
		}
		if strings.Contains(string(blob), enc[4:len(enc)-4]) {
			t.Fatalf("the REAL credential is recoverable from idempotency_keys as BASE64 (alignment %d)", off)
		}
	}
}
