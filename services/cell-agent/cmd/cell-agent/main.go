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
	"github.com/steloit/cloud/services/cell-agent/internal/driver/cnpg"
	"github.com/steloit/cloud/services/cell-agent/internal/kube"
	"github.com/steloit/cloud/services/cell-agent/internal/render"
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

	// Renderer selection (US-3.3): in a cell, converge for real via the
	// Kubernetes API; outside one, fall back to the Ack renderer and SAY SO —
	// a silent fallback would look like a working agent that provisions nothing.
	var renderer agent.Renderer
	if kc, err := kube.NewInCluster(); err == nil {
		gsa, wal := os.Getenv("CELL_GSA_EMAIL"), os.Getenv("CELL_WAL_BUCKET")
		if gsa == "" || wal == "" {
			log.Error("boot failed: CELL_GSA_EMAIL and CELL_WAL_BUCKET are required in-cluster (customer DB pods need workload identity + a WAL bucket)")
			os.Exit(1)
		}
		renderer = render.NewCNPGRenderer(cnpg.New(), kc, cell, gsa, wal, log)
		log.Info("renderer: CNPG (in-cluster, real apply)", "cell", cell, "wal_bucket", wal)
	} else {
		renderer = agent.NewAckRenderer(log)
		log.Warn("renderer: ACK (no cluster — desired state is acknowledged, NOTHING is provisioned)", "reason", err)
	}
	a := agent.New(cell, cp, renderer, log)

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
