// Package tenancy renders an environment's Kubernetes namespace.
//
// WHY THE AGENT AND NOT THE CONTROL PLANE — US-3.3a asked for this decision to
// be made and recorded. The control plane has NO Kubernetes dependency:
// services/api/go.mod contains no k8s.io/* at all, because the two-plane split
// (D6, frozen by ADR-0001) is that the control plane writes desired state and
// the cell-agent converges it. Creating the namespace control-plane-side would
// mean giving the control plane cluster credentials and a kube client — an
// architecture delta, not an implementation choice, and ADR-040 says a delta
// needs "implementation, performance, customer, or security evidence" — none of
// which this has.
//
// Agent-side is also level-triggered for free: a namespace deleted out from
// under us is recreated on the next converge, where a create-once call at
// environment creation would never notice.
//
// WHAT THIS PACKAGE DELIBERATELY DOES NOT RENDER — D7 (INF-001 §1) also calls
// for "default-deny NetworkPolicies, ResourceQuota, LimitRange". US-3.3a shipped
// all of them and the security review found each one either inert or harmful:
//
//   - NetworkPolicies are NOT ENFORCED on the cells we build. infra/modules/
//     gke-cell creates a GKE Standard cluster with no network_policy block and no
//     ADVANCED_DATAPATH, so the API server stores every policy and no packet is
//     ever dropped. Shipping them would put a green suite and a "D7 done" behind
//     a boundary that does not exist.
//   - Worse, the allow-set as written denies what CNPG requires — the metadata
//     server (Workload Identity), GCS (WAL archiving) and the apiserver (the
//     in-pod instance manager). Turning enforcement on would fence the first
//     Postgres pod before it reached ready.
//   - The LimitRange default (500m/512Mi) IS enforced, at admission, and the
//     Cluster template declares no resources of its own — so it would silently
//     become the hard cap on every managed Postgres, existing ones included.
//   - The ResourceQuota envelope is a product decision with no owner in
//     docs/founder-config.md and no dependence on plan.
//
// They are one change, not four: US-3.3c lands enforcement, the CNPG allow-set
// and a founder-owned envelope together, because any one of them alone is either
// a no-op or an outage.
package tenancy

import (
	"fmt"
	"regexp"
)

// rfc1123Label is what the API server will accept as a namespace name, and it is
// also the guard on a value that gets interpolated into a manifest and
// fmt.Sprintf'd into a request path. A namespace carrying a newline injects
// arbitrary YAML keys into the object; one carrying `../` walks out of the path
// it was supposed to address.
//
// NOTE ON WHAT THIS DOES AND DOES NOT BUY: refusing locally is not better than
// the API server refusing, in signalling terms — both produce a converge that
// errors, logs and retries, with no writeback and no status change for the
// customer. What it buys is that the malformed value never reaches a URL or a
// YAML document. Surfacing a terminal `failed` writeback is a separate gap and
// is not claimed here.
// quantity is the closed grammar the control plane emits (billing.validQuantity
// accepts whole cores and whole Mi/Gi/Ti). Re-checked HERE because this module
// interpolates the value into a manifest: the two ends of the wire are separate
// modules, and a value that got past one is not a value the other may assume.
var quantity = regexp.MustCompile(`^[1-9][0-9]*(Mi|Gi|Ti)?$`)

var rfc1123Label = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// ValidateNamespace is the ONE owner of "is this a namespace we will act on".
//
// It is exported because the teardown path needs the same answer: Converge's
// deleting branch returns before Render is ever called, so a namespace validated
// only inside Render is validated on the create path alone — and teardown is the
// path that fmt.Sprintf's the value into a DELETE URL.
func ValidateNamespace(ns string) error {
	if ns == "" {
		return fmt.Errorf("tenancy: namespace is required")
	}
	if len(ns) > 63 || !rfc1123Label.MatchString(ns) {
		return fmt.Errorf("tenancy: namespace %q is not an RFC1123 label", ns)
	}
	if len(ns) < 5 || ns[:4] != "env-" {
		return fmt.Errorf("tenancy: namespace %q is not env-<environment_id> (ADR-0012)", ns)
	}
	return nil
}

// ValidateCell is the ONE owner of "is this cell id usable". It is exported so
// the agent can refuse a bad RECONCILER_CELL at boot rather than failing every
// converge on the cell with no control-plane signal.
func ValidateCell(cell string) error {
	if cell == "" {
		return fmt.Errorf("tenancy: cell is required — every row carries cell_id (D7)")
	}
	if len(cell) > 63 || !rfc1123Label.MatchString(cell) {
		return fmt.Errorf("tenancy: cell %q is not an RFC1123 label", cell)
	}
	return nil
}

// Spec is what an environment needs in order to become a tenant boundary.
type Spec struct {
	Namespace string // env-<environment_id> (ADR-0012)
	Cell      string
	Quota     Quota // the plan's per-environment envelope, resolved by the control plane
}

// Quota is the per-environment resource envelope, as Kubernetes quantity
// strings. The control plane resolves it from the org's plan and ships the
// VALUES in the desired doc; this module never sees a plan name and holds no
// copy of plans.json — the same boundary as pricing.
type Quota struct {
	CPU     string // cores, e.g. "8"
	Memory  string // e.g. "16Gi"
	Storage string // total PVC capacity, e.g. "100Gi"
}

// Set reports whether an envelope was supplied at all.
func (q Quota) Set() bool { return q.CPU != "" || q.Memory != "" || q.Storage != "" }

// Manifest is one rendered object.
type Manifest struct {
	Kind, Name string
	YAML       []byte
}

// Render produces the environment's namespace.
//
// ORDER IS LOAD-BEARING for callers: the Namespace must be applied before
// anything namespaced, because applying into a namespace that does not exist yet
// is a 404. Callers must apply in slice order.
func Render(s Spec) ([]Manifest, error) {
	if err := ValidateNamespace(s.Namespace); err != nil {
		return nil, err
	}
	// The cell is interpolated as a label VALUE and is subject to the same
	// injection: a newline in it adds a key to the object.
	if err := ValidateCell(s.Cell); err != nil {
		return nil, err
	}

	// THE ENVELOPE. Absent means the control plane did not resolve one, which is
	// a bug there — rendering the namespace without a quota would silently give
	// that environment no ceiling, so it is refused. Partially-set is refused for
	// the same reason: a ResourceQuota missing a dimension bounds nothing on it.
	if !s.Quota.Set() {
		return nil, fmt.Errorf("tenancy: no quota envelope for %s — the control plane resolves "+
			"it from the org's plan; rendering without one leaves the environment unbounded", s.Namespace)
	}
	for dim, v := range map[string]string{
		"cpu": s.Quota.CPU, "memory": s.Quota.Memory, "storage": s.Quota.Storage,
	} {
		if !quantity.MatchString(v) {
			return nil, fmt.Errorf("tenancy: %s quota %q for %s is not a Kubernetes quantity — "+
				"the API server would reject the ResourceQuota at apply", dim, v, s.Namespace)
		}
	}

	// NOTE ON LABELS — there is deliberately no steloit.dev/environment-id here.
	// The agent does not receive the environment id; the desired doc carries the
	// namespace only. US-3.3a labelled it strings.TrimPrefix(namespace, "env-"),
	// which yields "9f3c1a2b" for the id "env_9f3c1a2b" — the label named
	// something that exists nowhere. The namespace NAME already identifies the
	// environment (it is env-<id>); a label restating a lossy re-derivation of
	// the name is not a second source, it is a second chance to be wrong.
	return []Manifest{
		{Kind: "Namespace", Name: s.Namespace, YAML: []byte(fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    steloit.dev/cell: %s
    steloit.dev/tenant: "true"
`, s.Namespace, s.Cell))},

		// THE PER-ENVIRONMENT CEILING.
		//
		// Enforced by the API server's ResourceQuota ADMISSION CONTROLLER, which
		// is in Kubernetes' default-enabled plugin list and needs no add-on. That
		// distinction is the whole reason this ships while D7's NetworkPolicies do
		// not: a NetworkPolicy needs a provider that this cell did not have, so
		// the objects were stored and ignored. A ResourceQuota has no such switch.
		//
		// Scoped to REQUESTS, not limits. A quota on `limits.*` forces every pod
		// to declare a limit, and the only way to supply one for a workload that
		// declares none is a LimitRange `default` — which then becomes that pod's
		// hard cap. The CNPG Cluster template declares no resources (US-3.3d owns
		// deriving them from the sold shape), so a default limit would silently
		// cap every managed Postgres, and one large enough not to would let a
		// single pod consume the whole environment. Requests are also what
		// actually allocate capacity: a pod requesting 1 CPU occupies 1 CPU of
		// schedulable capacity whether or not it uses it, so bounding requests
		// bounds the plan's real allocation.
		{Kind: "ResourceQuota", Name: "env-quota", YAML: []byte(fmt.Sprintf(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: env-quota
  namespace: %s
spec:
  hard:
    requests.cpu: "%s"
    requests.memory: %s
    requests.storage: %s
`, s.Namespace, s.Quota.CPU, s.Quota.Memory, s.Quota.Storage))},

		// THE PAIR. A ResourceQuota constraining requests.cpu/memory REJECTS any
		// pod that declares neither — that is what enforcement being real means
		// here, and shipping the quota alone would make the namespace refuse
		// ordinary pods. LimitRange supplies the missing requests.
		//
		// defaultRequest ONLY, deliberately no `default`: a default LIMIT becomes
		// a hard cap on any container that declares nothing, which is how an
		// earlier revision would have OOMKilled every managed Postgres at 512Mi.
		// A container that declares its own requests is unaffected by these.
		{Kind: "LimitRange", Name: "env-limits", YAML: []byte(fmt.Sprintf(`apiVersion: v1
kind: LimitRange
metadata:
  name: env-limits
  namespace: %s
spec:
  limits:
    - type: Container
      defaultRequest:
        cpu: 100m
        memory: 128Mi
`, s.Namespace))},
	}, nil
}
