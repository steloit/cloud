package tenancy_test

import (
	"strings"
	"testing"

	"github.com/steloit/cloud/services/cell-agent/internal/driver/tenancy"
	"gopkg.in/yaml.v3"
)

const (
	ns   = "env-9f3c1a2b"
	cell = "cell0"
)

func mustRender(t *testing.T) []tenancy.Manifest {
	t.Helper()
	objs, err := tenancy.Render(tenancy.Spec{Namespace: ns, Cell: cell})
	if err != nil {
		t.Fatal(err)
	}
	return objs
}

// The environment's namespace is the object nothing in the system created before
// US-3.3a — the defect was a 404 on the first converge into a new environment.
func TestRenderProducesTheNamespace(t *testing.T) {
	objs := mustRender(t)
	if len(objs) != 1 {
		t.Fatalf("want exactly the Namespace, got %d objects: %+v", len(objs), kinds(objs))
	}
	if objs[0].Kind != "Namespace" || objs[0].Name != ns {
		t.Fatalf("got %s/%s, want Namespace/%s", objs[0].Kind, objs[0].Name, ns)
	}
}

// D7's NetworkPolicies, ResourceQuota and LimitRange are deliberately absent
// until US-3.3c lands them WITH enforcement and a CNPG allow-set. This pins the
// absence so it stays a decision: re-adding any of them without changing this
// test is not possible, and changing it means reading why.
func TestTheD7PolicyObjectsAreDeliberatelyNotRenderedYet(t *testing.T) {
	for _, m := range mustRender(t) {
		switch m.Kind {
		case "NetworkPolicy", "ResourceQuota", "LimitRange":
			t.Fatalf("%s/%s is rendered again. It must not ship before US-3.3c: "+
				"NetworkPolicies are not enforced on the cell (gke-cell sets no "+
				"network_policy / ADVANCED_DATAPATH), the allow-set as written fences "+
				"CNPG off its metadata server, GCS and the apiserver, and the LimitRange "+
				"default becomes the hard cap on every managed Postgres.", m.Kind, m.Name)
		}
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
			if _, err := tenancy.Render(tenancy.Spec{Namespace: v, Cell: cell}); err == nil {
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
			if _, err := tenancy.Render(tenancy.Spec{Namespace: ns, Cell: v}); err == nil {
				t.Fatalf("Render accepted cell %q", v)
			}
		})
	}

	// The negative half is only meaningful if the positive half still passes:
	// a Render that refuses everything would satisfy every case above.
	for _, good := range []string{"env-a", "env-9f3c1a2b", "env-" + strings.Repeat("a", 59)} {
		if _, err := tenancy.Render(tenancy.Spec{Namespace: good, Cell: cell}); err != nil {
			t.Fatalf("Render refused a legitimate namespace %q: %v", good, err)
		}
	}
}

// Every namespace the control plane can mint must be one Render accepts. If this
// ever parts company, the agent hard-errors on EVERY converge for that
// environment and the control plane sees no signal at all.
func TestRenderAcceptsEveryShapeTheControlPlaneCanMint(t *testing.T) {
	// namespaceForEnv is services/api-side: k8sNamespace lowercases, replaces
	// each run of invalid characters with "-", trims dashes, truncates to 63.
	// These are the outputs it produces for ids of the shape env_<hex>.
	for _, produced := range []string{
		"env-9f3c1a2b",
		"env-0",
		"env-" + strings.Repeat("f", 59), // exactly 63
	} {
		if _, err := tenancy.Render(tenancy.Spec{Namespace: produced, Cell: cell}); err != nil {
			t.Fatalf("the control plane can mint %q and Render refuses it: %v", produced, err)
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
