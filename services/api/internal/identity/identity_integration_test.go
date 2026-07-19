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

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/events"
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
	"github.com/steloit/cloud/services/api/internal/platform/problem"
	"github.com/steloit/cloud/services/api/internal/platform/ratelimit"
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
	srv := httptest.NewServer(problem.Recover(streamer.Intercept(mux)))
	t.Cleanup(srv.Close)
	return &world{subs: subs, srv: srv, pool: pool, svc: svc, authz: authz, hub: hub, rec: recorder, prov: prov, vault: vault, kek: kek}
}

// testAPI composes the module handler sets exactly like the composition root.
type testAPI struct {
	*identity.Handlers
	Handlers2 *provisioning.Handlers
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
