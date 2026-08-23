package tenancy_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/steloit/cloud/services/cell-agent/internal/driver/tenancy"
	"gopkg.in/yaml.v3"
)

type resourceQuota struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		Hard map[string]string `yaml:"hard"`
	} `yaml:"spec"`
}

func quotaFrom(t *testing.T, objs []tenancy.Manifest) resourceQuota {
	t.Helper()
	for _, m := range objs {
		if m.Kind != "ResourceQuota" {
			continue
		}
		var q resourceQuota
		if err := yaml.Unmarshal(m.YAML, &q); err != nil {
			t.Fatalf("the ResourceQuota does not parse: %v\n%s", err, m.YAML)
		}
		return q
	}
	t.Fatal("no ResourceQuota rendered — the environment has no ceiling")
	return resourceQuota{}
}

// THE CATALOG IS THE SOURCE, for all four plans.
//
// Read from plans.json rather than retyped: a hand-maintained copy is a second
// source of truth, and the cell-agent must not hold one (it is a separate module
// and never sees a plan name — the control plane resolves the envelope and ships
// the values). This test reads the file the way parity_test.go reads the repo
// root, so a plan whose numbers move here turns the driver red.
func TestEveryPlanRendersItsAuthoritativeEnvelope(t *testing.T) {
	plans := authoritativePlans(t)
	if len(plans) != 4 {
		t.Fatalf("expected exactly the four plans, got %d: %v", len(plans), plans)
	}
	for _, name := range []string{"free", "pro", "business", "enterprise"} {
		if _, ok := plans[name]; !ok {
			t.Fatalf("plans.json has no %q — orgs.plan's CHECK constraint lists it", name)
		}
	}

	for plan, want := range plans {
		t.Run(plan, func(t *testing.T) {
			objs, err := tenancy.Render(tenancy.Spec{
				Namespace: ns, Cell: cell,
				Quota: tenancy.Quota{CPU: want.CPU, Memory: want.Memory, Storage: want.Storage},
			})
			if err != nil {
				t.Fatal(err)
			}
			q := quotaFrom(t, objs)

			// The RIGHT namespace. A quota in the wrong one bounds a different
			// tenant and leaves this one unbounded.
			if q.Metadata.Namespace != ns {
				t.Fatalf("the ResourceQuota is scoped to %q, want %q", q.Metadata.Namespace, ns)
			}
			// Exactly the three dimensions, exactly the catalog's values.
			for key, wantVal := range map[string]string{
				"requests.cpu":     want.CPU,
				"requests.memory":  want.Memory,
				"requests.storage": want.Storage,
			} {
				if got := q.Spec.Hard[key]; got != wantVal {
					t.Errorf("%s = %q, want plans.json's %q", key, got, wantVal)
				}
			}
			if len(q.Spec.Hard) != 3 {
				t.Errorf("spec.hard has %d keys (%v) — an extra dimension is one nobody approved, "+
					"a missing one is unbounded", len(q.Spec.Hard), q.Spec.Hard)
			}
		})
	}

	// NO TWO PLANS SHARE AN ENVELOPE. If they did, a test that rendered the wrong
	// plan's limits would still pass every assertion above.
	seen := map[string]string{}
	for plan, e := range plans {
		key := e.CPU + "|" + e.Memory + "|" + e.Storage
		if other, dup := seen[key]; dup {
			t.Errorf("%q and %q have identical envelopes (%s) — the wrong-plan case is "+
				"undetectable while that is true", plan, other, key)
		}
		seen[key] = plan
	}
}

// A quota rendered with ANOTHER plan's limits must be detectable. This is the
// negative half of the test above: it drives free's envelope and asserts the
// output is not pro's.
func TestOnePlansEnvelopeIsNotAnothers(t *testing.T) {
	plans := authoritativePlans(t)
	free, pro := plans["free"], plans["pro"]

	objs, err := tenancy.Render(tenancy.Spec{Namespace: ns, Cell: cell,
		Quota: tenancy.Quota{CPU: free.CPU, Memory: free.Memory, Storage: free.Storage}})
	if err != nil {
		t.Fatal(err)
	}
	q := quotaFrom(t, objs)
	if q.Spec.Hard["requests.cpu"] == pro.CPU {
		t.Fatalf("a free environment rendered pro's CPU ceiling (%s)", pro.CPU)
	}
	if q.Spec.Hard["requests.cpu"] != free.CPU {
		t.Fatalf("free rendered %q, want %q", q.Spec.Hard["requests.cpu"], free.CPU)
	}
}

// FAIL CLOSED. Every one of these would otherwise render a ResourceQuota the API
// server rejects at apply — a config problem discovered on a cell — or, for an
// absent envelope, an environment with no ceiling at all.
func TestAMalformedOrMissingEnvelopeIsRefused(t *testing.T) {
	good := tenancy.Quota{CPU: "8", Memory: "16Gi", Storage: "100Gi"}
	for name, q := range map[string]tenancy.Quota{
		// (asserted separately below, on its message)
		"cpu missing":         {Memory: "16Gi", Storage: "100Gi"},
		"memory missing":      {CPU: "8", Storage: "100Gi"},
		"storage missing":     {CPU: "8", Memory: "16Gi"},
		"cpu zero":            {CPU: "0", Memory: "16Gi", Storage: "100Gi"},
		"memory zero":         {CPU: "8", Memory: "0Gi", Storage: "100Gi"},
		"storage zero":        {CPU: "8", Memory: "16Gi", Storage: "0Gi"},
		"cpu negative":        {CPU: "-8", Memory: "16Gi", Storage: "100Gi"},
		"memory negative":     {CPU: "8", Memory: "-16Gi", Storage: "100Gi"},
		"cpu fractional":      {CPU: "0.5", Memory: "16Gi", Storage: "100Gi"},
		"cpu millicores":      {CPU: "500m", Memory: "16Gi", Storage: "100Gi"},
		"memory unitless":     {CPU: "8", Memory: "16", Storage: "100Gi"},
		"memory decimal unit": {CPU: "8", Memory: "16GB", Storage: "100Gi"},
		"storage nonsense":    {CPU: "8", Memory: "16Gi", Storage: "lots"},
		"cpu leading zero":    {CPU: "08", Memory: "16Gi", Storage: "100Gi"},
		"whitespace":          {CPU: " 8", Memory: "16Gi", Storage: "100Gi"},
		"yaml injection":      {CPU: "8", Memory: "16Gi\n    requests.cpu: \"999\"", Storage: "100Gi"},
		"exponent":            {CPU: "1e3", Memory: "16Gi", Storage: "100Gi"},
		"plus sign":           {CPU: "+8", Memory: "16Gi", Storage: "100Gi"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tenancy.Render(tenancy.Spec{Namespace: ns, Cell: cell, Quota: q}); err == nil {
				t.Fatalf("Render accepted %+v", q)
			}
		})
	}

	// AN ABSENT envelope is refused with its OWN message. Without that assertion
	// the guard is an equivalent mutant: an empty Quota also fails the grammar
	// check, so deleting the absence check changes nothing a test could see —
	// while turning a clear "no envelope was resolved" into a confusing
	// "cpu quota \"\" is not a whole number of cores".
	_, err := tenancy.Render(tenancy.Spec{Namespace: ns, Cell: cell})
	if err == nil {
		t.Fatal("Render accepted a spec with no envelope at all — the environment would have " +
			"no ceiling")
	}
	if !strings.Contains(err.Error(), "no quota envelope") {
		t.Fatalf("an absent envelope must say so, not fail as a malformed quantity: %v", err)
	}

	// An absurdly large but WELL-FORMED quantity is accepted: it is a founder
	// configuration error, not a malformed one, and the closed grammar's job is
	// shape rather than judgement. Recorded so the omission is deliberate.
	if _, err := tenancy.Render(tenancy.Spec{Namespace: ns, Cell: cell,
		Quota: tenancy.Quota{CPU: "999999999", Memory: "999999999Ti", Storage: "999999999Ti"}}); err != nil {
		t.Fatalf("a well-formed but enormous envelope was refused: %v", err)
	}

	// Positive control: the good one still renders, or every case above is
	// satisfied by a Render that refuses everything.
	if _, err := tenancy.Render(tenancy.Spec{Namespace: ns, Cell: cell, Quota: good}); err != nil {
		t.Fatalf("a legitimate envelope was refused: %v", err)
	}
}

// THE QUOTA AND THE LIMITRANGE ARE A PAIR, and that is a direct consequence of
// enforcement being real. A ResourceQuota constraining requests.cpu/memory makes
// the API server REJECT any pod that declares neither, so shipping the quota
// alone would make the namespace refuse ordinary pods.
//
// The LimitRange must supply defaultRequest and must NOT supply `default`: a
// default LIMIT becomes a hard cap on any container that declares nothing, which
// is how an earlier revision would have capped every managed Postgres at 512Mi.
func TestTheLimitRangeSuppliesRequestsWithoutCappingAnything(t *testing.T) {
	var lr struct {
		Spec struct {
			Limits []struct {
				Type           string            `yaml:"type"`
				Default        map[string]string `yaml:"default"`
				DefaultRequest map[string]string `yaml:"defaultRequest"`
			} `yaml:"limits"`
		} `yaml:"spec"`
	}
	var found bool
	for _, m := range mustRender(t) {
		if m.Kind != "LimitRange" {
			continue
		}
		found = true
		if err := yaml.Unmarshal(m.YAML, &lr); err != nil {
			t.Fatal(err)
		}
	}
	if !found {
		t.Fatal("no LimitRange — a namespace with a cpu/memory quota REJECTS every pod that " +
			"declares no requests, so the quota alone would refuse ordinary pods")
	}
	if len(lr.Spec.Limits) == 0 {
		t.Fatal("the LimitRange declares no limits")
	}
	for _, l := range lr.Spec.Limits {
		// TYPE MUST BE Container. Kubernetes FORBIDS default/defaultRequest on a
		// Pod-type limit ("may not be specified when `type` is 'Pod'"), so this
		// one word decides whether the object is accepted at all — and a rejected
		// LimitRange aborts the Apply before the Cluster, stopping EVERY service
		// in the environment from converging. It was unasserted and `Pod` was a
		// green mutation.
		if l.Type != "Container" {
			t.Errorf("limit type is %q, want Container — the API server Forbids defaultRequest "+
				"on a Pod-type limit, so every apply would 422 and nothing in the environment "+
				"would converge", l.Type)
		}
		// BOTH dimensions. The quota constrains requests.cpu AND requests.memory,
		// so a defaultRequest missing either still lets the quota reject every
		// undeclared pod — which is the failure the pair exists to prevent.
		// Dropping the cpu key was a green mutation.
		for _, dim := range []string{"cpu", "memory"} {
			if l.DefaultRequest[dim] == "" {
				t.Errorf("%s supplies no defaultRequest for %s — the quota then rejects every "+
					"pod that declares none", l.Type, dim)
			}
		}
		if len(l.Default) != 0 {
			t.Errorf("%s supplies a default LIMIT (%v). That becomes the hard cap on every "+
				"container that declares none, including the CNPG Cluster (which declares no "+
				"resources until US-3.3d), and OOMKills it.", l.Type, l.Default)
		}
	}
}

// THE DEFAULTS MUST FIT INSIDE THE SMALLEST PLAN.
//
// A defaultRequest is what every undeclared pod reserves, so if it exceeds a
// plan's whole envelope that plan admits NOTHING. Raising 100m/128Mi to 2/4Gi —
// larger than free's entire 1 vCPU / 2 GiB ceiling — was a green mutation.
//
// Asserted as a PROPERTY against the smallest envelope in plans.json rather than
// as literals, so retuning the defaults stays legal and "bigger than the whole
// free plan" cannot ship.
func TestTheLimitRangeDefaultsFitInsideTheSmallestPlanEnvelope(t *testing.T) {
	free, ok := authoritativePlans(t)["free"]
	if !ok {
		t.Fatal("plans.json has no free plan")
	}
	var lr struct {
		Spec struct {
			Limits []struct {
				DefaultRequest map[string]string `yaml:"defaultRequest"`
			} `yaml:"limits"`
		} `yaml:"spec"`
	}
	for _, m := range mustRender(t) {
		if m.Kind == "LimitRange" {
			if err := yaml.Unmarshal(m.YAML, &lr); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, l := range lr.Spec.Limits {
		if got, ceiling := milli(t, l.DefaultRequest["cpu"]), milli(t, free.CPU); got >= ceiling {
			t.Errorf("defaultRequest cpu %s is not smaller than free's whole envelope %s — a free "+
				"environment would admit no pod that declares nothing", l.DefaultRequest["cpu"], free.CPU)
		}
		if got, ceiling := bytesOf(t, l.DefaultRequest["memory"]), bytesOf(t, free.Memory); got >= ceiling {
			t.Errorf("defaultRequest memory %s is not smaller than free's whole envelope %s",
				l.DefaultRequest["memory"], free.Memory)
		}
	}
}

// milli parses the two CPU spellings that appear here: whole cores ("1") and
// millicores ("100m"). Deliberately tiny — this is a test helper, not a second
// quantity parser for production to depend on.
func milli(t *testing.T, v string) int64 {
	t.Helper()
	if strings.HasSuffix(v, "m") {
		n, err := strconv.ParseInt(strings.TrimSuffix(v, "m"), 10, 64)
		if err != nil {
			t.Fatalf("cpu %q: %v", v, err)
		}
		return n
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		t.Fatalf("cpu %q: %v", v, err)
	}
	return n * 1000
}

func bytesOf(t *testing.T, v string) int64 {
	t.Helper()
	for suf, mult := range map[string]int64{"Mi": 1 << 20, "Gi": 1 << 30, "Ti": 1 << 40} {
		if strings.HasSuffix(v, suf) {
			n, err := strconv.ParseInt(strings.TrimSuffix(v, suf), 10, 64)
			if err != nil {
				t.Fatalf("memory %q: %v", v, err)
			}
			return n * mult
		}
	}
	t.Fatalf("memory %q has no recognised unit", v)
	return 0
}

// authoritativePlans reads the envelope out of plans.json — the ONE definition.
func authoritativePlans(t *testing.T) map[string]tenancy.Quota {
	t.Helper()
	root := repoRootFor(t)
	raw, err := os.ReadFile(filepath.Join(root, "services/api/internal/billing/plans.json"))
	if err != nil {
		t.Fatalf("read the authoritative plan table: %v", err)
	}
	var doc struct {
		Plans map[string]struct {
			Quota struct {
				CPU     string `json:"cpu"`
				Memory  string `json:"memory"`
				Storage string `json:"storage"`
			} `json:"quota"`
		} `json:"plans"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse plans.json: %v", err)
	}
	out := map[string]tenancy.Quota{}
	for name, p := range doc.Plans {
		out[name] = tenancy.Quota{CPU: p.Quota.CPU, Memory: p.Quota.Memory, Storage: p.Quota.Storage}
	}
	if len(out) == 0 {
		t.Fatal("plans.json lists no plans — this test would prove nothing")
	}
	return out
}

func repoRootFor(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("repo root not found (AGENTS.md)")
	return ""
}
