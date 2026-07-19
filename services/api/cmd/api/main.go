// The §15 composition root: config → pool → stores → services → handlers,
// explicitly, in order. Modules mount themselves; the mux stays dumb.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/github"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/password"
	"github.com/steloit/cloud/services/api/internal/identity/policy"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/mailer"
	"github.com/steloit/cloud/services/api/internal/metering"
	"github.com/steloit/cloud/services/api/internal/platform/config"
	"github.com/steloit/cloud/services/api/internal/platform/db"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
	"github.com/steloit/cloud/services/api/internal/platform/ratelimit"
	"github.com/steloit/cloud/services/api/internal/provisioning"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

var _ = gen.SessionInfo{} // anchor: the contract types this binary serves

// apiServer composes each module's strict-handler set into the one
// gen.StrictServerInterface (method sets merge by embedding).
type apiServer struct {
	*identity.Handlers
	Handlers2 *provisioning.Handlers
}

func (s *apiServer) CreateProject(ctx context.Context, r gen.CreateProjectRequestObject) (gen.CreateProjectResponseObject, error) {
	return s.Handlers2.CreateProject(ctx, r)
}
func (s *apiServer) ListProjects(ctx context.Context, r gen.ListProjectsRequestObject) (gen.ListProjectsResponseObject, error) {
	return s.Handlers2.ListProjects(ctx, r)
}
func (s *apiServer) GetProject(ctx context.Context, r gen.GetProjectRequestObject) (gen.GetProjectResponseObject, error) {
	return s.Handlers2.GetProject(ctx, r)
}
func (s *apiServer) UpdateProject(ctx context.Context, r gen.UpdateProjectRequestObject) (gen.UpdateProjectResponseObject, error) {
	return s.Handlers2.UpdateProject(ctx, r)
}
func (s *apiServer) DeleteProject(ctx context.Context, r gen.DeleteProjectRequestObject) (gen.DeleteProjectResponseObject, error) {
	return s.Handlers2.DeleteProject(ctx, r)
}
func (s *apiServer) CreateEnvironment(ctx context.Context, r gen.CreateEnvironmentRequestObject) (gen.CreateEnvironmentResponseObject, error) {
	return s.Handlers2.CreateEnvironment(ctx, r)
}
func (s *apiServer) ListEnvironments(ctx context.Context, r gen.ListEnvironmentsRequestObject) (gen.ListEnvironmentsResponseObject, error) {
	return s.Handlers2.ListEnvironments(ctx, r)
}
func (s *apiServer) CreateEstimate(ctx context.Context, r gen.CreateEstimateRequestObject) (gen.CreateEstimateResponseObject, error) {
	return s.Handlers2.CreateEstimate(ctx, r)
}
func (s *apiServer) CreateService(ctx context.Context, r gen.CreateServiceRequestObject) (gen.CreateServiceResponseObject, error) {
	return s.Handlers2.CreateService(ctx, r)
}
func (s *apiServer) ListServices(ctx context.Context, r gen.ListServicesRequestObject) (gen.ListServicesResponseObject, error) {
	return s.Handlers2.ListServices(ctx, r)
}
func (s *apiServer) GetService(ctx context.Context, r gen.GetServiceRequestObject) (gen.GetServiceResponseObject, error) {
	return s.Handlers2.GetService(ctx, r)
}
func (s *apiServer) UpdateService(ctx context.Context, r gen.UpdateServiceRequestObject) (gen.UpdateServiceResponseObject, error) {
	return s.Handlers2.UpdateService(ctx, r)
}
func (s *apiServer) DeleteService(ctx context.Context, r gen.DeleteServiceRequestObject) (gen.DeleteServiceResponseObject, error) {
	return s.Handlers2.DeleteService(ctx, r)
}
func (s *apiServer) CreateBinding(ctx context.Context, r gen.CreateBindingRequestObject) (gen.CreateBindingResponseObject, error) {
	return s.Handlers2.CreateBinding(ctx, r)
}
func (s *apiServer) ListBindings(ctx context.Context, r gen.ListBindingsRequestObject) (gen.ListBindingsResponseObject, error) {
	return s.Handlers2.ListBindings(ctx, r)
}
func (s *apiServer) DeleteBinding(ctx context.Context, r gen.DeleteBindingRequestObject) (gen.DeleteBindingResponseObject, error) {
	return s.Handlers2.DeleteBinding(ctx, r)
}
func (s *apiServer) CreateDeployment(ctx context.Context, r gen.CreateDeploymentRequestObject) (gen.CreateDeploymentResponseObject, error) {
	return s.Handlers2.CreateDeployment(ctx, r)
}
func (s *apiServer) ListDeployments(ctx context.Context, r gen.ListDeploymentsRequestObject) (gen.ListDeploymentsResponseObject, error) {
	return s.Handlers2.ListDeployments(ctx, r)
}
func (s *apiServer) RollbackDeployment(ctx context.Context, r gen.RollbackDeploymentRequestObject) (gen.RollbackDeploymentResponseObject, error) {
	return s.Handlers2.RollbackDeployment(ctx, r)
}
func (s *apiServer) RenameEnvironment(ctx context.Context, r gen.RenameEnvironmentRequestObject) (gen.RenameEnvironmentResponseObject, error) {
	return s.Handlers2.RenameEnvironment(ctx, r)
}
func (s *apiServer) DeleteEnvironment(ctx context.Context, r gen.DeleteEnvironmentRequestObject) (gen.DeleteEnvironmentResponseObject, error) {
	return s.Handlers2.DeleteEnvironment(ctx, r)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("boot failed", "err", err)
		os.Exit(1)
	}
	cfg.LogEffective(logger)

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		logger.Error("boot failed", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("boot failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := store.New(pool)
	hasher := password.NewHasher(password.Params{
		MemoryKiB: cfg.ArgonMemoryKiB, Time: cfg.ArgonTime, Threads: cfg.ArgonThreads,
		SaltLen: 16, KeyLen: 32,
	})
	sessions := session.NewManager(cfg.SessionTTL, cfg.CookieSecure)
	loginLimiter := ratelimit.New(10, time.Minute)

	hub := events.NewHub()
	recorder := events.NewRecorder(queries, hub)
	kek, err := secrets.NewEnvKEK(cfg.SecretsKEKID, cfg.SecretsKEK)
	if err != nil {
		logger.Error("boot failed", "err", err)
		os.Exit(1)
	}
	svc, err := identity.NewService(pool, hasher, sessions, loginLimiter, recorder, kek)
	if err != nil {
		logger.Error("boot failed", "err", err)
		os.Exit(1)
	}
	matrix, err := rbac.Load()
	if err != nil {
		logger.Error("boot failed", "err", err)
		os.Exit(1)
	}
	policies := policy.NewEngine(identity.NewPolicySource(queries))
	svc.SetPolicyKinds(policies.Knows) // T12.1: authoring refuses enforce on an unimplemented kind
	authz := identity.NewAuthorizer(queries, rbac.NewEvaluator(matrix, policies), recorder)
	vault := secrets.NewVault(queries, kek)
	prov := provisioning.NewService(pool, recorder, vault, metering.NewEmitter(queries))
	envs := prov // T3.2 closed the env→org seam: environments are real rows

	// T10.4: email is Event-driven (nothing else sends mail). Resend if a key is
	// configured, else the Noop provider so the app runs without credentials.
	var mailProvider mailer.Provider = mailer.Noop{}
	if cfg.ResendAPIKey != "" {
		mailProvider = mailer.NewResend(cfg.ResendAPIKey)
	}
	logger.Info("email provider", "provider", mailProvider.Name())
	dispatcher := mailer.NewDispatcher(mailProvider, queries, identity.NewMailDirectory(queries, cfg.ConsoleBaseURL), cfg.EmailFrom)
	go dispatcher.RunOutbox(ctx, 10*time.Second)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	github.NewHandler(queries, recorder, cfg.GithubWebhookSecret).Mount(mux)
	idHandlers := identity.NewHandlers(svc, sessions, authz, events.NewReader(queries), envs, metering.NewEmitter(queries))
	// One strict server, module handler sets composed by embedding (§15).
	idHandlers.Mount(mux, &apiServer{
		Handlers:  idHandlers,
		Handlers2: provisioning.NewHandlers(prov, authz, queries, svc, estimates.NewService(queries)),
	})

	// SSE sits BEFORE the strict server: strict handlers buffer; streams need
	// the raw ResponseWriter (x-streamable listEvents).
	streamer := &events.Streamer{
		Q: queries, Hub: hub, Envs: envs,
		Principal: svc.PrincipalFromRequest,
		Authorize: authz.Require,
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           problem.Recover(streamer.Intercept(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	logger.Info("api listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server exited", "err", err)
		os.Exit(1)
	}
}
