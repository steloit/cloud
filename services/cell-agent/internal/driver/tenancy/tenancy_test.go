package tenancy

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func render(t *testing.T) []Manifest {
	t.Helper()
	ms, err := Render(Spec{Namespace: "env-e_123", Cell: "cell-0", EnvID: "e_123"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return ms
}

// D7 names four things. Rendering three of them is not the isolation boundary,
// so the set is asserted by KIND rather than by count — a count passes when one
// kind is rendered twice and another not at all.
func TestRenderProducesEveryD7Object(t *testing.T) {
	got := map[string]int{}
	for _, m := range render(t) {
		got[m.Kind]++
	}
	for kind, want := range map[string]int{
		"Namespace":     1,
		"NetworkPolicy": 3, // default-deny + the DNS and same-namespace exceptions
		"ResourceQuota": 1,
		"LimitRange":    1,
	} {
		if got[kind] != want {
			t.Errorf("%s rendered %d times, want %d — D7 requires the namespace, "+
				"default-deny NetworkPolicies, a ResourceQuota AND a LimitRange", kind, got[kind], want)
		}
	}
}

// The Namespace must come FIRST. Everything after it is namespaced, so applying
// out of order is a 404 on a live cluster — the class of defect that is
// expensive to find there and free to prevent here.
func TestNamespaceIsRenderedFirst(t *testing.T) {
	ms := render(t)
	if ms[0].Kind != "Namespace" {
		t.Fatalf("first manifest is %s, want Namespace — every later object is "+
			"namespaced and would 404 into a namespace that does not exist yet", ms[0].Kind)
	}
}

// Every manifest must be valid YAML with the fields the applier reads
// (apiVersion, kind, metadata.name), because kube.Client.Apply parses exactly
// those and a malformed one fails at apply time, on a cluster.
func TestEveryManifestParsesAndCarriesWhatTheApplierReads(t *testing.T) {
	for _, m := range render(t) {
		var obj struct {
			APIVersion string `yaml:"apiVersion"`
			Kind       string `yaml:"kind"`
			Metadata   struct {
				Name      string `yaml:"name"`
				Namespace string `yaml:"namespace"`
			} `yaml:"metadata"`
		}
		if err := yaml.Unmarshal(m.YAML, &obj); err != nil {
			t.Errorf("%s/%s is not valid YAML: %v", m.Kind, m.Name, err)
			continue
		}
		if obj.APIVersion == "" || obj.Kind != m.Kind || obj.Metadata.Name != m.Name {
			t.Errorf("%s/%s: manifest disagrees with its own descriptor (apiVersion=%q kind=%q name=%q)",
				m.Kind, m.Name, obj.APIVersion, obj.Kind, obj.Metadata.Name)
		}
		// A Namespace is cluster-scoped and must NOT carry metadata.namespace;
		// everything else must.
		if m.Kind == "Namespace" && obj.Metadata.Namespace != "" {
			t.Errorf("the Namespace carries metadata.namespace=%q — it is cluster-scoped", obj.Metadata.Namespace)
		}
		if m.Kind != "Namespace" && obj.Metadata.Namespace == "" {
			t.Errorf("%s/%s has no metadata.namespace — it would apply to the wrong place", m.Kind, m.Name)
		}
	}
}

// The default-deny must deny BOTH directions. A policy with only Ingress leaves
// egress wide open, and egress is the half that reaches the metadata server and
// other tenants. Asserted as two separate facts because they are two.
func TestDefaultDenyCoversIngressAndEgress(t *testing.T) {
	var spec struct {
		Spec struct {
			PodSelector map[string]any `yaml:"podSelector"`
			PolicyTypes []string       `yaml:"policyTypes"`
			Ingress     []any          `yaml:"ingress"`
			Egress      []any          `yaml:"egress"`
		} `yaml:"spec"`
	}
	found := false
	for _, m := range render(t) {
		if m.Name != "default-deny-all" {
			continue
		}
		found = true
		if err := yaml.Unmarshal(m.YAML, &spec); err != nil {
			t.Fatal(err)
		}
	}
	if !found {
		t.Fatal("no default-deny-all policy was rendered")
	}
	types := strings.Join(spec.Spec.PolicyTypes, ",")
	if !strings.Contains(types, "Ingress") {
		t.Error("default-deny does not name Ingress — inbound traffic is unrestricted")
	}
	if !strings.Contains(types, "Egress") {
		t.Error("default-deny does not name Egress — a compromised pod can reach the metadata server and other tenants")
	}
	// An empty podSelector selects EVERY pod. A non-empty one would silently
	// exempt everything that does not match it.
	if len(spec.Spec.PodSelector) != 0 {
		t.Errorf("default-deny podSelector is %v, want {} — anything else exempts the pods it does not match", spec.Spec.PodSelector)
	}
	// Naming a policyType with NO rules is what denies it. A rule list here
	// would turn the denial into an allowance.
	if len(spec.Spec.Ingress) != 0 || len(spec.Spec.Egress) != 0 {
		t.Error("default-deny carries rules — a policyType with rules ALLOWS that traffic; the denial requires none")
	}
}

// The quota and the LimitRange are a pair: a quota constraining requests/limits
// rejects any pod that omits them, so without defaults the namespace refuses
// ordinary workloads.
func TestQuotaIsShippedWithDefaultsThatMakeItUsable(t *testing.T) {
	var quota, limits bool
	var lr struct {
		Spec struct {
			Limits []struct {
				Default        map[string]string `yaml:"default"`
				DefaultRequest map[string]string `yaml:"defaultRequest"`
			} `yaml:"limits"`
		} `yaml:"spec"`
	}
	for _, m := range render(t) {
		switch m.Kind {
		case "ResourceQuota":
			quota = true
			if !strings.Contains(string(m.YAML), "requests.cpu") || !strings.Contains(string(m.YAML), "limits.cpu") {
				t.Error("the quota does not constrain cpu requests and limits")
			}
		case "LimitRange":
			limits = true
			if err := yaml.Unmarshal(m.YAML, &lr); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !quota || !limits {
		t.Fatalf("quota=%v limits=%v — both are required; the quota alone makes the namespace reject pods that omit requests", quota, limits)
	}
	if len(lr.Spec.Limits) == 0 || len(lr.Spec.Limits[0].Default) == 0 || len(lr.Spec.Limits[0].DefaultRequest) == 0 {
		t.Error("the LimitRange sets no default/defaultRequest — the quota then rejects any pod that omits them")
	}
}

// The namespace name belongs to the control plane (ADR-0012). Deriving a second
// opinion here is how two derivations drift.
func TestRenderRefusesANamespaceItDidNotExpect(t *testing.T) {
	for _, bad := range []Spec{
		{Namespace: "", Cell: "cell-0", EnvID: "e_1"},
		{Namespace: "env-e_1", Cell: "cell-0", EnvID: ""},
		{Namespace: "env-e_1", Cell: "", EnvID: "e_1"},
		{Namespace: "proj--env", Cell: "cell-0", EnvID: "e_1"}, // the pre-ADR-0012 shape
		{Namespace: "default", Cell: "cell-0", EnvID: "e_1"},
	} {
		if _, err := Render(bad); err == nil {
			t.Errorf("Render(%+v) was accepted — a wrong namespace applies the tenant boundary to the wrong place", bad)
		}
	}
}

// Rendering is deterministic; the applier reapplies these every converge.
func TestRenderIsDeterministic(t *testing.T) {
	first := render(t)
	for i := 0; i < 20; i++ {
		next := render(t)
		if len(next) != len(first) {
			t.Fatalf("run %d produced %d manifests, first produced %d", i, len(next), len(first))
		}
		for j := range first {
			if string(next[j].YAML) != string(first[j].YAML) || next[j].Kind != first[j].Kind {
				t.Fatalf("run %d differs at %s/%s — SSA reapplies these every converge", i, first[j].Kind, first[j].Name)
			}
		}
	}
}
