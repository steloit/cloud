// Command cell-agent runs the data-plane reconciler loop inside one cell
// (US-1.3). 12-factor config; fail fast on missing required values.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/steloit/cloud/services/cell-agent/internal/agent"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/cnpg"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/tenancy"
	"github.com/steloit/cloud/services/cell-agent/internal/kube"
	"github.com/steloit/cloud/services/cell-agent/internal/render"
)

// bootConfig reads and VALIDATES the environment before anything uses it. It
// takes getenv so the wiring is testable: main() itself is not, and a validation
// that is only reachable from main() is a validation nothing pins — deleting the
// ValidateCell call left the whole suite green, because cmd/cell-agent had no
// test files at all.
//
// The cell id is validated AT BOOT, not at converge, because it becomes a label
// value on every environment namespace and tenancy.Render refuses one that is not
// an RFC1123 label. Without this a plausible typo — RECONCILER_CELL=cell_0 or
// Cell-0 — starts cleanly and then fails EVERY converge for EVERY service on the
// cell; the agent loop logs and continues, so nothing is written back and the
// control plane sees every service sit in provisioning forever.
func bootConfig(getenv func(string) string) (cell, base, token string, err error) {
	cell = getenv("RECONCILER_CELL")
	if cell == "" {
		cell = "cell-0"
	}
	if e := tenancy.ValidateCell(cell); e != nil {
		return "", "", "", fmt.Errorf("RECONCILER_CELL=%q is not usable as a label value: %w", cell, e)
	}
	base, token = getenv("CONTROL_PLANE_URL"), getenv("RECONCILER_SECRET")
	if base == "" || token == "" {
		return "", "", "", fmt.Errorf("CONTROL_PLANE_URL and RECONCILER_SECRET are required")
	}
	return cell, base, token, nil
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cell, base, token, err := bootConfig(os.Getenv)
	if err != nil {
		log.Error("boot failed", "err", err)
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
