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
	// Objects names what a service owns without rendering it, so teardown cannot
	// fail for a reason that only applies to creating a volume (T3.4c).
	Objects(driver.Spec) driver.Manifests
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
// namespaceOf reads the control-plane-resolved env namespace (env-<id>, ADR-0012) from
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
	// reported gone. The objects are addressed by the names the DRIVER chose
	// (dnsName-sanitized), never the raw service id — they differ whenever an id
	// contains characters k8s rejects, and addressing the wrong name makes Delete
	// 404 → "already gone" → report gone while a real cluster keeps running.
	if svc.Status == "deleting" || asBool(svc.Desired["deleting"]) {
		objs, err := r.teardownObjects(svc, namespace)
		if err != nil {
			// A teardown that cannot enumerate its objects must NOT report gone —
			// that would mark the service deleted while its cluster keeps running.
			return "", fmt.Errorf("render: enumerate objects for teardown of %s: %w", svc.ID, err)
		}
		for _, o := range objs {
			if err := r.applier.Delete(ctx, namespace, o.Kind, o.Name); err != nil {
				return "", fmt.Errorf("render: delete %s/%s: %w", o.Kind, o.Name, err)
			}
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
	// Observe the CLUSTER by the name the driver gave it (manifests[0] is the
	// Cluster; see driver.Manifests ordering).
	phase, err := r.applier.Observe(ctx, namespace, manifests[0].Name)
	if err != nil {
		return "", fmt.Errorf("render: observe %s: %w", manifests[0].Name, err)
	}
	status := statusFromPhase(phase)
	// BLOCKER: the Renderer contract requires a TERMINAL status. Reporting a
	// transient `provisioning` would advance observed_generation, drop the row
	// out of the outstanding set (observed < generation), and the service would
	// never be re-polled to later report ready — on a real cell that is ~45s of
	// convergence turning into "never ready, no metering". Signal not-yet
	// instead: the agent skips the writeback and the row stays outstanding.
	if !terminal(status) {
		return "", fmt.Errorf("%w: %s is %q (phase %q)", ErrNotConverged, manifests[0].Name, status, phase)
	}
	return status, nil
}

// ErrNotConverged aliases the agent's sentinel so the loop can errors.Is it:
// the apply succeeded but the resource has not reached a terminal state yet.
var ErrNotConverged = agent.ErrNotConverged

func terminal(status string) bool {
	switch status {
	case "ready", "failed", "gone", "degraded":
		return true
	}
	return false
}

// teardownObjects returns the driver-canonical kind+name for a service's
// objects, so Delete addresses exactly what Apply created. It uses the REAL
// namespace and product — a fabricated placeholder would be sound only by
// accident, since nothing in the Driver contract promises namespace-independent
// names.
//
// The error return is currently UNREACHABLE: Driver.Objects names objects
// without rendering them and cannot fail. It is kept because the signature is
// the contract a future driver whose Objects needs I/O would use, and because
// the caller's "a teardown that cannot enumerate its objects must NOT report
// gone" arm is the behaviour we want if that ever becomes reachable.
func (r *CNPGRenderer) teardownObjects(svc agent.DesiredService, namespace string) (driver.Manifests, error) {
	return r.pg.Objects(driver.Spec{
		Name: svc.ID, Namespace: namespace, Product: svc.Product,
		Intent: asString(svc.Desired["intent"]), Shape: asMap(svc.Desired["shape"]),
		Instances: instancesOf(svc.Desired), Cell: r.cell,
		GSAEmail: r.gsaEmail, WALBucket: r.walBucket,
	}), nil
}

// statusFromPhase maps a CNPG cluster phase to the ADR-024 vocabulary using an
// EXPLICIT table of the operator's phase constants (api/v1/cluster_types.go).
//
// A substring heuristic ({failed,failure,error}) was tried and is unsound: it
// catches almost none of CNPG's real terminal-bad phases — "Cluster is
// unrecoverable and needs manual intervention", "Invalid cluster definition",
// "Unable to create required cluster objects" contain none of those words, so a
// permanently broken cluster read as `provisioning` and retried forever with no
// signal. Unknown phases fail CLOSED to `degraded` (visible, actionable) rather
// than to `provisioning` (invisible, retried forever).
var phaseStatus = map[string]string{
	// terminal-good
	"Cluster in healthy state": "ready",
	// transient (converging) — never terminal, never reported
	"":                   "provisioning",
	"Setting up primary": "provisioning",
	"Waiting for the instances to become active":   "provisioning",
	"Creating a new replica":                       "provisioning",
	"Primary instance is being restarted in-place": "provisioning",
	"Switchover in progress":                       "provisioning",
	"Failing over":                                 "provisioning",
	"Upgrading cluster":                            "provisioning",
	"Waiting for user action":                      "degraded",
	// terminal-bad (manual intervention or a definition error)
	"Cluster is unrecoverable and needs manual intervention":                                  "failed",
	"Invalid cluster definition":                                                              "failed",
	"Unable to create required cluster objects":                                               "failed",
	"Cluster has incomplete or invalid image catalog":                                         "failed",
	"Cluster cannot proceed to reconciliation due to an unknown plugin being required":        "failed",
	"Cluster cannot execute instance online upgrade due to missing architecture binary":       "failed",
	"Cluster cannot proceed to reconciliation due to an error while interacting with plugins": "failed",
}

func statusFromPhase(phase string) string {
	if st, ok := phaseStatus[phase]; ok {
		return st
	}
	// Unknown phase: fail closed to `failed`, NOT `degraded`. The transition
	// table allows provisioning → {ready, failed, deleting} but NOT
	// provisioning → degraded, and provisioning is exactly the state an unknown
	// phase is most likely to appear in — mapping to degraded would be rejected
	// by the status machine and the agent would retry the writeback forever,
	// which is invisible, the opposite of failing closed.
	return "failed"
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
