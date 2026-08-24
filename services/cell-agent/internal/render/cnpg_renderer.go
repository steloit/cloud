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
	"github.com/steloit/cloud/services/cell-agent/internal/driver/tenancy"
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

	// apiServerCIDR is the cell's control-plane endpoint range, for the D7
	// egress policy that lets CNPG's instance manager reach the kube-apiserver.
	// A CELL property, not a per-service one, and substrate detail (D8) — so it
	// rides the agent's own config, never the desired doc.
	apiServerCIDR string
}

// cnpgDriver is the subset of the T3.4 driver the renderer needs (an interface
// so the renderer is testable and the driver is swappable per product later).
type cnpgDriver interface {
	Render(driver.Spec) (driver.Manifests, error)
	// Objects names what a service owns without rendering it, so teardown cannot
	// fail for a reason that only applies to creating a volume (T3.4c).
	Objects(driver.Spec) (driver.Manifests, error)
}

// NewCNPGRenderer — cell, gsaEmail, walBucket are the agent's own placement
// (it runs IN the cell with those credentials); only the per-service namespace
// comes from the control plane via the desired doc.
func NewCNPGRenderer(pg cnpgDriver, applier kube.Applier, cell, gsaEmail, walBucket, apiServerCIDR string, log *slog.Logger) *CNPGRenderer {
	// A nil applier is a wiring bug that would otherwise surface as a nil deref
	// on the first converge, on a cell, at 3am. Building one is a programming
	// error, so panic here rather than return an error nobody would check.
	if applier == nil {
		panic("render: CNPGRenderer built with a nil applier")
	}
	return &CNPGRenderer{pg: pg, applier: applier, log: log, cell: cell,
		gsaEmail: gsaEmail, walBucket: walBucket, apiServerCIDR: apiServerCIDR}
}

// Placement reports the per-cell values this renderer was built with.
//
// Exported so a CALLER's wiring can be asserted rather than inferred from a log
// line: swapping the gsaEmail and walBucket arguments at the construction site
// renders `destinationPath: gs://sa@…` and an empty workload-identity
// annotation — no WAL archiving and no PITR (ADR-0007 F3) — while every log line
// still reads correctly.
func (r *CNPGRenderer) Placement() (cell, gsaEmail, walBucket string) {
	return r.cell, r.gsaEmail, r.walBucket
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
	// Validated HERE, not inside tenancy.Render, because Converge's deleting
	// branch returns before Render is reached — so a check living in Render
	// guards the create path only, and teardown is the path that interpolates
	// this value into a DELETE URL. One owner, both paths.
	if err := tenancy.ValidateNamespace(ns); err != nil {
		return "", fmt.Errorf("render: service %s: %w", svc.ID, err)
	}
	return ns, nil
}

// quotaOf reads the plan's per-environment envelope out of the desired doc.
//
// The control plane resolves it from the org's plan (billing.Table.Envelope) and
// ships the VALUES; the agent never sees a plan name and holds no copy of the
// plan table. An absent or partial envelope is refused by tenancy.Render rather
// than defaulted — a missing ceiling is the failure this exists to prevent.
func quotaOf(svc agent.DesiredService) tenancy.Quota {
	q := asMap(svc.Desired["quota"])
	return tenancy.Quota{
		CPU:     asString(q["cpu"]),
		Memory:  asString(q["memory"]),
		Storage: asString(q["storage"]),
	}
}

// tenancyManifests renders the environment's namespace.
//
// The namespace is passed through, never re-derived: it is the single value the
// control plane resolved (ADR-0012, env-<environment_id>), and tenancy.Render
// refuses anything that is not in that shape rather than trusting this caller.
func (r *CNPGRenderer) tenancyManifests(namespace string, svc agent.DesiredService) ([][]byte, error) {
	objs, err := tenancy.Render(tenancy.Spec{
		Namespace:     namespace,
		Cell:          r.cell,
		APIServerCIDR: r.apiServerCIDR,
		Quota:         quotaOf(svc),
	})
	if err != nil {
		return nil, fmt.Errorf("render: tenancy for %s: %w", namespace, err)
	}
	yamls := make([][]byte, len(objs))
	for i, o := range objs {
		yamls[i] = o.YAML
	}
	return yamls, nil
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
		// A DELETE THAT WAS ACCEPTED IS NOT A WORKLOAD THAT IS GONE.
		//
		// Kubernetes answers 2xx the MOMENT it accepts a delete, with finalizers
		// and graceful termination still pending — and a CNPG Cluster has both.
		// Reporting `gone` on the strength of that 2xx reports acceptance as
		// completion, and everything downstream reads `gone` as absence:
		// US-3.3h converges `deleting` + `gone`, and US-3.3b then advertises the
		// environment's NAMESPACE for deletion, which would remove a database
		// whose pods are still terminating and whose WAL is still archiving.
		//
		// So observe it. "" is a real 404 — the Cluster is actually gone — and
		// anything else means still terminating, which is ErrNotConverged: the
		// apply landed, the row stays outstanding, and the next tick re-checks.
		// That is the same shape the provisioning path already uses, arrived at
		// from the other end.
		// The name is taken from the objects we just deleted, NOT re-derived:
		// they are dnsName-sanitized by the driver, and observing a name the
		// driver did not use would 404 and read as "gone" — the same
		// wrong-name-reads-as-absent trap the comment above this block names.
		var cluster string
		for _, o := range objs {
			if o.Kind == "Cluster" {
				cluster = o.Name
			}
		}
		if cluster == "" {
			return "", fmt.Errorf("render: teardown of %s enumerated no Cluster to observe — "+
				"reporting gone would be reporting a delete nobody confirmed", svc.ID)
		}
		if phase, err := r.applier.Observe(ctx, namespace, cluster); err != nil {
			return "", fmt.Errorf("render: observe %s during teardown: %w", svc.ID, err)
		} else if phase != "" {
			return "", fmt.Errorf("%w: %s is still terminating (%s)", agent.ErrNotConverged, svc.ID, phase)
		}
		r.log.Info("converged: teardown complete", "service", svc.ID, "namespace", namespace)
		return "gone", nil
	}

	// THE ENV NAMESPACE AND ITS D7 BOUNDARY COME FIRST (US-3.3a).
	//
	// Nothing created this namespace before: not the control plane (which has no
	// kube dependency at all), not terraform (which makes only cnpg-system and
	// control-plane), not the agent. The live e2e worked because a runbook ran
	// `kubectl create ns` in preflight, so a genuinely new project/env would 404
	// on first apply.
	//
	// D7 also requires the namespace to CARRY default-deny NetworkPolicies, a
	// ResourceQuota and a LimitRange. The quota and the LimitRange ARE rendered
	// now (US-3.3e), from the envelope the control plane resolved and put in this
	// service's desired doc. The NetworkPolicies are still withheld — see the
	// tenancy package doc for the reason and task/US-3.3f for the enforcement.
	//
	// NOTE THE ASYMMETRY, because it is the reason US-3.3g exists: the namespace
	// and its quota are ENVIRONMENT-scoped, but they are rendered from a SERVICE's
	// doc, and every service in the environment renders them. Sibling docs written
	// either side of a plan change disagree, and the namespace then carries
	// whichever service converged last — the quota oscillates rather than merely
	// going stale.
	//
	// Applied on every converge, not once: SSA is idempotent, and level-triggered
	// means a namespace or policy deleted out from under us comes back.
	tenancyObjs, err := r.tenancyManifests(namespace, svc)
	if err != nil {
		return "", err
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
	// ONE Apply, in order: the namespace, then the service.
	// Apply iterates in slice order, so the ordering invariant lives in
	// tenancy.Render's documented ordering rather than being re-established here
	// — and a single call keeps "how many times did we apply" meaning one
	// converge, which is what the retry accounting asserts.
	objs := make([][]byte, 0, len(tenancyObjs)+len(manifests))
	objs = append(objs, tenancyObjs...)
	for _, m := range manifests {
		objs = append(objs, m.YAML)
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
// The error return is REACHABLE, and an earlier revision of this comment said it
// was not. Objects carries Driver.Render's two entry guards — the product check
// and requireName — because dropping them here is what let a `valkey` service in
// `deleting` converge to "gone" while deleting postgres-shaped objects. There is
// one renderer for all four products, so the caller's "a teardown that cannot
// enumerate its objects must NOT report gone" arm is live, not aspirational.
func (r *CNPGRenderer) teardownObjects(svc agent.DesiredService, namespace string) (driver.Manifests, error) {
	return r.pg.Objects(driver.Spec{
		Name: svc.ID, Namespace: namespace, Product: svc.Product,
		Intent: asString(svc.Desired["intent"]), Shape: asMap(svc.Desired["shape"]),
		Instances: instancesOf(svc.Desired), Cell: r.cell,
		GSAEmail: r.gsaEmail, WALBucket: r.walBucket,
	})
}

// statusFromPhase maps a CNPG cluster phase to the ADR-024 vocabulary using an
// EXPLICIT table of the operator's phase constants (api/v1/cluster_types.go).
//
// A substring heuristic ({failed,failure,error}) was tried and is unsound: it
// catches almost none of CNPG's real terminal-bad phases — "Cluster is
// unrecoverable and needs manual intervention", "Invalid cluster definition",
// "Unable to create required cluster objects" contain none of those words, so a
// permanently broken cluster read as `provisioning` and retried forever with no
// signal. Unknown phases fail CLOSED to `failed` (visible, actionable) rather
// than to `provisioning` (invisible, retried forever) — see statusFromPhase
// below, which explains why `failed` and not `degraded`. This comment said
// `degraded` and contradicted both the code and that explanation.
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
	// `failed`, NOT `degraded`. The control plane's status machine allows
	// provisioning → {ready, failed, deleting} only; degraded is reachable from
	// `ready`. An agent that answers a still-provisioning cluster with `degraded`
	// has its writeback REJECTED every tick, so observed_generation never
	// advances and the row is retried forever — the exact invisible-retry failure
	// statusFromPhase argues against thirty lines below, arrived at from the
	// other side.
	"Waiting for user action": "failed",
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

// TeardownEnvironment removes an environment's cluster-scoped objects — today
// the namespace, which takes the ResourceQuota, the LimitRange and every
// workload inside it along with it.
//
// SAFE ONLY BECAUSE THE CONTROL PLANE GATES IT. Deleting a namespace deletes
// everything in it, including a database. What makes that acceptable is
// server-side: ListEnvironmentTeardownsForCell does not advertise an environment
// until every service in it is `deleting` AND has caught up its observed
// generation, so a still-terminating CNPG cluster can never be inside one of
// these. This function deliberately does NOT re-check that — it has no view of
// the control plane's intent and a second, weaker copy of the rule would be
// worse than none. It removes what it is told to remove.
//
// IDEMPOTENT: kube.Delete maps a 404 to success, so a namespace already gone is
// not an error. That matters because the confirmation can fail after the delete
// succeeded, and the next tick will re-run this.
func (r *CNPGRenderer) TeardownEnvironment(ctx context.Context, namespace string) error {
	// The namespace arrives over the wire and is interpolated into a request
	// path, so it MUST be validated before anything is deleted — a teardown is
	// the one operation where a wrong-but-plausible value is unrecoverable.
	//
	// That validation is TeardownObjects', not a second one here: it renders the
	// real object set, and Render refuses an invalid namespace before producing
	// anything. An explicit ValidateNamespace call above this line was measured
	// to be an equivalent mutant — removing it failed no test, because nothing
	// can reach a Delete without going through Render first. A guard nothing can
	// distinguish is not a guard.
	objs, err := tenancy.TeardownObjects(namespace)
	if err != nil {
		return err
	}
	for _, o := range objs {
		// The namespace is passed as the SCOPE, which resourcePath ignores for a
		// cluster-scoped kind — it is the object's own name that addresses it.
		if err := r.applier.Delete(ctx, namespace, o.Kind, o.Name); err != nil {
			return fmt.Errorf("render: delete %s/%s for environment teardown: %w",
				o.Kind, o.Name, err)
		}
		r.log.Info("environment object deleted", "kind", o.Kind, "name", o.Name,
			"namespace", namespace)
	}
	return nil
}
