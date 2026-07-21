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
	// Desired returns services whose generation exceeds sinceGeneration.
	Desired(ctx context.Context, cell string, sinceGeneration int64) ([]DesiredService, error)
	// Report writes back observed status; a stale report is rejected by the
	// control plane, which the agent treats as "re-poll", never as an error to
	// act on locally.
	Report(ctx context.Context, cell string, r Report) error
}

// Renderer converges one service toward its desired state and returns the
// status it observes. This is the SEAM: at alpha it is a stub that acknowledges
// desired; T1.4/T3.4 replace it with real CNPG/K8s server-side-apply. The loop
// contract does not change when the renderer does.
//
// Converge MUST be idempotent — the loop may call it repeatedly for the same
// unchanged desired state, and a redundant converge must be a no-op.
type Renderer interface {
	Converge(ctx context.Context, svc DesiredService) (observedStatus string, err error)
}

// Agent is the poll→converge→writeback loop for one cell.
type Agent struct {
	cell   string
	cp     ControlPlane
	render Renderer
	log    *slog.Logger
	// watermark is the highest generation the agent has polled past. Advanced
	// only after a service both converges AND its status is accepted, so a
	// converge that fails is retried on the next pass rather than skipped.
	watermark int64
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
	services, err := a.cp.Desired(ctx, a.cell, a.watermark)
	if err != nil {
		// Cannot make changes. Do NOT touch actual state — degrade to read-only.
		a.log.Warn("desired poll failed; skipping convergence (control plane unreachable)",
			"cell", a.cell, "err", err)
		return
	}
	// The watermark may advance past a generation ONLY once that generation is
	// fully reconciled (converged AND reported). Generations are not contiguous
	// and a batch can succeed out of order (gen 4 done while gen 3 is stuck), so
	// tracking maxSucceeded alone would skip the stuck one forever. Instead cap
	// the advance below the lowest UNFINISHED generation. Re-polling a few
	// already-done services is harmless — converge is idempotent.
	maxSucceeded := a.watermark
	var minUnfinished int64 // 0 = nothing unfinished
	note := func(gen int64) {
		if minUnfinished == 0 || gen < minUnfinished {
			minUnfinished = gen
		}
	}
	for _, svc := range services {
		observed, err := a.render.Converge(ctx, svc)
		if err != nil {
			a.log.Error("converge failed", "service", svc.ID, "generation", svc.Generation, "err", err)
			note(svc.Generation)
			continue
		}
		rep := Report{ServiceID: svc.ID, ObservedGeneration: svc.Generation, Status: observed}
		if err := a.cp.Report(ctx, a.cell, rep); err != nil {
			// Writeback failed (network, or a stale-generation rejection). The
			// converge already happened and is idempotent, so retry the report
			// next pass — this generation is unfinished until the report lands.
			a.log.Warn("status writeback failed; will retry", "service", svc.ID, "err", err)
			note(svc.Generation)
			continue
		}
		if svc.Generation > maxSucceeded {
			maxSucceeded = svc.Generation
		}
	}
	if minUnfinished > 0 {
		// Hold just below the earliest unfinished generation, so it (and
		// everything after it) is re-polled next pass.
		a.watermark = minUnfinished - 1
	} else {
		a.watermark = maxSucceeded
	}
}
