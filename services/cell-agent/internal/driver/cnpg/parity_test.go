package cnpg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	t.Fatal("repo root not found (AGENTS.md) — the ground-truth parity test must not silently disarm")
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

// TestBranchMatchesSpikeGroundTruth ties SnapshotBranch's recovery subtree to the
// VALIDATED spike manifest (infra/spike/manifests/spike-b-recovery.yaml — the one
// whose 52.4s branch e2e ADR-0007 measured). The zfs storageClass/affinity
// legitimately differ from the driver's pd output (excluded, like the cluster
// parity excludes name/initdb); the recovery structure must be field-identical.
func TestBranchMatchesSpikeGroundTruth(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "infra/spike/manifests/spike-b-recovery.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sub := strings.NewReplacer("spike-a-snap", "svc-db01-snap-1").Replace(string(raw))
	var truth map[string]any
	if err := yaml.Unmarshal([]byte(sub), &truth); err != nil {
		t.Fatal(err)
	}
	m, err := New().SnapshotBranch(driver.BranchSource{
		Name: "svc_db01", Namespace: "proj--prod", Cell: "cell-0",
		SnapshotName: "svc_db01-snap-1", Target: "svc_db01-branch",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(m[1].YAML, &got); err != nil { // m[1] is the Cluster
		t.Fatal(err)
	}
	for _, path := range []string{
		"spec.bootstrap.recovery.volumeSnapshots.storage.name",
		"spec.bootstrap.recovery.volumeSnapshots.storage.kind",
		"spec.bootstrap.recovery.volumeSnapshots.storage.apiGroup",
		"spec.postgresql.parameters.archive_timeout",
	} {
		if tv, gv := dig(truth, path), dig(got, path); !equalScalar(tv, gv) {
			t.Fatalf("%s drifted from spike-b-recovery.yaml: driver=%v, truth=%v", path, gv, tv)
		}
	}
}

// TestPITRMatchesSpikeGroundTruth ties PITRBranch's read-source-WAL contract to
// spike-pitr.yaml (ADR-0003 "restore never in place"). The externalClusters WAL
// path is the field the recovering pod actually reads — it must match ground
// truth, not just the driver's own golden.
func TestPITRMatchesSpikeGroundTruth(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "infra/spike/manifests/spike-pitr.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sub := strings.NewReplacer(
		"spike-a", "svc-db01",
		"__TARGET_TIME__", "2026-07-20T12:00:00Z",
		"__WAL_BUCKET__", "steloit-dev-wal-customer",
	).Replace(string(raw))
	var truth map[string]any
	if err := yaml.Unmarshal([]byte(sub), &truth); err != nil {
		t.Fatal(err)
	}
	m, err := New().PITRBranch(driver.BranchSource{
		Name: "svc_db01", Namespace: "proj--prod", Cell: "cell-0", Target: "svc_db01-pitr",
		WALBucket: "steloit-dev-wal-customer", GSAEmail: "g@x", HasArchivedWAL: true,
		TargetTime: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(m[0].YAML, &got); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"spec.bootstrap.recovery.source",
		"spec.bootstrap.recovery.recoveryTarget.targetTime",
	} {
		if tv, gv := dig(truth, path), dig(got, path); !equalScalar(tv, gv) {
			t.Fatalf("%s drifted from spike-pitr.yaml: driver=%v, truth=%v", path, gv, tv)
		}
	}
	// externalClusters[0] — the read-source-WAL path the pod actually uses.
	te, ge := extCluster(truth), extCluster(got)
	for _, k := range []string{"name"} {
		if !equalScalar(te[k], ge[k]) {
			t.Fatalf("externalClusters[0].%s drifted: driver=%v, truth=%v", k, ge[k], te[k])
		}
	}
	tb, _ := te["barmanObjectStore"].(map[string]any)
	gb, _ := ge["barmanObjectStore"].(map[string]any)
	if !equalScalar(tb["destinationPath"], gb["destinationPath"]) {
		t.Fatalf("PITR WAL read path drifted: driver=%v, truth=%v", gb["destinationPath"], tb["destinationPath"])
	}
	tc, _ := tb["googleCredentials"].(map[string]any)
	gc, _ := gb["googleCredentials"].(map[string]any)
	if !equalScalar(tc["gkeEnvironment"], gc["gkeEnvironment"]) {
		t.Fatal("PITR WAL read must use workload identity (gkeEnvironment), matching ground truth")
	}
}

func extCluster(m map[string]any) map[string]any {
	arr, _ := dig(m, "spec.externalClusters").([]any)
	if len(arr) == 0 {
		return map[string]any{}
	}
	c, _ := arr[0].(map[string]any)
	return c
}

// dnsName must always yield a valid RFC1123 name (a single example can't prove it).
func TestDNSNameIsAlwaysValid(t *testing.T) {
	valid := func(s string) bool {
		if s == "" || strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") {
			return false
		}
		for _, r := range s {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
		return true
	}
	for _, in := range []string{"svc_AbC_01", "___", "UPPER", "a.b.c", "svc-x", "9", "", "  spaces  ", "üñïçödé", "--lead--"} {
		if out := dnsName(in); !valid(out) {
			t.Fatalf("dnsName(%q) = %q — not a valid RFC1123 name", in, out)
		}
	}
}
