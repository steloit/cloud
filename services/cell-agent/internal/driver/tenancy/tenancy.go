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
// WHAT IT ALSO RENDERS, AND WHAT IS STILL WITHHELD — D7 (INF-001 §1) calls for
// "default-deny NetworkPolicies, ResourceQuota, LimitRange". US-3.3a shipped all
// three and the security review found each one inert or harmful; US-3.3e brings
// back two of them, with the reasons they were withdrawn actually addressed:
//
//   - ResourceQuota and LimitRange ARE rendered here now. The blocker was that
//     the envelope was a product decision with no owner; it has one (founder,
//     2026-08-23, docs/founder-config.md §5) and the control plane resolves the
//     org's plan to concrete values and ships them in the desired doc. This
//     package holds NO copy of the table and no default: a doc with no envelope
//     is REFUSED, not rendered bare and not rendered at some fallback. A control
//     plane that forgets to send one gets a loud, repeated failure instead of a
//     ceiling nobody granted, silently applied to a paying customer.
//
//     The cost of that choice is real and is paid deliberately: a service whose
//     doc predates US-3.3e cannot converge AT ALL until it has an envelope, which
//     is why the feature ships with a backfill migration
//     (20260823140000_service_quota_backfill) rather than relying on each service
//     being touched. Refusing is right because the alternative is unbounded, and
//     an unbounded environment looks exactly like a working one.
//
//   - The LimitRange supplies defaultRequest ONLY, never default. That is the
//     whole lesson of the withdrawal: a `default` becomes the hard cap on every
//     container that declares nothing, and cluster.yaml.tmpl declares nothing, so
//     the 512Mi default would have capped every managed Postgres. A quota on
//     requests.* still needs SOME request on every pod or admission rejects it —
//     hence defaultRequest, which fills the request and caps nothing.
//
//   - Because the CNPG Cluster declares no resources of its own, the cpu and
//     memory ceilings currently bind as a POD-COUNT proxy rather than as compute.
//     Storage is the only dimension that truly binds today. US-3.3d makes the
//     Cluster declare its own requests; until it lands, do not read this package
//     as enforcing the compute half of the envelope.
//
//   - NetworkPolicies are STILL withheld. They are not enforced on a GKE Standard
//     cluster with no network_policy and no ADVANCED_DATAPATH — the API server
//     stores every policy and no packet is dropped — and the allow-set as written
//     denies what CNPG requires: the metadata server (Workload Identity), GCS
//     (WAL archiving) and the apiserver (the in-pod instance manager). Enforcement
//     is being turned on separately (task/US-3.3f); the allow-set lands with it,
//     because either alone is a no-op or an outage.
package tenancy

import (
	"bytes"
	"fmt"
	"io"
	"net/netip"
	"regexp"

	"gopkg.in/yaml.v3"
)

// The closed grammar the control plane emits (billing.validQuantity). Re-checked
// HERE because this module interpolates the value into a manifest: the two ends
// of the wire are separate modules, and a value that got past one is not a value
// the other may assume.
//
// CPU is unitless whole cores; memory and storage REQUIRE a unit. That split is
// not cosmetic — an unsuffixed Kubernetes quantity is BYTES, so a memory quota
// of "16" is sixteen bytes and admits nothing. One shared optional-unit pattern
// accepted exactly that.
var (
	cpuQuantity  = regexp.MustCompile(`^[1-9][0-9]*$`)
	byteQuantity = regexp.MustCompile(`^[1-9][0-9]*(Mi|Gi|Ti)$`)
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

	// APIServerCIDR is the cluster's PRIVATE control-plane endpoint, as an
	// ipBlock CIDR — on GKE, `privateClusterConfig.privateEndpoint` (e.g.
	// 10.30.0.2/32).
	//
	// CNPG's instance manager and its bootstrap job both watch the Cluster CR,
	// so they must reach the kube-apiserver. Under default-deny egress a
	// selector-only peer never matches it, because the API server's backing
	// address is not a pod IP.
	//
	// IT IS NOT THE `kubernetes` SERVICE ClusterIP, and that distinction cost a
	// live debugging session. Pods dial 34.118.224.1:443 (the ClusterIP), so
	// naming that as the ipBlock LOOKS obviously right — and does not work:
	// Dataplane V2 evaluates egress against the post-translation destination, so
	// an ipBlock matching a virtual service IP matches nothing. Measured on
	// cell-verify: with the ClusterIP the initdb pod logged
	//   dial tcp 34.118.224.1:443: i/o timeout
	// and the Cluster sat in "Setting up primary" indefinitely; with the private
	// endpoint alone it reached "Cluster in healthy state" in 45s. The public
	// endpoint is not needed.
	//
	// (The same translation rule is why the DNS peer names node-local-dns pods
	// rather than the kube-dns ClusterIP — one mechanism, two symptoms.)
	//
	// REQUIRED for the same reason the quota is: rendering the set without it
	// produces a boundary that fences the first Postgres pod, which is exactly
	// the outage US-3.3a's review predicted.
	APIServerCIDR string
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

// RefuseEndPort rejects any NetworkPolicy carrying a port RANGE.
//
// EXPORTED SO IT CAN BE TESTED AGAINST A POLICY THAT ACTUALLY CARRIES ONE.
// Inline, it was unreachable: Render validates the namespace first, so the only
// input that could smuggle `endPort` into the bytes was already refused, and
// deleting the whole guard left the suite green. A guard whose test cannot reach
// it is not a guard — it is a comment that compiles.
//
// Dataplane V2 silently does not enforce port ranges on affected versions: the
// API server accepts the policy and drops nothing, which is the same defect
// class as shipping NetworkPolicies to a cluster with no provider (ADR-0015).
func RefuseEndPort(ms []Manifest) error {
	for _, m := range ms {
		if m.Kind != "NetworkPolicy" {
			continue
		}
		if bytes.Contains(m.YAML, []byte("endPort")) {
			return fmt.Errorf("tenancy: %s/%s carries endPort — Dataplane V2 does not "+
				"enforce port ranges on affected versions, so this policy would be stored and "+
				"ignored (ADR-0015). Enumerate single ports instead", m.Kind, m.Name)
		}
	}
	return nil
}

// ValidateCIDR refuses anything that is not a plain IPv4 CIDR.
//
// It is interpolated into an ipBlock, so the same injection rule as the
// namespace applies: a newline here adds a key to the policy. It is also the
// one field whose WRONGNESS is silent — a malformed CIDR makes the API server
// reject the whole policy, and a *valid but wrong* one produces a boundary that
// looks right and fences the instance manager.
func ValidateCIDR(c string) error {
	if c == "" {
		return fmt.Errorf("no API server CIDR: CNPG's instance manager must reach the " +
			"kube-apiserver, and under default-deny egress a selector-only peer never matches it")
	}
	pfx, err := netip.ParsePrefix(c)
	if err != nil {
		// A shape check (a regexp over digits and dots) accepted
		// 999.999.999.999/99, which the API server then refuses — so EVERY apply
		// on the cell fails, from a value that looked fine at boot.
		return fmt.Errorf("API server CIDR %q is not a valid CIDR: %w", c, err)
	}
	if !pfx.Addr().Is4() {
		return fmt.Errorf("API server CIDR %q is not IPv4", c)
	}
	// A wide prefix is the dangerous case, not the malformed one: 0.0.0.0/0
	// silently turns the apiserver allowance into unrestricted TCP/443 egress
	// for every CNPG pod — including into the private ranges the sibling rule's
	// `except` list exists to exclude. A control-plane endpoint is a host or a
	// small block; /24 is already generous.
	if pfx.Bits() < 24 {
		return fmt.Errorf("API server CIDR %q is a /%d — a control plane endpoint is a host or a "+
			"small block, and a wide prefix here grants CNPG pods egress far beyond it",
			c, pfx.Bits())
	}
	return nil
}

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
	if err := ValidateCIDR(s.APIServerCIDR); err != nil {
		return nil, fmt.Errorf("tenancy: %s: %w", s.Namespace, err)
	}
	if !s.Quota.Set() {
		return nil, fmt.Errorf("tenancy: no quota envelope for %s — the control plane resolves "+
			"it from the org's plan; rendering without one leaves the environment unbounded", s.Namespace)
	}
	if !cpuQuantity.MatchString(s.Quota.CPU) {
		return nil, fmt.Errorf("tenancy: cpu quota %q for %s is not a whole number of cores — "+
			"the API server would reject the ResourceQuota at apply", s.Quota.CPU, s.Namespace)
	}
	for dim, v := range map[string]string{"memory": s.Quota.Memory, "storage": s.Quota.Storage} {
		if !byteQuantity.MatchString(v) {
			return nil, fmt.Errorf("tenancy: %s quota %q for %s is not a whole Mi/Gi/Ti quantity — "+
				"an unsuffixed value means BYTES, which admits nothing", dim, v, s.Namespace)
		}
	}

	// NOTE ON LABELS — there is deliberately no steloit.dev/environment-id here.
	// The agent does not receive the environment id; the desired doc carries the
	// namespace only. US-3.3a labelled it strings.TrimPrefix(namespace, "env-"),
	// which yields "9f3c1a2b" for the id "env_9f3c1a2b" — the label named
	// something that exists nowhere. The namespace NAME already identifies the
	// environment (it is env-<id>); a label restating a lossy re-derivation of
	// the name is not a second source, it is a second chance to be wrong.
	// endPort IS REFUSED, NEVER WRITTEN (ADR-0015, US-3.3c AC 8). Dataplane V2
	// silently does not enforce port RANGES on affected versions — a policy the
	// API server accepts and does not apply, which is the exact defect class
	// ADR-0015 exists to close. The refusal lives HERE, where policies are
	// produced, so no caller can forget it; it is checked against the rendered
	// bytes at the end of this function rather than trusted to review.

	out := []Manifest{
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
		//
		// requests.storage IS WITHDRAWN, and this is the same withdrawal US-3.3a
		// made of the D7 policies, for the same class of reason: shipping an
		// enforcement whose preconditions do not exist.
		//
		// The founder's storage number is real and is carried all the way here
		// in the desired doc — but the API CANNOT PREDICT WHAT PVC THE CELL WILL
		// CREATE, so it cannot refuse a create that will not fit. Measured on
		// this branch: `estimates.Resolve` returns `storage_gb: 0` for dev,
		// standard AND performance, while `cnpg.storageForShape` renders 10Gi,
		// 32Gi and 128Gi from a size table the DRIVER owns. Rendering the
		// ceiling anyway means the API prices a shape, returns 201, starts
		// billing it, and admission then refuses the PVC — the service sits in
		// `provisioning` forever with no writeback and no alert. A free org
		// ordering a `standard` (32Gi against 10Gi), or a SECOND dev (10Gi+10Gi
		// against 10Gi), both did exactly that.
		//
		// Enforcing it needs the rendered size to be derivable from the catalog
		// both planes read — which is T3.4c (storage_gb actually sizes the PVC)
		// plus a catalog-owned floor for the sizes whose minimum the driver
		// currently owns alone. Duplicating that table in services/api would put
		// a data-plane sizing rule in the control plane, which is the boundary
		// this whole design exists to keep. Filed as US-3.3i.
		{Kind: "ResourceQuota", Name: "env-quota", YAML: []byte(fmt.Sprintf(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: env-quota
  namespace: %s
spec:
  hard:
    requests.cpu: "%s"
    requests.memory: %s
`, s.Namespace, s.Quota.CPU, s.Quota.Memory))},

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

		// ---- D7: THE ENVIRONMENT IS A NETWORK BOUNDARY --------------------
		//
		// These are enforced ONLY because the cell runs Dataplane V2
		// (US-3.3f/ADR-0015). On GKE Standard without it the API server stores
		// every one of these and drops nothing — US-3.3a shipped them into
		// exactly that and the boundary was nominal. The module now sets
		// datapath_provider = ADVANCED_DATAPATH and a terraform test asserts it.

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
`, s.Namespace))},

		// DNS. TWO peers, because TWO different pods can serve it, and each peer
		// keeps namespaceSelector and podSelector in the SAME list element — an
		// AND. As two separate elements it becomes an OR over kube-system, which
		// allows egress to EVERY pod there; US-3.3a's tests could not tell the
		// difference and widening it stayed green.
		//
		// node-local-dns IS REQUIRED, and this was measured live, not reasoned
		// about (US-3.3c). On a stock GKE cell with NodeLocal DNSCache — which is
		// on by default and is NOT pinned by our terraform — `/etc/resolv.conf`
		// still names the kube-dns ClusterIP, so the rule LOOKS right; but the
		// node-local cache answers the query, policy is evaluated against that
		// pod, and a rule naming only `k8s-app: kube-dns` matches nothing.
		// Measured on cell-verify: with only the kube-dns peer, `nslookup
		// kubernetes.default` returned "connection timed out; no servers could be
		// reached"; adding this peer resolved it. AC 5 predicted this failure and
		// predicted hostNetwork as the reason — on this version node-local-dns
		// has ordinary pod IPs, so a podSelector CAN match it, which is why the
		// fix is a peer rather than an ipBlock.
		//
		// Both peers stay: which one serves depends on whether NodeLocal DNSCache
		// is enabled, and a cell without it must still resolve.
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
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
          podSelector:
            matchLabels:
              k8s-app: node-local-dns
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
`, s.Namespace))},

		// Intra-environment traffic. THIS is what makes env A unreachable from
		// env B: a bare `podSelector: {}` peer means "pods in THIS namespace",
		// not "all pods" — the namespace scoping is implicit and is the boundary.
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
`, s.Namespace))},

		// ---- THE FOUR CNPG ALLOWANCES ------------------------------------
		//
		// Without these, default-deny egress fences the first Postgres pod and it
		// never reaches ready. A `to:` peer carrying only pod/namespace selectors
		// never matches a non-pod IP, so each of these needs an ipBlock.
		//
		// SCOPED TO CNPG PODS, and that scoping IS AC 9. Customer code runs in
		// the same namespace under gVisor and must NOT reach the metadata server
		// (GKE Sandbox's own docs name NetworkPolicy as the control for that),
		// while CNPG REQUIRES it for Workload Identity. The asymmetry is
		// structural rather than a second rule: only CNPG-managed pods match, so
		// everything else is still covered by default-deny. Widening this to
		// `podSelector: {}` silently grants customer code the metadata server.
		//
		// `cnpg.io/cluster` EXISTS, not `cnpg.io/podRole: instance`. This is not
		// a fifth ALLOWANCE — the four above are still four — it is a fifth
		// WORKLOAD that none of the four covered, which US-3.3a's review did not
		// name and only a live run found. CNPG bootstraps through a JOB whose pod carries
		// `cnpg.io/jobRole: initdb` and `cnpg.io/cluster`, but NOT
		// `cnpg.io/podRole: instance`: that label appears only once an instance
		// exists. Selecting on podRole therefore matches NOTHING during
		// bootstrap, so the initdb pod is fenced by default-deny and the cluster
		// never starts. Measured on cell-verify: the initdb pod logged
		//   dial tcp 34.118.224.1:443: i/o timeout
		// against the apiserver and the Cluster sat in "Setting up primary" until
		// the job backed off. `cnpg.io/cluster` is present at EVERY stage —
		// bootstrap, join and steady state — and is carried only by
		// operator-managed pods, so it is both sufficient and still narrow.
		{Kind: "NetworkPolicy", Name: "allow-cnpg-egress", YAML: []byte(fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-cnpg-egress
  namespace: %s
spec:
  podSelector:
    matchExpressions:
      - key: cnpg.io/cluster
        operator: Exists
  policyTypes:
    - Egress
  egress:
    - to:
        - ipBlock:
            cidr: 169.254.169.254/32
      ports:
        - protocol: TCP
          port: 80
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
              - 10.0.0.0/8
              - 172.16.0.0/12
              - 192.168.0.0/16
              - 169.254.0.0/16
      ports:
        - protocol: TCP
          port: 443
    - to:
        - ipBlock:
            cidr: %s
      ports:
        - protocol: TCP
          port: 443
`, s.Namespace, s.APIServerCIDR))},

		// The operator reaches the instance manager on 8000 (status, lifecycle).
		// Without this the Cluster is created and never becomes ready.
		{Kind: "NetworkPolicy", Name: "allow-cnpg-operator-ingress", YAML: []byte(fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-cnpg-operator-ingress
  namespace: %s
spec:
  podSelector:
    matchExpressions:
      - key: cnpg.io/cluster
        operator: Exists
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: cnpg-system
      ports:
        - protocol: TCP
          port: 8000
`, s.Namespace))},
	}

	// endPort IS REFUSED ON THE RENDERED BYTES, not on the inputs.
	//
	// ADR-0015 states in the present tense that "US-3.3c carries an AC that
	// tenancy.Render must refuse a policy carrying endPort". Checking the bytes
	// rather than a parameter is what makes that true of anything this function
	// can ever emit: a future policy that gains a port RANGE is refused by the
	// same line, with no new place to remember.
	//
	// Dataplane V2 silently does not enforce port ranges on affected versions —
	// the API server accepts the policy and drops nothing, which is the same
	// class of defect as shipping NetworkPolicies to a cluster with no provider.
	if err := RefuseEndPort(out); err != nil {
		return nil, err
	}
	return out, nil
}

// TeardownObjects is what removing an environment must delete EXPLICITLY.
//
// Deleting a Namespace deletes everything inside it, so the namespaced objects
// Render produces — the ResourceQuota, the LimitRange, and whatever US-3.3c adds
// beside them — need no delete of their own. Only the CLUSTER-SCOPED ones do,
// and today that is the Namespace itself.
//
// SCOPE IS READ FROM THE RENDERED BYTES, not from a list. An object that
// declares `metadata.namespace` is namespaced and dies with the namespace;
// one that does not is cluster-scoped and must be deleted by name. That is the
// same fact kube.Apply enforces from the other side (it refuses a cluster-scoped
// kind that declares a namespace), so the two cannot disagree — and there is no
// second table to forget to update when US-3.3c adds an object.
//
// The quota is irrelevant to a teardown (nothing is being enforced, only
// removed) so it takes a placeholder that satisfies Render's validation. That is
// the one seam: if Render ever varies its OBJECT SET by quota rather than only
// its contents, this stops being complete, which is what
// TestTeardownCoversEveryObjectTenancyRenders exists to catch.
func TeardownObjects(namespace string) ([]Manifest, error) {
	all, err := Render(Spec{
		Namespace:     namespace,
		Cell:          "teardown",
		Quota:         Quota{CPU: "1", Memory: "1Gi", Storage: "1Gi"},
		APIServerCIDR: "10.0.0.0/28", // placeholder: teardown deletes, it enforces nothing
	})
	if err != nil {
		return nil, fmt.Errorf("tenancy: enumerate teardown objects for %q: %w", namespace, err)
	}
	out := make([]Manifest, 0, 1)
	for _, m := range all {
		scoped, err := declaresNamespace(m.YAML)
		if err != nil {
			return nil, fmt.Errorf("tenancy: %s/%s: %w", m.Kind, m.Name, err)
		}
		if !scoped {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("tenancy: teardown for %q would delete NOTHING — Render produced "+
			"no cluster-scoped object, so the namespace would be left behind", namespace)
	}
	return out, nil
}

// declaresNamespace reports whether a rendered object sets metadata.namespace.
//
// It REFUSES a multi-document stream rather than answering for the first one.
// yaml.Unmarshal decodes only the first document and returns a nil error, so a
// stream whose SECOND object is cluster-scoped would be classified from the
// first and silently never torn down. kube.applyOne refuses multi-doc for the
// same reason (exactlyOneDocument): one object per []byte, or the metadata we
// read does not describe all of the bytes.
func declaresNamespace(manifest []byte) (bool, error) {
	type object struct {
		Metadata struct {
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(manifest))
	var docs []object
	for {
		var o object
		err := dec.Decode(&o)
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("parse rendered manifest: %w", err)
		}
		docs = append(docs, o)
	}
	switch len(docs) {
	case 0:
		return false, fmt.Errorf("rendered manifest carries no object")
	case 1:
		return docs[0].Metadata.Namespace != "", nil
	default:
		return false, fmt.Errorf("rendered manifest carries %d documents; scope is per-object "+
			"and reading only the first would silently mis-scope the rest", len(docs))
	}
}
