// The §15 composition root: config → pool → stores → services → handlers,
// explicitly, in order. Modules mount themselves; the mux stays dumb.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/password"
	"github.com/steloit/cloud/services/api/internal/identity/policy"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/config"
	"github.com/steloit/cloud/services/api/internal/platform/db"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
	"github.com/steloit/cloud/services/api/internal/platform/ratelimit"
)

var _ = gen.SessionInfo{} // anchor: the contract types this binary serves

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
	svc, err := identity.NewService(pool, hasher, sessions, loginLimiter, recorder)
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
	authz := identity.NewAuthorizer(queries, rbac.NewEvaluator(matrix, policies), recorder)
	envs := events.NoEnvs{} // env→org seam; real environments arrive with T3.2

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	identity.NewHandlers(svc, sessions, authz, events.NewReader(queries), envs).Mount(mux)

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
