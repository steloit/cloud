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
	if len(m) != 1 || m[0].Kind != "Cluster" {
		t.Fatalf("expected one Cluster manifest, got %+v", m)
	}
	goldenCheck(t, "cluster", m[0].YAML)
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
		Name: "svc_db01", Namespace: "proj--prod", SnapshotName: "svc_db01-snap-1", Target: "svc_db01-branch",
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
		Name: "svc_db01", Namespace: "proj--prod", SnapshotName: "svc_db01-snap-1", Target: "svc_db01-branch",
	})
	if !strings.Contains(string(m[1].YAML), "svc_db01-snap-1") {
		t.Fatal("branch cluster must recover from the named snapshot")
	}
}

func TestPITRBranchManifest(t *testing.T) {
	ts := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m, err := New().PITRBranch(driver.BranchSource{
		Name: "svc_db01", Namespace: "proj--prod", Target: "svc_db01-pitr", HasArchivedWAL: true, TargetTime: ts,
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
	// Same desired → byte-identical manifests, across many renders (maps in the
	// spec must not leak nondeterministic ordering into the output).
	first, _ := New().Render(devSpec())
	for range 50 {
		again, _ := New().Render(devSpec())
		if string(again[0].YAML) != string(first[0].YAML) {
			t.Fatal("render is not deterministic — same desired produced different manifests")
		}
	}
}

func TestRenderRejectsNonPostgres(t *testing.T) {
	s := devSpec()
	s.Product = "valkey"
	if _, err := New().Render(s); err == nil {
		t.Fatal("the CNPG driver must reject a non-postgres product")
	}
}
