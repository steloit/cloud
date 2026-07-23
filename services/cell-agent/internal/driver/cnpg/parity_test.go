package cnpg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/steloit/cloud/services/cell-agent/internal/driver"
)

// TestClusterMatchesGroundTruthManifest is the ANTI-CIRCULARITY test: the golden
// files are self-generated (they prove determinism, not correctness), so this
// ties the driver's render to the VALIDATED infra manifest that T1.0 actually
// applied — infra/k8s/control-plane/cnpg-cluster.yaml. It substitutes the same
// template vars the driver uses, parses both, and asserts the LOAD-BEARING
// subtree (the fields ADR-0007 measured) is field-equal. A driver that drifts
// from ground truth fails HERE, not just against its own golden.
func TestClusterMatchesGroundTruthManifest(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "infra/k8s/control-plane/cnpg-cluster.yaml"))
	if err != nil {
		t.Fatalf("read ground-truth manifest: %v", err)
	}
	// The infra file is a Terraform template; substitute the same values the
	// driver renders so the two are comparable.
	sub := strings.NewReplacer(
		"${cluster_name}", "svc_db01",
		"${namespace}", "proj--prod",
		"${storage_size}", "10Gi",
		"${gsa_email}", "ci-image-push@steloit-dev.iam.gserviceaccount.com",
		"${wal_control_bucket}", "steloit-dev-wal-customer",
	).Replace(string(raw))
	var truth map[string]any
	if err := yaml.Unmarshal([]byte(sub), &truth); err != nil {
		t.Fatalf("parse ground truth: %v", err)
	}

	m, err := New().Render(devSpec())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(m[0].YAML, &got); err != nil {
		t.Fatalf("parse render: %v", err)
	}

	// Load-bearing paths the driver MUST reproduce from ground truth. (Fields
	// that legitimately differ — object name, destinationPath suffix, the
	// control-plane's initdb — are excluded on purpose.)
	for _, path := range []string{
		"spec.instances",
		"spec.storage.storageClass",
		"spec.affinity.nodeSelector.pool",
		"spec.postgresql.parameters.archive_timeout",
		"spec.backup.barmanObjectStore.googleCredentials.gkeEnvironment",
		"spec.backup.retentionPolicy",
		"apiVersion",
		"kind",
	} {
		tv, gv := dig(truth, path), dig(got, path)
		if tv == nil {
			t.Fatalf("ground truth missing %s — update the parity paths", path)
		}
		if !equalScalar(tv, gv) {
			t.Fatalf("%s drifted from ground truth: driver=%v, cnpg-cluster.yaml=%v", path, gv, tv)
		}
	}
	// The storage-taint toleration must be present (a pod without it cannot land
	// on the tainted db-storage pool, T1.2).
	if !hasStorageToleration(got) {
		t.Fatal("render dropped the storage-taint toleration ground truth carries")
	}
}

func dig(m map[string]any, path string) any {
	var cur any = m
	for _, k := range strings.Split(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func equalScalar(a, b any) bool {
	return strings.TrimSpace(strings.Trim(toStr(a), `"`)) == strings.TrimSpace(strings.Trim(toStr(b), `"`))
}

func toStr(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return strings.TrimSpace(yamlScalar(x))
	}
}

func yamlScalar(v any) string {
	b, _ := yaml.Marshal(v)
	return strings.TrimSpace(string(b))
}

func hasStorageToleration(m map[string]any) bool {
	aff, _ := dig(m, "spec.affinity").(map[string]any)
	tols, _ := aff["tolerations"].([]any)
	for _, t := range tols {
		tm, _ := t.(map[string]any)
		if tm["key"] == "storage" && tm["effect"] == "NoSchedule" {
			return true
		}
	}
	return false
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// walk up from the test dir until we find go.work-less repo markers (AGENTS.md)
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("repo root not found (AGENTS.md) — parity test needs the infra manifests")
	return ""
}

// The F3 base backup must be rendered alongside the cluster (WAL archiving alone
// is not restorable — cnpg-scheduled-backup.yaml is load-bearing from day one).
func TestRenderEmitsBaseBackup(t *testing.T) {
	m, _ := New().Render(devSpec())
	var found bool
	for _, obj := range m {
		if obj.Kind == "ScheduledBackup" {
			found = true
			if !strings.Contains(string(obj.YAML), "immediate: true") {
				t.Fatal("the base backup must be immediate:true (restorable from apply time, F3)")
			}
		}
	}
	if !found {
		t.Fatal("Render must emit a ScheduledBackup — WAL archiving alone is not restorable (ADR-0007 F3)")
	}
	_ = driver.Spec{}
}
