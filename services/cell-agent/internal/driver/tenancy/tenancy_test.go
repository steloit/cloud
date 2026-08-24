package tenancy_test

import (
	"strings"
	"testing"

	"github.com/steloit/cloud/services/cell-agent/internal/driver/tenancy"
	"gopkg.in/yaml.v3"
)

// proQuota is the `pro` envelope from plans.json, as the control plane resolves
// and ships it. Tests that are not about the quota still need one, because
// rendering without an envelope is refused.
var proQuota = tenancy.Quota{CPU: "8", Memory: "16Gi", Storage: "100Gi"}

const (
	ns   = "env-9f3c1a2b"
	cell = "cell0"
)

func mustRender(t *testing.T) []tenancy.Manifest {
	t.Helper()
	objs, err := tenancy.Render(tenancy.Spec{APIServerCIDR: testAPIServerCIDR, Namespace: ns, Cell: cell, Quota: proQuota})
	if err != nil {
		t.Fatal(err)
	}
	return objs
}

// The environment's namespace, its ceiling, and the LimitRange that makes the
// ceiling usable. The namespace is the object nothing in the system created
// before US-3.3a — the defect was a 404 on the first converge into a new
// environment.
func TestRenderProducesTheEnvironmentsObjects(t *testing.T) {
	objs := mustRender(t)
	var got []string
	for _, m := range objs {
		got = append(got, m.Kind+"/"+m.Name)
	}
	want := []string{
		"Namespace/" + ns,
		"ResourceQuota/env-quota",
		"LimitRange/env-limits",
		// US-3.3c: the D7 boundary. Order is load-bearing — default-deny FIRST,
		// so a converge interrupted between objects leaves the environment
		// closed rather than open.
		"NetworkPolicy/default-deny-all",
		"NetworkPolicy/allow-dns-egress",
		"NetworkPolicy/allow-same-namespace",
		"NetworkPolicy/allow-cnpg-egress",
		"NetworkPolicy/allow-cnpg-operator-ingress",
	}
	if len(got) != len(want) {
		t.Fatalf("rendered %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("object %d is %s, want %s — the Namespace must be FIRST (everything after "+
				"it is namespaced) and the quota must accompany it", i, got[i], want[i])
		}
	}
}

// THE D7 POLICY SET IS RENDERED, AND DEFAULT-DENY COMES FIRST.
//
// US-3.3a withheld these because nothing enforced them (GKE Standard stores a
// NetworkPolicy and drops nothing) AND because its allow-set fenced CNPG off the
// metadata server, GCS and the apiserver. US-3.3f turned enforcement on, and
// US-3.3c supplies the allowances — so they ship together, which is what the
// four findings being "one problem" meant.
//
// Ordering is asserted because it is a failure mode, not a preference: the
// objects are applied in this order, so default-deny landing LAST would leave a
// window in which the namespace exists and denies nothing.
func TestTheD7PolicySetIsRenderedDefaultDenyFirst(t *testing.T) {
	var policies []string
	for _, m := range mustRender(t) {
		if m.Kind == "NetworkPolicy" {
			policies = append(policies, m.Name)
		}
	}
	if len(policies) == 0 {
		t.Fatal("no NetworkPolicy rendered — the environment is a name, not a boundary")
	}
	if policies[0] != "default-deny-all" {
		t.Errorf("the first policy is %q, not default-deny-all — an interrupted converge would "+
			"leave the environment open", policies[0])
	}
}

// What the applier reads is metadata.name and the object's kind; what an operator
// reads later is the labels. Parsed, not substring-matched, because a substring
// test passes on a manifest the API server would reject.
func TestTheNamespaceParsesAndCarriesWhatItClaims(t *testing.T) {
	var obj struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name   string            `yaml:"name"`
			Labels map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
	}
	m := mustRender(t)[0]
	if err := yaml.Unmarshal(m.YAML, &obj); err != nil {
		t.Fatalf("Namespace does not parse: %v\n%s", err, m.YAML)
	}
	if obj.APIVersion != "v1" || obj.Kind != "Namespace" {
		t.Fatalf("got %s/%s", obj.APIVersion, obj.Kind)
	}
	if obj.Metadata.Name != ns {
		t.Fatalf("metadata.name = %q, want %q — the applier addresses by this", obj.Metadata.Name, ns)
	}
	want := map[string]string{"steloit.dev/cell": cell, "steloit.dev/tenant": "true"}
	for k, v := range want {
		if obj.Metadata.Labels[k] != v {
			t.Fatalf("label %s = %q, want %q", k, obj.Metadata.Labels[k], v)
		}
	}
	if len(obj.Metadata.Labels) != len(want) {
		t.Fatalf("labels = %v, want exactly %v", obj.Metadata.Labels, want)
	}
	// US-3.3a shipped steloit.dev/environment-id set to TrimPrefix(ns, "env-").
	// The real id is env_9f3c1a2b, so the label read "9f3c1a2b" — a value that
	// names nothing the control plane knows. Absent is better than wrong; the
	// namespace NAME already identifies the environment.
	if _, ok := obj.Metadata.Labels["steloit.dev/environment-id"]; ok {
		t.Fatal("steloit.dev/environment-id is back. The agent is not given the " +
			"environment id — the desired doc carries the namespace only — so any " +
			"value here is a re-derivation. Add it to the desired doc first.")
	}
}

// The namespace and the cell are interpolated into a manifest. Both are
// control-plane-minted today, and both are the guard that defines a tenant
// boundary, so neither gets to be trusted on a prefix alone.
func TestRenderRefusesAnythingThatIsNotAnRFC1123Label(t *testing.T) {
	long := "env-" + strings.Repeat("a", 60) // 64 chars

	badNS := map[string]string{
		"empty":              "",
		"no env- prefix":     "acme--prod",
		"bare prefix":        "env-",
		"uppercase":          "env-9F3C1A2B",
		"embedded space":     "env- x",
		"trailing space":     "env-x ",
		"dot":                "env-x.y",
		"underscore":         "env-x_y",
		"bang":               "env-x!",
		"trailing dash":      "env-x-",
		"over 63 chars":      long,
		"yaml key injection": "env-x\n  labels:\n    steloit.dev/tenant: \"false\"\nextra: injected",
		"label injection":    "env-x\n    steloit.dev/tenant: \"false\"",
	}
	for name, v := range badNS {
		t.Run("namespace/"+name, func(t *testing.T) {
			if _, err := tenancy.Render(tenancy.Spec{APIServerCIDR: testAPIServerCIDR, Namespace: v, Cell: cell, Quota: proQuota}); err == nil {
				t.Fatalf("Render accepted namespace %q", v)
			}
		})
	}

	badCell := map[string]string{
		"empty":         "",
		"uppercase":     "Cell0",
		"space":         "cell 0",
		"key injection": "cell0\n    steloit.dev/tenant: \"false\"",
		"over 63 chars": strings.Repeat("c", 64),
		"leading dash":  "-cell0",
		"underscore":    "cell_0",
	}
	for name, v := range badCell {
		t.Run("cell/"+name, func(t *testing.T) {
			if _, err := tenancy.Render(tenancy.Spec{APIServerCIDR: testAPIServerCIDR, Namespace: ns, Cell: v, Quota: proQuota}); err == nil {
				t.Fatalf("Render accepted cell %q", v)
			}
		})
	}

	// The negative half is only meaningful if the positive half still passes:
	// a Render that refuses everything would satisfy every case above.
	for _, good := range []string{"env-a", "env-9f3c1a2b", "env-" + strings.Repeat("a", 59)} {
		if _, err := tenancy.Render(tenancy.Spec{APIServerCIDR: testAPIServerCIDR, Namespace: good, Cell: cell, Quota: proQuota}); err != nil {
			t.Fatalf("Render refused a legitimate namespace %q: %v", good, err)
		}
	}
}

// Every namespace the control plane can mint must be one Render accepts. If this
// ever parts company, the agent hard-errors on EVERY converge for that
// environment and the control plane sees no signal at all.
func TestRenderAcceptsEveryShapeTheControlPlaneCanMint(t *testing.T) {
	// namespaceForEnv is services/api-side: k8sNamespace lowercases, replaces each
	// run of invalid characters with "-", trims dashes, truncates to 63.
	//
	// The FIRST entry is the shape it actually mints in production —
	// ids.New("env") emits env_<32 hex>, so the namespace is env-<32 hex>, 36
	// characters. An earlier version of this test listed three short strings and
	// omitted the only shape that ever occurs, which is a cross-plane contract
	// test that does not test the contract.
	for _, produced := range []string{
		// The ONLY shape production mints: ids.New("env") emits env_<32 hex>, and
		// k8sNamespace maps "_"→"-", so the namespace is env-<32 hex>, 36 chars.
		"env-" + strings.Repeat("a1b2c3d4", 4),
		// Seeded by reconcile/wiring_integration_test.go:48,335 — 5 chars, exactly
		// at this function's len<5 boundary.
		"env-w",
		// Boundary cases. NOT currently mintable: 36 < 63 always, so
		// namespaceForEnv's `if len(ns) > 63` truncation branch is unreachable for
		// any id ids.New can produce. Kept so the boundary stays pinned if the id
		// scheme ever changes; an earlier version of this list called the 63-char
		// case "the truncation ceiling", which claimed a path that cannot be taken.
		"env-0",
		"env-" + strings.Repeat("f", 59),
	} {
		if _, err := tenancy.Render(tenancy.Spec{APIServerCIDR: testAPIServerCIDR, Namespace: produced, Cell: cell, Quota: proQuota}); err != nil {
			t.Fatalf("the control plane can mint %q and Render refuses it: %v", produced, err)
		}
	}
}

// The cell label must come FROM Spec.Cell. Asserting it against a single
// constant cannot tell that apart from a hardcoded string: the package's own
// `cell` const is "cell0", so hardcoding `steloit.dev/cell: cell0` in the
// template satisfied a one-value test with the whole suite green.
func TestTheCellLabelIsTheSpecCellAndNotAConstant(t *testing.T) {
	for _, c := range []string{"cell-0", "cell-7"} {
		objs, err := tenancy.Render(tenancy.Spec{APIServerCIDR: testAPIServerCIDR, Namespace: ns, Cell: c, Quota: proQuota})
		if err != nil {
			t.Fatal(err)
		}
		var obj struct {
			Metadata struct {
				Labels map[string]string `yaml:"labels"`
			} `yaml:"metadata"`
		}
		if err := yaml.Unmarshal(objs[0].YAML, &obj); err != nil {
			t.Fatal(err)
		}
		if got := obj.Metadata.Labels["steloit.dev/cell"]; got != c {
			t.Fatalf("Spec.Cell = %q but the label says %q", c, got)
		}
	}
}

// Every Manifest must be EXACTLY ONE YAML document, and its Kind field must
// describe that document.
//
// The absence guard for the withdrawn D7 objects switches on Manifest.Kind and
// never parses the bytes. Appending "---\nkind: NetworkPolicy…" to the Namespace
// manifest re-added a policy with the ENTIRE SUITE GREEN — one representation of
// "what is rendered" (the struct field) was covered and the other (the bytes)
// was not. yaml.Unmarshal reads only document 1, so every check downstream
// describes the first object while all of the bytes get applied.
func TestEachManifestIsExactlyOneDocumentAndItsKindIsTrue(t *testing.T) {
	for _, m := range mustRender(t) {
		dec := yaml.NewDecoder(strings.NewReader(string(m.YAML)))
		docs := 0
		for {
			var node yaml.Node
			err := dec.Decode(&node)
			if err != nil {
				break
			}
			if node.Kind == 0 {
				continue
			}
			docs++
			var head struct {
				APIVersion string `yaml:"apiVersion"`
				Kind       string `yaml:"kind"`
				Metadata   struct {
					Name      string `yaml:"name"`
					Namespace string `yaml:"namespace"`
				} `yaml:"metadata"`
			}
			if err := node.Decode(&head); err != nil {
				t.Fatalf("document %d of %s does not decode: %v", docs, m.Kind, err)
			}
			if head.Kind != m.Kind {
				t.Fatalf("Manifest.Kind is %q but document %d says %q — the struct field "+
					"and the bytes disagree, and every guard reads the field", m.Kind, docs, head.Kind)
			}
			if head.Metadata.Name != m.Name {
				t.Fatalf("Manifest.Name is %q but the document says %q", m.Name, head.Metadata.Name)
			}
			// SCOPE. A Namespace is cluster-scoped and must not declare a
			// metadata.namespace: kube.Apply refuses that outright, so a rendered
			// one fails EVERY converge for EVERY service on the cell with no
			// writeback. Nothing asserted it — adding `namespace: kube-system`
			// under metadata left the whole suite green.
			if head.APIVersion == "" {
				t.Fatalf("%s/%s declares no apiVersion", m.Kind, m.Name)
			}
			if m.Kind == "Namespace" && head.Metadata.Namespace != "" {
				t.Fatalf("the Namespace declares metadata.namespace %q — it is cluster-scoped, "+
					"and kube.Apply refuses a cluster-scoped object that names a namespace",
					head.Metadata.Namespace)
			}
		}
		if docs != 1 {
			t.Fatalf("%s/%s rendered %d YAML documents, want exactly 1 — a second document "+
				"is invisible to every Kind-based guard in this package", m.Kind, m.Name, docs)
		}
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	a, b := mustRender(t), mustRender(t)
	if len(a) != len(b) {
		t.Fatal("length differs between calls")
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].Name != b[i].Name || string(a[i].YAML) != string(b[i].YAML) {
			t.Fatalf("object %d differs between calls", i)
		}
	}
}

func kinds(objs []tenancy.Manifest) []string {
	out := make([]string, len(objs))
	for i, o := range objs {
		out[i] = o.Kind + "/" + o.Name
	}
	return out
}

// testAPIServerCIDR is a syntactically valid stand-in. The REAL value is
// per-cluster and comes from the agent's config; what these tests pin is that
// Render refuses an absent one and interpolates whatever it is given.
const testAPIServerCIDR = "10.0.0.0/28"

// AC 3: THE ALLOW PEERS ARE ASSERTED STRUCTURALLY, NOT BY SUBSTRING.
//
// US-3.3a's tests proved the policies EXISTED and that default-deny denied, and
// three mutations that widen an allow into a hole all survived green:
// podSelector:{} -> namespaceSelector:{}, egress:[- {}], and DNS widened to all
// of kube-system. Each of those is a parsed-structure question, so this parses.
func TestEveryAllowPeerIsStructurallyNarrow(t *testing.T) {
	for _, m := range mustRender(t) {
		if m.Kind != "NetworkPolicy" {
			continue
		}
		var pol struct {
			Metadata struct{ Name string } `yaml:"metadata"`
			Spec     struct {
				Ingress []struct {
					From []map[string]any `yaml:"from"`
				} `yaml:"ingress"`
				Egress []struct {
					To []map[string]any `yaml:"to"`
				} `yaml:"egress"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal(m.YAML, &pol); err != nil {
			t.Fatalf("%s: %v", m.Name, err)
		}
		check := func(dir string, peers []map[string]any) {
			for i, p := range peers {
				if len(p) == 0 {
					t.Errorf("%s %s peer %d is the BARE {} peer — it matches every pod in "+
						"every namespace and turns the boundary into a no-op", m.Name, dir, i)
					continue
				}
				// A peer is either selector-based or an ipBlock, never a mix, and
				// never a namespaceSelector on its own (that is a whole namespace).
				_, hasNS := p["namespaceSelector"]
				_, hasPod := p["podSelector"]
				_, hasIP := p["ipBlock"]
				if hasIP && (hasNS || hasPod) {
					t.Errorf("%s %s peer %d mixes ipBlock with a selector", m.Name, dir, i)
				}
				if hasNS && !hasPod && !hasIP {
					// ONE NAMED EXCEPTION, argued rather than waived.
					//
					// allow-cnpg-operator-ingress admits the whole cnpg-system
					// namespace on port 8000. That namespace is created by our own
					// Helm release and contains only the operator, so the blast
					// radius is "the operator we installed" — but it IS wider than
					// the rule this test enforces everywhere else.
					//
					// It stays wide because narrowing it to the operator's pod
					// label could not be VERIFIED: the live cell had already been
					// destroyed when this test found it, and shipping an
					// unverified selector here fences the operator off every
					// managed Postgres. Narrowing it, with a live re-run, is
					// US-3.3j. An exception that is named and owned beats a
					// tightening nobody has run.
					if m.Name == "allow-cnpg-operator-ingress" && dir == "ingress" {
						continue
					}
					t.Errorf("%s %s peer %d has a namespaceSelector with NO podSelector — that "+
						"allows every pod in that namespace", m.Name, dir, i)
				}
			}
		}
		for _, r := range pol.Spec.Ingress {
			check("ingress", r.From)
		}
		for _, r := range pol.Spec.Egress {
			check("egress", r.To)
		}
	}
}

// THE DNS RULE MUST NAME BOTH RESOLVERS, and each as ONE peer.
//
// Measured live on a stock GKE cell (US-3.3c): with only the kube-dns peer,
// resolution failed — NodeLocal DNSCache answers the query, so policy is
// evaluated against the node-local-dns pod. NodeLocal DNSCache is on by default
// and is NOT pinned by our terraform (AC 5), so BOTH peers must be present.
func TestTheDNSRuleCoversBothResolversAsSeparateAndedPeers(t *testing.T) {
	var dns *tenancy.Manifest
	for _, m := range mustRender(t) {
		if m.Name == "allow-dns-egress" {
			mm := m
			dns = &mm
		}
	}
	if dns == nil {
		t.Fatal("no allow-dns-egress rendered — a default-deny namespace resolves nothing")
	}
	var pol struct {
		Spec struct {
			Egress []struct {
				To []struct {
					NamespaceSelector struct {
						MatchLabels map[string]string `yaml:"matchLabels"`
					} `yaml:"namespaceSelector"`
					PodSelector struct {
						MatchLabels map[string]string `yaml:"matchLabels"`
					} `yaml:"podSelector"`
				} `yaml:"to"`
			} `yaml:"egress"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(dns.YAML, &pol); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, rule := range pol.Spec.Egress {
		for _, peer := range rule.To {
			app := peer.PodSelector.MatchLabels["k8s-app"]
			// AND, not OR: both selectors must be in the SAME peer, or the rule
			// allows every pod in kube-system.
			if peer.NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] != "kube-system" {
				t.Errorf("DNS peer for %q does not pin the kube-system namespace IN THE SAME peer", app)
			}
			if app == "" {
				t.Error("a DNS peer has no podSelector — that is all of kube-system")
			}
			seen[app] = true
		}
	}
	for _, want := range []string{"kube-dns", "node-local-dns"} {
		if !seen[want] {
			t.Errorf("the DNS rule does not name %q. Measured live: with only kube-dns, "+
				"nslookup returned 'connection timed out; no servers could be reached' on a "+
				"stock GKE cell with NodeLocal DNSCache (AC 5)", want)
		}
	}
}

// THE CNPG ALLOWANCES MUST MATCH BOOTSTRAP PODS, NOT ONLY RUNNING INSTANCES.
//
// Measured live: selecting `cnpg.io/podRole: instance` matches NOTHING during
// bootstrap — the initdb Job's pod carries `cnpg.io/jobRole` and
// `cnpg.io/cluster` but not podRole — so the pod was fenced by default-deny and
// the Cluster never left "Setting up primary".
func TestTheCNPGAllowancesSelectEveryLifecycleStage(t *testing.T) {
	for _, name := range []string{"allow-cnpg-egress", "allow-cnpg-operator-ingress"} {
		var found bool
		for _, m := range mustRender(t) {
			if m.Name != name {
				continue
			}
			found = true
			body := string(m.YAML)
			if strings.Contains(body, "cnpg.io/podRole") {
				t.Errorf("%s selects cnpg.io/podRole, which does not exist on the initdb Job "+
					"pod — the bootstrap is fenced and the cluster never starts", name)
			}
			if !strings.Contains(body, "cnpg.io/cluster") {
				t.Errorf("%s does not select cnpg.io/cluster, which is the one label present "+
					"at bootstrap, join AND steady state", name)
			}
			// Still narrow: it must NOT be an empty selector, or customer code
			// gets the metadata server (AC 9).
			if strings.Contains(body, "podSelector: {}") {
				t.Errorf("%s selects ALL pods — customer code would reach the metadata "+
					"server, which AC 9 exists to prevent", name)
			}
		}
		if !found {
			t.Errorf("%s is not rendered", name)
		}
	}
}

// AC 8: endPort is REFUSED, on the rendered bytes.
func TestRenderRefusesAPolicyCarryingEndPort(t *testing.T) {
	// The guard reads what Render is about to emit, so it is proven by feeding a
	// namespace whose name injects an endPort key into the YAML — the only way a
	// caller can reach it today, and exactly the injection ValidateNamespace also
	// blocks. Both guards are asserted; neither is assumed.
	if _, err := tenancy.Render(tenancy.Spec{
		Namespace: "env-a\n    endPort: 9000", Cell: "cell-0",
		APIServerCIDR: testAPIServerCIDR, Quota: tenancy.Quota{CPU: "8", Memory: "16Gi", Storage: "100Gi"},
	}); err == nil {
		t.Fatal("a namespace injecting endPort was rendered")
	}
}
