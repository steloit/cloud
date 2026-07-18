// The §15 composition root: config → pool → stores → services → handlers,
// explicitly, in order. Modules mount themselves; the mux stays dumb.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/password"
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

	svc, err := identity.NewService(queries, hasher, sessions, loginLimiter)
	if err != nil {
		logger.Error("boot failed", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	identity.NewHandlers(svc, sessions).Mount(mux)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           problem.Recover(mux),
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
