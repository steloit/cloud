package cnpg

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/cell-agent/internal/driver"
)

var update = flag.Bool("update", false, "regenerate golden manifests")

func goldenCheck(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden.yaml")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run -update to create): %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s does not match golden %s:\n--- got ---\n%s\n--- want ---\n%s", name, path, got, want)
	}
}

func devSpec() driver.Spec {
	return driver.Spec{
		Name: "svc_db01", Namespace: "proj--prod", Product: "postgres", Intent: "database",
		Shape: map[string]any{"size": "dev"}, Instances: 1, Cell: "cell-0",
		GSAEmail: "ci-image-push@steloit-dev.iam.gserviceaccount.com", WALBucket: "steloit-dev-wal-customer",
	}
}

func TestRenderClusterMatchesGolden(t *testing.T) {
	m, err := New().Render(devSpec())
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || m[0].Kind != "Cluster" || m[1].Kind != "ScheduledBackup" {
		t.Fatalf("expected [Cluster, ScheduledBackup], got %d manifests", len(m))
	}
	goldenCheck(t, "cluster", m[0].YAML)
	goldenCheck(t, "scheduled-backup", m[1].YAML)
}

// The measured RPO bound (ADR-0007 F4 / A1.3) must be encoded, not incidental.
func TestArchiveTimeoutIsMeasuredRPO(t *testing.T) {
	m, _ := New().Render(devSpec())
	if !strings.Contains(string(m[0].YAML), `archive_timeout: "300s"`) {
		t.Fatal("archive_timeout must be the measured 300s RPO bound (ADR-0007)")
	}
	// WAL to GCS via workload identity, zero static keys (D5/F5).
	if !strings.Contains(string(m[0].YAML), "gkeEnvironment: true") {
		t.Fatal("WAL archiving must use workload identity (gkeEnvironment), never static keys")
	}
}

func TestRenderFromShapeSizes(t *testing.T) {
	cases := map[string]string{"dev": "10Gi", "standard": "32Gi", "pro": "128Gi", "unknown": "10Gi"}
	for size, want := range cases {
		s := devSpec()
		s.Shape = map[string]any{"size": size}
		m, err := New().Render(s)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(m[0].YAML), "size: "+want) {
			t.Fatalf("size %q → want storage %q, manifest:\n%s", size, want, m[0].YAML)
		}
	}
	// explicit shape.storage wins over the size mapping
	s := devSpec()
	s.Shape = map[string]any{"size": "dev", "storage": "64Gi"}
	m, _ := New().Render(s)
	if !strings.Contains(string(m[0].YAML), "size: 64Gi") {
		t.Fatal("explicit shape.storage must override the size mapping")
	}
}

func TestSnapshotBranchManifest(t *testing.T) {
	m, err := New().SnapshotBranch(driver.BranchSource{
		Name: "svc_db01", Namespace: "proj--prod", Cell: "cell-0", SnapshotName: "svc_db01-snap-1", Target: "svc_db01-branch",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Apply order: snapshot BEFORE the cluster that recovers from it.
	if len(m) != 2 || m[0].Kind != "VolumeSnapshot" || m[1].Kind != "Cluster" {
		t.Fatalf("expected [VolumeSnapshot, Cluster], got %+v", []string{m[0].Kind, m[1].Kind})
	}
	if !strings.Contains(string(m[1].YAML), "bootstrap:") || !strings.Contains(string(m[1].YAML), "recovery:") {
		t.Fatal("branch cluster must bootstrap from recovery (the D2 primitive)")
	}
	goldenCheck(t, "snapshot", m[0].YAML)
	goldenCheck(t, "branch-cluster", m[1].YAML)
}

func TestBranchRecoveryFromSnapshot(t *testing.T) {
	m, _ := New().SnapshotBranch(driver.BranchSource{
		Name: "svc_db01", Namespace: "proj--prod", Cell: "cell-0", SnapshotName: "svc_db01-snap-1", Target: "svc_db01-branch",
	})
	if !strings.Contains(string(m[1].YAML), "svc-db01-snap-1") {
		t.Fatal("branch cluster must recover from the named snapshot")
	}
}

func TestPITRBranchManifest(t *testing.T) {
	ts := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m, err := New().PITRBranch(driver.BranchSource{
		Name: "svc_db01", Namespace: "proj--prod", Target: "svc_db01-pitr", Cell: "cell-0",
		WALBucket: "steloit-dev-wal-customer", GSAEmail: "ci-image-push@steloit-dev.iam.gserviceaccount.com",
		HasArchivedWAL: true, TargetTime: ts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(m[0].YAML), `targetTime: "2026-07-20T12:00:00Z"`) {
		t.Fatalf("PITR must render the recovery target time:\n%s", m[0].YAML)
	}
	goldenCheck(t, "pitr-cluster", m[0].YAML)
}

// A PITR with no archived-WAL basis is REFUSED, never rendered (ADR-0007 F4).
func TestPITRRequiresArchivedWAL(t *testing.T) {
	_, err := New().PITRBranch(driver.BranchSource{
		Name: "svc_db01", Namespace: "proj--prod", Target: "svc_db01-pitr",
		HasArchivedWAL: false, TargetTime: time.Now(),
	})
	if err == nil {
		t.Fatal("PITR without an archived-WAL basis must be refused (recovery targets derive from WAL, never wall-clock)")
	}
	if !strings.Contains(err.Error(), "wall-clock") {
		t.Fatalf("the refusal must name the F4 reason, got: %v", err)
	}
}

func TestHibernateWakePatch(t *testing.T) {
	h, err := New().Hibernate("svc_db01", "proj--prod")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(h.Body), `"cnpg.io/hibernation":"on"`) {
		t.Fatalf("hibernate patch wrong: %s", h.Body)
	}
	w, _ := New().Wake("svc_db01", "proj--prod")
	if !strings.Contains(string(w.Body), `"cnpg.io/hibernation":"off"`) {
		t.Fatalf("wake patch wrong: %s", w.Body)
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	// Same desired → byte-identical manifests. Use a MULTI-KEY shape map so this
	// is not vacuous: if any render path ever ranged over the shape map, key
	// order would vary and this would catch it.
	s := devSpec()
	s.Shape = map[string]any{"size": "dev", "storage": "20Gi", "a": 1, "b": 2, "c": 3, "z": 9}
	first, _ := New().Render(s)
	for range 50 {
		again, _ := New().Render(s)
		if string(again[0].YAML) != string(first[0].YAML) {
			t.Fatalf("render is not deterministic — multi-key shape leaked ordering:\n%s\nvs\n%s", first[0].YAML, again[0].YAML)
		}
	}
}

func TestRenderSanitizesToRFC1123Name(t *testing.T) {
	s := devSpec()
	s.Name = "svc_AbC_01" // service ids carry underscores/case — invalid k8s names
	m, err := New().Render(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(m[0].YAML), "name: svc-abc-01") {
		t.Fatalf("service id not sanitized to an RFC1123 name:\n%s", m[0].YAML)
	}
	if strings.Contains(string(m[0].YAML), "svc_AbC_01") {
		t.Fatal("the raw underscored/upper id leaked into metadata.name (k8s would reject it)")
	}
}

func TestRenderErrorPaths(t *testing.T) {
	d := New()
	if _, err := d.Render(driver.Spec{Product: "postgres", Namespace: "ns"}); err == nil {
		t.Fatal("empty name must error")
	}
	if _, err := d.Render(driver.Spec{Product: "postgres", Name: "x"}); err == nil {
		t.Fatal("empty namespace must error")
	}
	// nil shape must not panic (falls to dev)
	m, err := d.Render(driver.Spec{Product: "postgres", Name: "x", Namespace: "ns", Shape: nil})
	if err != nil || !strings.Contains(string(m[0].YAML), "size: 10Gi") {
		t.Fatalf("nil shape should default to dev 10Gi: %v", err)
	}
	// non-string size must not panic
	m2, err := d.Render(driver.Spec{Product: "postgres", Name: "x", Namespace: "ns", Shape: map[string]any{"size": 42}})
	if err != nil || !strings.Contains(string(m2[0].YAML), "size: 10Gi") {
		t.Fatalf("non-string size should default to dev: %v", err)
	}
}

func TestInstancesClampAndFlowThrough(t *testing.T) {
	s := devSpec()
	s.Instances = 0
	m, _ := New().Render(s)
	if !strings.Contains(string(m[0].YAML), "instances: 1") {
		t.Fatal("instances<1 must clamp to 1")
	}
	s.Instances = 3
	m, _ = New().Render(s)
	if !strings.Contains(string(m[0].YAML), "instances: 3") {
		t.Fatal("instances must flow through")
	}
}

func TestPITRRequiresTargetTimeAndPlacement(t *testing.T) {
	base := driver.BranchSource{Name: "s", Namespace: "ns", Target: "t", HasArchivedWAL: true,
		WALBucket: "b", GSAEmail: "g@x"}
	// zero target time refused
	if _, err := New().PITRBranch(base); err == nil {
		t.Fatal("PITR with zero target time must be refused")
	}
	// missing WAL bucket / GSA refused (the pod could not read the source WAL)
	nb := base
	nb.TargetTime = time.Now()
	nb.WALBucket = ""
	if _, err := New().PITRBranch(nb); err == nil {
		t.Fatal("PITR without the source WAL bucket must be refused")
	}
}

func TestBranchAndPITRErrorPaths(t *testing.T) {
	d := New()
	if _, err := d.SnapshotBranch(driver.BranchSource{Namespace: "ns", Target: "t", SnapshotName: "s"}); err == nil {
		t.Fatal("snapshot branch with empty source name must error")
	}
	if _, err := d.SnapshotBranch(driver.BranchSource{Name: "s", Namespace: "ns"}); err == nil {
		t.Fatal("snapshot branch without Target/SnapshotName must error")
	}
	if _, err := d.Hibernate("", "ns"); err == nil {
		t.Fatal("hibernate with empty cluster must error")
	}
}

func TestRenderRejectsNonPostgres(t *testing.T) {
	s := devSpec()
	s.Product = "valkey"
	if _, err := New().Render(s); err == nil {
		t.Fatal("the CNPG driver must reject a non-postgres product")
	}
}
