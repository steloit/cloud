// Package render is the cell-agent's REAL renderer (US-3.3), replacing the
// AckRenderer stub: it renders a service's desired state to CNPG/K8s manifests
// via the T3.4 driver, server-side-applies them through the kube.Applier seam,
// observes the cluster to ready, and returns the reconciler status. This is the
// converge half of the reconciler loop wired to a real cluster (the Applier is
// fake in tests, real in Phase B).
package render

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/steloit/cloud/services/cell-agent/internal/agent"
	"github.com/steloit/cloud/services/cell-agent/internal/driver"
	"github.com/steloit/cloud/services/cell-agent/internal/kube"
)

// CNPGRenderer converges postgres services onto a cell via CNPG. It holds the
// per-cell placement the control plane does NOT know per-service (the workload
// identity SA and WAL bucket are cell-level); the per-service namespace comes
// from the desired doc's placement, resolved by the control plane.
type CNPGRenderer struct {
	pg        cnpgDriver
	applier   kube.Applier
	log       *slog.Logger
	cell      string // this agent's cell id
	gsaEmail  string // the cell's workload-identity SA for customer DB pods
	walBucket string // the cell's customer WAL bucket
}

// cnpgDriver is the subset of the T3.4 driver the renderer needs (an interface
// so the renderer is testable and the driver is swappable per product later).
type cnpgDriver interface {
	Render(driver.Spec) (driver.Manifests, error)
}

// NewCNPGRenderer — cell, gsaEmail, walBucket are the agent's own placement
// (it runs IN the cell with those credentials); only the per-service namespace
// comes from the control plane via the desired doc.
func NewCNPGRenderer(pg cnpgDriver, applier kube.Applier, cell, gsaEmail, walBucket string, log *slog.Logger) *CNPGRenderer {
	return &CNPGRenderer{pg: pg, applier: applier, log: log, cell: cell, gsaEmail: gsaEmail, walBucket: walBucket}
}

// placement is the control-plane-resolved location for a service, carried in the
// desired doc (US-3.3 extends US-1.3a's desiredDoc to populate it). The renderer
// never guesses a namespace — an unresolved placement is an error, not a default.
// namespaceOf reads the control-plane-resolved env namespace (proj--env) from
// the desired doc. Service-specific, so it must come from the control plane —
// the agent never guesses a namespace.
func namespaceOf(svc agent.DesiredService) (string, error) {
	ns, _ := svc.Desired["namespace"].(string)
	if ns == "" {
		return "", fmt.Errorf("render: service %s has no resolved namespace in desired", svc.ID)
	}
	return ns, nil
}

// Converge renders → applies → observes. A deleting service tears down and
// reports gone. Everything else applies the CNPG manifests and returns the
// observed status mapped to the reconciler vocabulary (ADR-024): the cluster is
// `provisioning` until CNPG reports a healthy phase, then `ready`.
func (r *CNPGRenderer) Converge(ctx context.Context, svc agent.DesiredService) (string, error) {
	namespace, err := namespaceOf(svc)
	if err != nil {
		return "", err
	}

	// Teardown: a deleting service (status or desired flag) is deleted and
	// reported gone — the same terminal the AckRenderer used, now with a real
	// delete behind the seam.
	if svc.Status == "deleting" || asBool(svc.Desired["deleting"]) {
		if err := r.applier.Delete(ctx, namespace, svc.ID); err != nil {
			return "", fmt.Errorf("render: delete %s: %w", svc.ID, err)
		}
		r.log.Info("converged: teardown applied", "service", svc.ID, "namespace", namespace)
		return "gone", nil
	}

	spec := driver.Spec{
		Name: svc.ID, Namespace: namespace, Product: svc.Product,
		Intent: asString(svc.Desired["intent"]), Shape: asMap(svc.Desired["shape"]),
		Instances: instancesOf(svc.Desired), Cell: r.cell,
		GSAEmail: r.gsaEmail, WALBucket: r.walBucket,
	}
	manifests, err := r.pg.Render(spec)
	if err != nil {
		return "", fmt.Errorf("render: %w", err)
	}
	objs := make([][]byte, len(manifests))
	for i, m := range manifests {
		objs[i] = m.YAML
	}
	if err := r.applier.Apply(ctx, namespace, objs); err != nil {
		return "", fmt.Errorf("render: apply %s: %w", svc.ID, err)
	}
	phase, err := r.applier.Observe(ctx, namespace, svc.ID)
	if err != nil {
		return "", fmt.Errorf("render: observe %s: %w", svc.ID, err)
	}
	return statusFromPhase(phase), nil
}

// statusFromPhase maps CNPG's cluster phase to the ADR-024 reconciler vocabulary.
// CNPG reports "Cluster in healthy state" when primary is accepting connections;
// anything else (empty/absent, setting up, failover) is still provisioning. A
// failed phase maps to failed so the control plane and metering see the truth.
func statusFromPhase(phase string) string {
	switch phase {
	case "Cluster in healthy state":
		return "ready"
	case "":
		return "provisioning" // not created yet / no status
	default:
		if isFailure(phase) {
			return "failed"
		}
		return "provisioning"
	}
}

func isFailure(phase string) bool {
	p := strings.ToLower(phase)
	for _, f := range []string{"failed", "failure", "error"} {
		if strings.Contains(p, f) {
			return true
		}
	}
	return false
}

// --- small, panic-free extractors from the untyped desired map ---

func asBool(v any) bool     { b, _ := v.(bool); return b }
func asString(v any) string { s, _ := v.(string); return s }
func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
func instancesOf(desired map[string]any) int {
	if o, ok := desired["override"].(map[string]any); ok {
		if n, ok := o["instances"].(float64); ok && n >= 1 {
			return int(n)
		}
	}
	return 1
}
