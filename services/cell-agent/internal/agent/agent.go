// Package agent is the data-plane half of the reconciler protocol (US-1.3,
// e1-substrate-design.md §2). It runs INSIDE a cell, polls the control plane
// for desired state, converges actual state, and reports back.
//
// The control plane is the source of truth for DESIRED; the cell is the source
// of truth for ACTUAL (A2.5). The loop is level-triggered: it renders from the
// full desired document every pass and never diffs by memory, so a dropped poll
// or an agent restart costs latency, never correctness.
//
// The single most important property (US-1.3 headline AC): when the control
// plane is unreachable, the loop logs the error and does NOTHING ELSE. It never
// tears down, degrades, or mutates a running workload on a failed poll — a
// control-plane outage must degrade to "cannot make changes", never "apps down".
package agent

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// DesiredService is one service's desired state, as the /desired endpoint
// returns it. Kept minimal and decoupled from the control plane's store types —
// the agent depends on the wire contract, not on the API's internals.
type DesiredService struct {
	ID                 string         `json:"id"`
	CellID             string         `json:"cell_id"`
	Product            string         `json:"product"`
	Status             string         `json:"status"`
	Generation         int64          `json:"generation"`
	ObservedGeneration int64          `json:"observed_generation"`
	Desired            map[string]any `json:"desired"`
}

// ErrNotConverged is returned by a Renderer whose apply succeeded but whose
// resource has not reached a terminal state yet. The loop treats it as "come
// back next tick", never as a failure, and never reports a transient status.
var ErrNotConverged = errors.New("agent: not converged yet")

// Report is one status writeback.
type Report struct {
	ServiceID          string `json:"service_id"`
	ObservedGeneration int64  `json:"observed_generation"`
	Status             string `json:"status"`
	Event              string `json:"event,omitempty"`
}

// ControlPlane is the API surface the agent consumes. An interface so the loop
// is testable without a live server and so the transport (HTTP now, long-poll
// or SSE later) is a swappable detail, exactly as §2 anticipates.
type ControlPlane interface {
	// Desired returns the cell's outstanding work: services whose generation
	// exceeds sinceGeneration, and environments whose namespace must be removed.
	Desired(ctx context.Context, cell string, sinceGeneration int64) (DesiredState, error)
	// Report writes back observed status; a stale report is rejected by the
	// control plane, which the agent treats as "re-poll", never as an error to
	// act on locally.
	Report(ctx context.Context, cell string, r Report) error
	// ConfirmEnvironmentTeardown reports that an environment's namespace is
	// gone. Separate from Report because a namespace belongs to no service —
	// Report requires a service id and writes a per-service row.
	ConfirmEnvironmentTeardown(ctx context.Context, cell, envID string) error
}

// DesiredState is the whole poll answer: per-service work, and the environments
// whose namespace must be removed.
type DesiredState struct {
	Services     []DesiredService             `json:"services"`
	Environments []DesiredEnvironmentTeardown `json:"environments"`
}

// DesiredEnvironmentTeardown is one environment whose namespace must go.
//
// The namespace is GIVEN, never derived here. The control plane resolved it
// (ADR-0012) and is the only place that knows it exactly; US-3.3a shipped a
// second, agent-side derivation and it named nothing the control plane knew.
type DesiredEnvironmentTeardown struct {
	ID        string `json:"id"`
	Namespace string `json:"namespace"`
}

// Renderer converges one service toward its desired state and returns the
// status it observes. This is the SEAM: at alpha it is a stub that acknowledges
// desired; T1.4/T3.4 replace it with real CNPG/K8s server-side-apply. The loop
// contract does not change when the renderer does.
//
// Converge MUST be idempotent — the loop may call it repeatedly for the same
// unchanged desired state, and a redundant converge must be a no-op.
//
// It MUST return a TERMINAL observed status (ready/failed/gone/…), never a
// transient one. The control plane marks the reported generation observed, so a
// row that reports a non-terminal status (e.g. "provisioning" while a real CNPG
// cluster is still spinning up) drops out of the outstanding set and is never
// re-polled to later report "ready". A real renderer (T1.4/T3.4) that needs to
// report progress must either block until terminal or grow its own
// observation-only reporting path — the alpha AckRenderer only returns terminal
// statuses, so this constraint is not yet load-bearing but the seam must honor it.
type Renderer interface {
	Converge(ctx context.Context, svc DesiredService) (observedStatus string, err error)
}

// EnvironmentTeardowner removes an environment's cluster-scoped objects — today
// its namespace, which takes everything inside it with it.
//
// A SEPARATE interface, not a method on Renderer, for the reason valkey is not a
// BranchingDriver: not every renderer owns environment-scoped objects, and the
// alpha AckRenderer owns none. The loop skips the environment half entirely when
// its renderer does not implement this, rather than requiring every renderer to
// carry a no-op.
//
// MUST be idempotent: a namespace already gone is success, not an error. The
// loop may call it repeatedly, because a confirmation that fails to reach the
// control plane leaves the environment outstanding for the next tick.
type EnvironmentTeardowner interface {
	TeardownEnvironment(ctx context.Context, namespace string) error
}

// Agent is the poll→converge→writeback loop for one cell.
//
// It holds NO mutable reconciliation state: each Tick asks the control plane
// for outstanding work (services whose observed generation trails desired) and
// converges what it gets. There is no client-side watermark — a per-row
// generation cannot be a cell-wide cursor without starving new services, and
// the control plane already knows what needs reconciling. A dropped poll, an
// agent restart, or a converge that fails all self-heal on the next tick
// because the work is still outstanding server-side. Tick is therefore safe to
// call concurrently, though Run drives it single-threaded.
type Agent struct {
	cell   string
	cp     ControlPlane
	render Renderer
	log    *slog.Logger
}

func New(cell string, cp ControlPlane, render Renderer, log *slog.Logger) *Agent {
	return &Agent{cell: cell, cp: cp, render: render, log: log}
}

// Run loops until ctx is cancelled. interval is the poll cadence (alpha: a
// fixed poll; long-poll/SSE is a later transport swap behind ControlPlane).
func (a *Agent) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	a.Tick(ctx) // converge once immediately, don't wait a full interval
	for {
		select {
		case <-ctx.Done():
			a.log.Info("agent stopping", "cell", a.cell)
			return
		case <-t.C:
			a.Tick(ctx)
		}
	}
}

// Tick is one poll→converge→writeback pass. Exported so a test can drive single
// passes deterministically without a real clock.
//
// A poll failure returns EARLY, before any convergence — this is the
// control-plane-outage guarantee in code. Nothing about a running workload
// depends on the control plane being reachable.
func (a *Agent) Tick(ctx context.Context) {
	// since_generation=0: ask for ALL outstanding work in the cell. The server
	// filters on observed_generation < generation, so there is no cursor to
	// advance and nothing to starve — a service converged last tick simply
	// stops appearing once its report lands.
	state, err := a.cp.Desired(ctx, a.cell, 0)
	if err != nil {
		// Cannot make changes. Do NOT touch actual state — degrade to read-only.
		a.log.Warn("desired poll failed; skipping convergence (control plane unreachable)",
			"cell", a.cell, "err", err)
		return
	}
	for _, svc := range state.Services {
		observed, err := a.render.Converge(ctx, svc)
		if err != nil {
			// ErrNotConverged is NOT a failure: the apply landed and the
			// resource is still becoming ready. Skipping the writeback keeps
			// the row outstanding server-side so the next tick re-observes it —
			// reporting a transient status here would drop it from the
			// outstanding set and it would never reach ready.
			if errors.Is(err, ErrNotConverged) {
				a.log.Info("converging", "service", svc.ID, "generation", svc.Generation, "detail", err)
				continue
			}
			// A real failure. Also stays outstanding, retried next tick.
			a.log.Error("converge failed", "service", svc.ID, "generation", svc.Generation, "err", err)
			continue
		}
		rep := Report{ServiceID: svc.ID, ObservedGeneration: svc.Generation, Status: observed}
		if err := a.cp.Report(ctx, a.cell, rep); err != nil {
			// Writeback failed (network, or a generation-mismatch rejection —
			// desired moved while we converged). Converge is idempotent and the
			// row stays outstanding, so the next tick re-polls and reports the
			// current generation.
			a.log.Warn("status writeback failed; will retry", "service", svc.ID, "err", err)
			continue
		}
	}
	a.tearDownEnvironments(ctx, state.Environments)
}

// tearDownEnvironments removes the namespace of each environment the control
// plane says is finished with.
//
// IT RUNS AFTER THE SERVICES, and that ordering is belt-and-braces rather than
// the guarantee: deleting a namespace deletes everything in it, so the real
// protection is server-side — an environment is not advertised until every
// service in it is actually gone (status `deleting` AND observed caught up), so
// a still-terminating database can never be inside one of these. Running last
// costs nothing and means a service teardown that lands this same tick is
// already done.
func (a *Agent) tearDownEnvironments(ctx context.Context, envs []DesiredEnvironmentTeardown) {
	if len(envs) == 0 {
		return
	}
	td, ok := a.render.(EnvironmentTeardowner)
	if !ok {
		// A renderer that owns no environment-scoped objects (the alpha
		// AckRenderer) cannot tear one down. Loud rather than silent: the
		// control plane is asking for something this agent cannot do, and the
		// environment will stay outstanding forever until someone notices.
		a.log.Error("control plane asked for an environment teardown and this renderer cannot "+
			"perform one; the namespaces will leak",
			"cell", a.cell, "environments", len(envs))
		return
	}
	for _, env := range envs {
		if err := td.TeardownEnvironment(ctx, env.Namespace); err != nil {
			// Stays outstanding server-side: the confirmation is what stops it
			// being advertised, and we are not sending one.
			a.log.Error("environment teardown failed; will retry",
				"environment", env.ID, "namespace", env.Namespace, "err", err)
			continue
		}
		if err := a.cp.ConfirmEnvironmentTeardown(ctx, a.cell, env.ID); err != nil {
			// The namespace IS gone; only the confirmation failed. Next tick
			// re-advertises it, the teardown is idempotent, and the confirmation
			// is retried — which is why TeardownEnvironment must treat an absent
			// namespace as success.
			a.log.Warn("environment teardown confirmation failed; will retry",
				"environment", env.ID, "err", err)
			continue
		}
		a.log.Info("environment namespace removed",
			"environment", env.ID, "namespace", env.Namespace)
	}
}
