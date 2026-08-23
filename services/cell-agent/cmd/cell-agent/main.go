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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, log); err != nil {
		log.Error("boot failed", "err", err)
		os.Exit(1)
	}
	log.Info("cell-agent stopped")
}

// run holds everything main() used to, so that main() is four lines nothing can
// hide in. Extracting bootConfig alone was not enough: the validator, the call
// to it, and main HONOURING its error are three representations, and replacing
// `if err != nil { os.Exit(1) }` with `_ = err` was still a green change — the
// agent would then boot with an empty cell, base and token, which is the exact
// "every converge fails and nothing is written back" failure the guard exists
// to prevent.
// ctx is a parameter, not created inside, so a test can hand run an
// already-cancelled one. Without that, any mutation that lets a bad config
// through does not FAIL the test — it reaches a.Run and blocks until Go's 10m
// test timeout. Measured: mutation 33 (deleting the ValidateCell call) turned a
// 5-second suite into a 10-minute one. A guard whose failure mode is a hang is
// still caught, but it is caught the most expensive way available.
func run(ctx context.Context, getenv func(string) string, log *slog.Logger) error {
	cell, base, token, err := bootConfig(getenv)
	if err != nil {
		return err
	}
	interval := 10 * time.Second
	if v := getenv("POLL_INTERVAL_SECONDS"); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n > 0 {
			interval = time.Duration(n) * time.Second
		}
	}

	cp := agent.NewHTTPControlPlane(base, token)

	// Renderer selection (US-3.3): in a cell, converge for real via the
	// Kubernetes API; outside one, fall back to the Ack renderer and SAY SO —
	// a silent fallback would look like a working agent that provisions nothing.
	var renderer agent.Renderer
	if kc, kerr := kube.NewInCluster(); kerr == nil {
		gsa, wal := getenv("CELL_GSA_EMAIL"), getenv("CELL_WAL_BUCKET")
		if gsa == "" || wal == "" {
			return fmt.Errorf("CELL_GSA_EMAIL and CELL_WAL_BUCKET are required in-cluster " +
				"(customer DB pods need workload identity + a WAL bucket)")
		}
		renderer = render.NewCNPGRenderer(cnpg.New(), kc, cell, gsa, wal, log)
		log.Info("renderer: CNPG (in-cluster, real apply)", "cell", cell, "wal_bucket", wal)
	} else {
		renderer = agent.NewAckRenderer(log)
		log.Warn("renderer: ACK (no cluster — desired state is acknowledged, NOTHING is provisioned)", "reason", kerr)
	}
	a := agent.New(cell, cp, renderer, log)

	log.Info("cell-agent starting", "cell", cell, "control_plane", base, "interval", interval.String())
	a.Run(ctx, interval)
	return nil
}
