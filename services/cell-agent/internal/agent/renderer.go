package agent

import (
	"context"
	"log/slog"
)

// AckRenderer is the ALPHA renderer: it acknowledges desired state and reports
// the service ready without provisioning anything. It exists so the protocol —
// poll, converge, writeback, watermark, the outage guarantee — is provable and
// running end-to-end before any substrate exists, keeping the trial burn at ~$0
// (US-1.3 is scoped to the protocol; T1.4/T3.4 own real CNPG/K8s rendering).
//
// It is a legitimate Renderer, not a mock: it is idempotent, it is what runs in
// the dev cell today, and the seam it fills is the exact one the real renderer
// replaces. When that lands, this file is deleted and nothing else changes.
type AckRenderer struct{ log *slog.Logger }

func NewAckRenderer(log *slog.Logger) *AckRenderer { return &AckRenderer{log: log} }

// Converge acknowledges desired and reports the terminal status the desired
// lifecycle implies. A service whose desired state is a deletion converges to
// gone; everything else converges to ready. Idempotent by construction — it
// holds no state and derives its answer purely from the input.
func (r *AckRenderer) Converge(_ context.Context, svc DesiredService) (string, error) {
	if deleting, _ := svc.Desired["deleting"].(bool); deleting {
		r.log.Info("converged (ack): teardown acknowledged", "service", svc.ID, "generation", svc.Generation)
		return "gone", nil
	}
	r.log.Info("converged (ack): desired acknowledged", "service", svc.ID, "product", svc.Product, "generation", svc.Generation)
	return "ready", nil
}
