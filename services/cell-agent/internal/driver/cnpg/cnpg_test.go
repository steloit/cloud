package cnpg

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// THE CATALOG IS THE SOURCE. Read from pricing.json rather than retyped, because
// a hand-maintained list is exactly what let two defects ship: `standard`
// rendered 32Gi while its base price includes 50 GB, and `performance` — the
// most expensive size — had no case at all and fell through to the smallest
// volume. A list can be wrong in the same direction as the code it checks.
//
// The cell-agent is a separate Go module and must not IMPORT the API's pricing
// table; the test reads the file from the repo root, the precedent parity_test.go
// already sets.
func TestEveryCatalogSizeRendersAtLeastItsIncludedStorage(t *testing.T) {
	for size, includedGB := range catalogSizes(t) {
		spec := devSpec()
		spec.Shape = map[string]any{"size": size}
		m, err := New().Render(spec)
		if err != nil {
			t.Fatalf("size %q is in the catalog and the driver refuses it: %v", size, err)
		}
		got := renderedStorageGB(t, m[0].YAML)
		if got < includedGB {
			t.Errorf("size %q includes %d GB but renders a %dGi PVC — the customer is billed "+
				"for storage the volume does not have", size, includedGB, got)
		}
		if got <= 0 {
			t.Errorf("size %q renders a %dGi PVC, which is not a volume", size, got)
		}
	}
}

// The priced storage_gb must reach the PVC. This is the filed defect: the driver
// read `shape["storage"]`, a key the API's closed schema rejects, so the priced
// value was never read and a customer billed for 78 GB got the size default.
func TestThePricedStorageGBSizesTheVolume(t *testing.T) {
	for _, tc := range []struct {
		name string
		val  any
		want int
	}{
		{"int", 78, 78},
		{"float64 as it arrives over JSON", float64(78), 78},
		{"int64", int64(78), 78},
		{"below the size floor is raised to it", 1, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := devSpec()
			spec.Shape = map[string]any{"size": "standard", "storage_gb": tc.val}
			m, err := New().Render(spec)
			if err != nil {
				t.Fatal(err)
			}
			if got := renderedStorageGB(t, m[0].YAML); got != tc.want {
				t.Fatalf("storage_gb %v (%T) → %dGi, want %dGi", tc.val, tc.val, got, tc.want)
			}
		})
	}

	// The legacy `storage` key is REMOVED, not dual-read. It was never in the
	// API's schema, so nothing can legitimately send it; continuing to honour it
	// would keep a second, unpriced way to size a volume.
	spec := devSpec()
	spec.Shape = map[string]any{"size": "dev", "storage": "64Gi"}
	m, err := New().Render(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got := renderedStorageGB(t, m[0].YAML); got != 10 {
		t.Fatalf("the legacy `storage` key still sizes the volume (%dGi) — it is not in "+
			"estimates.shapeSchema and must not be a second, unpriced input", got)
	}
}

// A size the catalog does not know must be REFUSED, not silently sized. The old
// `default:` arm is how `performance` shipped with the smallest volume.
func TestAnUncatalogedSizeIsRefusedRatherThanGuessed(t *testing.T) {
	for _, size := range []string{"pro", "xl", "Standard", "performance-2"} {
		spec := devSpec()
		spec.Shape = map[string]any{"size": size}
		if _, err := New().Render(spec); err == nil {
			t.Errorf("size %q was silently sized instead of refused", size)
		}
	}
	// Positive control: every catalog size must still render, or the above would
	// be satisfied by a driver that refuses everything.
	for size := range catalogSizes(t) {
		spec := devSpec()
		spec.Shape = map[string]any{"size": size}
		if _, err := New().Render(spec); err != nil {
			t.Errorf("catalog size %q was refused: %v", size, err)
		}
	}
}

// catalogSizes reads postgres.sizes[*].included_gb from the API's pricing table.
func catalogSizes(t *testing.T) map[string]int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "services/api/internal/estimates/pricing.json"))
	if err != nil {
		t.Fatalf("read the catalog: %v", err)
	}
	var doc struct {
		Postgres struct {
			Sizes map[string]struct {
				IncludedGB int `json:"included_gb"`
			} `json:"sizes"`
		} `json:"postgres"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse the catalog: %v", err)
	}
	if len(doc.Postgres.Sizes) == 0 {
		t.Fatal("the catalog lists no postgres sizes — this test would prove nothing")
	}
	out := map[string]int{}
	for k, v := range doc.Postgres.Sizes {
		out[k] = v.IncludedGB
	}
	return out
}

var storageRe = regexp.MustCompile(`(?m)^\s*size:\s*(\d+)Gi\s*$`)

func renderedStorageGB(t *testing.T, yaml []byte) int {
	t.Helper()
	m := storageRe.FindSubmatch(yaml)
	if m == nil {
		t.Fatalf("no storage size in the rendered manifest:\n%s", yaml)
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatal(err)
	}
	return n
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
	// An ABSENT size is contractually dev — that is the API's closed-schema
	// default (estimates.shapeSchema), so matching it is following the contract
	// rather than guessing.
	m, err := d.Render(driver.Spec{Product: "postgres", Name: "x", Namespace: "ns", Shape: nil})
	if err != nil || !strings.Contains(string(m[0].YAML), "size: 10Gi") {
		t.Fatalf("nil shape should default to dev 10Gi: %v", err)
	}
	// A non-string size must ERROR, not default. This test previously asserted it
	// silently became dev — the smallest volume — which is precisely the
	// under-provisioning T3.4c exists to close: `size: 42` reaching the driver
	// means the API's string schema was bypassed, and answering that with the
	// cheapest PVC is a guess about a contract we cannot read.
	if _, err := d.Render(driver.Spec{Product: "postgres", Name: "x", Namespace: "ns", Shape: map[string]any{"size": 42}}); err == nil {
		t.Fatal("a non-string size must be refused, not silently sized as dev")
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
