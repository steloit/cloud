// Command cell-agent runs the data-plane reconciler loop inside one cell
// (US-1.3). 12-factor config; fail fast on missing required values.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/steloit/cloud/services/cell-agent/internal/agent"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cell := env("RECONCILER_CELL", "cell-0")
	base := os.Getenv("CONTROL_PLANE_URL")
	token := os.Getenv("RECONCILER_SECRET")
	if base == "" || token == "" {
		log.Error("boot failed: CONTROL_PLANE_URL and RECONCILER_SECRET are required")
		os.Exit(1)
	}
	interval := 10 * time.Second
	if v := os.Getenv("POLL_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = time.Duration(n) * time.Second
		}
	}

	cp := agent.NewHTTPControlPlane(base, token)
	a := agent.New(cell, cp, agent.NewAckRenderer(log), log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("cell-agent starting", "cell", cell, "control_plane", base, "interval", interval.String())
	a.Run(ctx, interval)
	log.Info("cell-agent stopped")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
