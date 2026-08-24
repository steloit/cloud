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
	if err := run(context.Background(), os.Getenv, log); err != nil {
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
// parent is a parameter so a test can hand run an already-cancelled context.
// Without it, any mutation that lets a bad config through does not FAIL the test
// — it reaches a.Run and blocks until Go's 10m default timeout. Measured:
// mutation 33 (deleting the ValidateCell call) turned a 5-second suite into a
// 10-minute one at 0% CPU. A guard whose failure mode is a hang is still caught,
// but the most expensive way available.
//
// The signal handler is still installed HERE, after boot, exactly where it was
// before this refactor. Hoisting it into main() looked equivalent and was not: a
// SIGTERM arriving during boot would be absorbed by the context, boot would
// finish, a.Run would return at once and the process would exit **0** — an
// orchestrator reads that as "completed", where the default disposition would
// have killed it. Deriving from `parent` keeps production behaviour identical
// and still lets a cancelled parent short-circuit the loop.
// selectRenderer is extracted so the choice can be ASSERTED, not merely
// announced. A test that reads the log line cannot tell these apart, and all
// four were green: constructing the CNPG renderer and then handing the agent an
// Ack one (a silent fallback that logs "real apply" — verbatim the failure this
// branch exists to prevent); swapping the GSA and bucket arguments, which yields
// `destinationPath: gs://sa@…` and no workload identity, so no WAL archiving and
// NO PITR (ADR-0007 F3); hardcoding the cell; and passing a nil applier.
//
// Returning the renderer lets a test assert its concrete type and the values it
// was built with.
func selectRenderer(getenv func(string) string, cell string, log *slog.Logger) (agent.Renderer, error) {
	// US-3.3: in a cell, converge for real via the Kubernetes API; outside one,
	// fall back to the Ack renderer and SAY SO — a silent fallback would look
	// like a working agent that provisions nothing.
	kc, kerr := kube.NewInCluster()
	if kerr != nil {
		log.Warn("renderer: ACK (no cluster — desired state is acknowledged, NOTHING is provisioned)", "reason", kerr)
		return agent.NewAckRenderer(log), nil
	}
	gsa, wal := getenv("CELL_GSA_EMAIL"), getenv("CELL_WAL_BUCKET")
	apiCIDR := getenv("CELL_APISERVER_CIDR")
	if gsa == "" || wal == "" || apiCIDR == "" {
		// CELL_APISERVER_CIDR joins the same gate rather than defaulting: without
		// it the D7 egress policy cannot let CNPG's instance manager reach the
		// kube-apiserver, and the first Postgres pod on the cell never reaches
		// ready. A silent default would be a per-cluster value guessed wrong.
		return nil, fmt.Errorf("CELL_GSA_EMAIL, CELL_WAL_BUCKET and CELL_APISERVER_CIDR are " +
			"required in-cluster (customer DB pods need workload identity + a WAL bucket, and " +
			"the D7 egress policy needs the control plane's endpoint range)")
	}
	log.Info("renderer: CNPG (in-cluster, real apply)", "cell", cell, "wal_bucket", wal, "gsa", gsa)
	return render.NewCNPGRenderer(cnpg.New(), kc, cell, gsa, wal, apiCIDR, log), nil
}

func run(parent context.Context, getenv func(string) string, log *slog.Logger) error {
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

	renderer, err := selectRenderer(getenv, cell, log)
	if err != nil {
		return err
	}
	a := agent.New(cell, cp, renderer, log)

	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("cell-agent starting", "cell", cell, "control_plane", base, "interval", interval.String())
	a.Run(ctx, interval)
	return nil
}
