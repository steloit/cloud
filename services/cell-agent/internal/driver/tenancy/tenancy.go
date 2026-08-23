// Package tenancy renders an environment's namespace and the D7 isolation
// boundary that makes it a tenant boundary rather than just a name.
//
// WHY THE AGENT AND NOT THE CONTROL PLANE — US-3.3a asked for this decision to
// be made and recorded. The control plane has NO Kubernetes dependency:
// services/api/go.mod contains no k8s.io/* at all, because the two-plane split
// (D6, frozen by ADR-0001) is that the control plane writes desired state and
// the cell-agent converges it. Creating the namespace control-plane-side would
// mean giving the control plane cluster credentials and a kube client — an
// architecture delta, not an implementation choice, and ADR-0040 says a delta
// needs evidence from implementation or a customer, which this does not have.
//
// Agent-side is also level-triggered for free: a namespace deleted out from
// under us is recreated on the next converge, where a create-once call at
// environment creation would never notice.
//
// D7 (INF-001 §1): "Environment → Kubernetes namespace (env-<environment_id>)
// with default-deny NetworkPolicies, ResourceQuota, LimitRange." All of them are
// rendered together, because a namespace without them is the isolation boundary
// in name only — which is exactly the state US-3.3 left behind.
package tenancy

import (
	"fmt"
	"strings"
)

// Spec is what an environment needs in order to become a tenant boundary.
type Spec struct {
	Namespace string // env-<environment_id> (ADR-0012)
	Cell      string
	EnvID     string
}

// Manifest is one rendered object.
type Manifest struct {
	Kind, Name string
	YAML       []byte
}

// Render produces the namespace and its D7 policies.
//
// ORDER IS LOAD-BEARING: the Namespace is first, because everything after it is
// namespaced and applying into a namespace that does not exist yet is a 404.
// Callers must apply in slice order.
func Render(s Spec) ([]Manifest, error) {
	if s.Namespace == "" || s.EnvID == "" {
		return nil, fmt.Errorf("tenancy: namespace and env id are required")
	}
	if s.Cell == "" {
		return nil, fmt.Errorf("tenancy: cell is required — every row carries cell_id (D7)")
	}
	// The namespace name is the control plane's to choose (ADR-0012,
	// env-<environment_id>). Refuse anything else rather than deriving a second
	// opinion here — two derivations is how they drift.
	if !strings.HasPrefix(s.Namespace, "env-") {
		return nil, fmt.Errorf("tenancy: namespace %q is not env-<environment_id> (ADR-0012)", s.Namespace)
	}

	ns := s.Namespace
	return []Manifest{
		{Kind: "Namespace", Name: ns, YAML: []byte(fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    steloit.dev/environment-id: %s
    steloit.dev/cell: %s
    steloit.dev/tenant: "true"
`, ns, s.EnvID, s.Cell))},

		// DEFAULT-DENY, BOTH DIRECTIONS, stated rather than implied.
		//
		// An empty podSelector selects every pod in the namespace, and naming a
		// policyType with no matching rules denies all traffic of that type.
		// Ingress and Egress are SEPARATE denials: a policy with only Ingress
		// leaves egress wide open, and egress is the half that lets a compromised
		// pod reach the metadata server and other tenants.
		{Kind: "NetworkPolicy", Name: "default-deny-all", YAML: []byte(fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: %s
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
`, ns))},

		// DNS is the one egress exception, scoped to kube-dns rather than opened
		// to the cluster. Without it every pod fails to resolve anything,
		// including its own database service — a default-deny that breaks the
		// product does not ship, it gets reverted.
		{Kind: "NetworkPolicy", Name: "allow-dns-egress", YAML: []byte(fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-dns-egress
  namespace: %s
spec:
  podSelector: {}
  policyTypes:
    - Egress
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
`, ns))},

		// Same-namespace traffic. A service must reach its own database, and the
		// default-deny above forbids even that until this allows it. Scoped to
		// the namespace, so it is not a hole in the tenant boundary.
		{Kind: "NetworkPolicy", Name: "allow-same-namespace", YAML: []byte(fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-same-namespace
  namespace: %s
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector: {}
  egress:
    - to:
        - podSelector: {}
`, ns))},

		{Kind: "ResourceQuota", Name: "env-quota", YAML: []byte(fmt.Sprintf(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: env-quota
  namespace: %s
spec:
  hard:
    requests.cpu: "8"
    requests.memory: 16Gi
    limits.cpu: "16"
    limits.memory: 32Gi
    persistentvolumeclaims: "16"
    count/services: "32"
`, ns))},

		// A ResourceQuota constraining requests/limits REJECTS any pod that omits
		// them. LimitRange supplies the defaults, so the quota is enforceable
		// without every workload spelling them out. The two are a PAIR — shipping
		// the quota alone makes the namespace refuse ordinary pods.
		{Kind: "LimitRange", Name: "env-limits", YAML: []byte(fmt.Sprintf(`apiVersion: v1
kind: LimitRange
metadata:
  name: env-limits
  namespace: %s
spec:
  limits:
    - type: Container
      default:
        cpu: 500m
        memory: 512Mi
      defaultRequest:
        cpu: 100m
        memory: 128Mi
`, ns))},
	}, nil
}
