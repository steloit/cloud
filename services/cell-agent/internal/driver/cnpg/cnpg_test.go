package cnpg

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/cell-agent/internal/driver"
	"gopkg.in/yaml.v3"
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
func TestEveryCatalogSizeRendersExactlyItsIncludedStorage(t *testing.T) {
	catalog := catalogSizes(t)

	// EQUALITY, not `>=`. An inequality binds only one direction, and two
	// mutations survived it: lowering `performance.included_gb` in the catalog
	// (green in BOTH modules, re-pricing every performance customer by 50c/GB),
	// and raising the driver's floor to 500 (green everywhere — an 11200c service
	// silently getting a 500Gi PVC). The same number lives in two files; the test
	// has to pin them together, not merely order them.
	for size, includedGB := range catalog {
		spec := devSpec()
		spec.Shape = map[string]any{"size": size}
		m, err := New().Render(spec)
		if err != nil {
			t.Fatalf("size %q is in the catalog and the driver refuses it: %v", size, err)
		}
		want := includedGB
		if want < minVolumeGB {
			want = minVolumeGB // dev includes 0 GB, and a 0Gi PVC is not a volume
		}
		if got := renderedStorageGB(t, m[0].YAML); got != want {
			t.Errorf("size %q includes %d GB and renders a %dGi PVC, want exactly %dGi. "+
				"Under-rendering bills for storage the volume does not have; over-rendering "+
				"provisions storage nobody sold.", size, includedGB, got, want)
		}
	}

	// The two maps must name the SAME sizes. A catalog size missing from the
	// driver is refused at converge; a driver size missing from the catalog is a
	// floor for something nobody can buy, and neither is visible from the loop
	// above.
	for size := range catalog {
		if _, ok := includedFloorGB[size]; !ok {
			t.Errorf("the catalog sells %q and includedFloorGB has no entry — every converge "+
				"for that size fails", size)
		}
	}
	for size := range includedFloorGB {
		if _, ok := catalog[size]; !ok {
			t.Errorf("includedFloorGB has %q, which the catalog does not sell", size)
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

var storageRe = regexp.MustCompile(`^(\d+)Gi$`)

// renderedStorageGB reads `spec.storage.size` BY PATH, not by taking the first
// `size:` in the document.
//
// The regex form was positional and correct only by accident of field order.
// Measured: adding a `spec.walStorage` block (a real CNPG field, and ordinary
// practice) above `spec.storage` and regressing `spec.storage.size` back to a
// literal `10Gi` left the ENTIRE module green — every size assertion silently
// measured the WAL volume while the data volume was the old constant. Goldens
// did not object either, because `-update` regenerates them.
func renderedStorageGB(t *testing.T, manifest []byte) int {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal(manifest, &doc); err != nil {
		t.Fatalf("parse the rendered manifest: %v\n%s", err, manifest)
	}
	v := dig(doc, "spec.storage.size")
	if v == nil {
		t.Fatalf("no spec.storage.size in the rendered manifest:\n%s", manifest)
	}
	m := storageRe.FindStringSubmatch(fmt.Sprint(v))
	if m == nil {
		t.Fatalf("spec.storage.size is %v — the driver must render whole Gi, and a unit change "+
			"would otherwise be read as a number change", v)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSnapshotBranchManifest(t *testing.T) {
	m, err := New().SnapshotBranch(driver.BranchSource{
		Name: "svc_db01", Namespace: "proj--prod", Cell: "cell-0", SnapshotName: "svc_db01-snap-1", Target: "svc_db01-branch",
		Shape: map[string]any{"size": "dev"},
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
		Shape: map[string]any{"size": "dev"},
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
		HasArchivedWAL: true, TargetTime: ts, Shape: map[string]any{"size": "dev"},
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
	base := driver.BranchSource{Name: "s", Namespace: "ns", Target: "t", HasArchivedWAL: true, Shape: map[string]any{},
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
	// Shape is supplied on BOTH so each case has exactly ONE reason to fail, and
	// the message is checked. Left nil, each had two, and neither assertion could
	// tell which fired — so the guard ORDER (branchStorage last) was silently
	// load-bearing for a test that never mentioned it.
	if _, err := d.SnapshotBranch(driver.BranchSource{
		Namespace: "ns", Target: "t", SnapshotName: "s", Shape: map[string]any{},
	}); err == nil {
		t.Fatal("snapshot branch with empty source name must error")
	} else if !strings.Contains(err.Error(), "name") {
		t.Fatalf("the refusal must name the missing field, got: %v", err)
	}
	if _, err := d.SnapshotBranch(driver.BranchSource{
		Name: "s", Namespace: "ns", Shape: map[string]any{},
	}); err == nil {
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

// A BRANCH IS SIZED FROM THE SOURCE — EVERY CATALOG SIZE, EVERY STORAGE SHAPE,
// BOTH ENTRY POINTS.
//
// The two dimensions are the point. A one-dimensional version of this (catalog
// size only, on the snapshot path only) is what let the first cut of this task
// ship a hole: "the priced storage reaches the branch" has TWO representations,
// snapshot and PITR, and a mutation stripping `storage_gb` from the PITR path
// alone stayed green — `{size: performance, storage_gb: 2000}` rendered
// create=2000Gi, snapshot=2000Gi, **pitr=50Gi**. That is the path with no
// restoreSize refusal in front of it, so the volume comes up and fills
// mid-restore. Same class as the branch it was fixed on, other representation.
//
// The storage dimension matters for the same reason: shapes of the form
// `{"size": X}` with no `storage_gb` are shapes the API never emits (per the
// founder decision of 2026-08-23 an unset storage_gb resolves to `included_gb`
// before the driver sees it), so a size-only sweep tests the one shape that does
// not occur and misses the ones that do.
//
// Everything is asserted against what `Render` produces for the SAME shape, by
// EQUALITY — the T3.4c rule: an inequality binds one direction only, and
// mutations survive it. Equality satisfies "at least as large as the source"
// a fortiori. Binding to Render rather than to a table of expected numbers is
// what stops create, branch and PITR drifting; a table would be a fourth copy of
// the catalog.
func TestEveryCatalogSizeAndStorageShapeBranchesAtTheSourceSize(t *testing.T) {
	entryPoints := []struct {
		name string
		size func(t *testing.T, shape map[string]any) int
	}{
		{"snapshot branch", func(t *testing.T, shape map[string]any) int {
			m, err := New().SnapshotBranch(driver.BranchSource{
				Name: "svc_db01", Namespace: "proj--prod", Cell: "cell-0",
				SnapshotName: "svc_db01-snap-1", Target: "svc_db01-branch", Shape: shape,
			})
			if err != nil {
				t.Fatalf("snapshot branch: %v", err)
			}
			return renderedStorageGB(t, m[1].YAML) // m[0] is the VolumeSnapshot
		}},
		{"PITR restore", func(t *testing.T, shape map[string]any) int {
			m, err := New().PITRBranch(driver.BranchSource{
				Name: "svc_db01", Namespace: "proj--prod", Cell: "cell-0", Target: "svc_db01-pitr",
				WALBucket: "b", GSAEmail: "g@x", HasArchivedWAL: true,
				TargetTime: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC), Shape: shape,
			})
			if err != nil {
				t.Fatalf("PITR: %v", err)
			}
			return renderedStorageGB(t, m[0].YAML)
		}},
	}

	checked := 0
	for size, includedGB := range catalogSizes(t) {
		// storage_gb as JSON delivers it (float64), across the interesting
		// relations to the size's included floor.
		variants := map[string]any{
			"unset":              nil,
			"zero":               float64(0),
			"exactly included":   float64(includedGB),
			"well above":         float64(includedGB + 350),
			"one below included": float64(includedGB - 1),
		}
		if includedGB == 0 {
			delete(variants, "one below included") // -1 GB is not a shape
		}
		for label, storageGB := range variants {
			shape := map[string]any{"size": size}
			if storageGB != nil {
				shape["storage_gb"] = storageGB
			}
			spec := devSpec()
			spec.Shape = shape
			created, err := New().Render(spec)
			if err != nil {
				t.Fatalf("size %q / %s: Render: %v", size, label, err)
			}
			want := renderedStorageGB(t, created[0].YAML)

			for _, ep := range entryPoints {
				checked++
				if got := ep.size(t, shape); got != want {
					t.Errorf("size %q, storage_gb %s: the source volume is %dGi and the %s asks "+
						"for %dGi.\n"+
						"  snapshot: external-provisioner refuses a request below the snapshot's "+
						"restoreSize outright (#727, closed NOT PLANNED) — the branch does not come "+
						"up at all.\n"+
						"  PITR: nothing refuses it. The volume is created and the base backup + "+
						"WAL replay fills it, mid-restore.",
						size, label, want, ep.name, got)
				}
			}
		}
	}
	// Both entry points, every catalog size, every storage relation. Asserted as
	// the product rather than as a floor: a floor cannot see a dimension that
	// silently stopped being swept.
	if want := len(catalogSizes(t))*len(entryPoints)*5 - len(entryPoints); checked != want {
		t.Fatalf("swept %d combinations, want %d (sizes x %d entry points x 5 storage variants, "+
			"less the one `dev` cannot have)", checked, want, len(entryPoints))
	}
}

// A MISSING SHAPE IS REFUSED, NOT DEFAULTED.
//
// storageForShape reads an absent size as `dev` (the API's closed schema defaults
// it), which is correct for a create and would silently re-render exactly the
// 10Gi this task exists to remove. So nil — "the caller never plumbed it" — has
// to be an error, while an empty-but-present shape stays legal (a dev postgres
// names no size).
func TestABranchWithNoSourceShapeIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(driver.BranchSource) (driver.Manifests, error)
		base driver.BranchSource
	}{
		{"snapshot", New().SnapshotBranch, driver.BranchSource{
			Name: "s", Namespace: "ns", SnapshotName: "snap", Target: "t"}},
		{"pitr", New().PITRBranch, driver.BranchSource{
			Name: "s", Namespace: "ns", Target: "t", WALBucket: "b", GSAEmail: "g@x",
			HasArchivedWAL: true, TargetTime: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)}},
	} {
		if _, err := tc.call(tc.base); err == nil {
			t.Errorf("%s: a branch with no source Shape was rendered — it would ask for a "+
				"defaulted 10Gi volume", tc.name)
		} else if !strings.Contains(err.Error(), "Shape") {
			t.Errorf("%s: the refusal must name the missing field, got: %v", tc.name, err)
		}

		// An empty-but-present shape is a legitimate dev postgres.
		withEmpty := tc.base
		withEmpty.Shape = map[string]any{}
		if _, err := tc.call(withEmpty); err != nil {
			t.Errorf("%s: an empty shape is a dev postgres and must render: %v", tc.name, err)
		}

		// And an unknown size is loud, exactly as it is on the create path.
		withUnknown := tc.base
		withUnknown.Shape = map[string]any{"size": "enormous"}
		if _, err := tc.call(withUnknown); err == nil {
			t.Errorf("%s: an unknown size branched silently", tc.name)
		}
	}
}

// A SIZE DOWNGRADE RENDERS THE RETAINED VOLUME, NEVER THE SMALLER SIZE'S DEFAULT.
//
// STORAGE IS A RATCHET (founder, 2026-08-25): a PostgreSQL volume may grow and
// must never physically shrink, so a downgrade must not attempt to shrink the
// existing volume and the effective storage stays at the provisioned capacity.
//
// The control plane enforces that upstream — the PATCH merge carries the stored
// `storage_gb` forward, `estimates.Resolve` only ever RAISES a value below the
// size's included_gb, and an explicit reduction is refused. This is the DRIVER's
// half of the same rule: given the ratcheted shape it must render the retained
// size, because rendering the smaller size's default is precisely the shrink the
// CSI driver would reject — leaving the row outstanding forever with nothing
// written back.
//
// Measured: `{size: standard, storage_gb: 50}` -> 50Gi; after a downgrade
// `{size: dev, storage_gb: 50}` -> 50Gi; and `{size: dev, storage_gb: 0}` — what
// re-deriving from the smaller requested size would produce — is 10Gi, the
// shrink. The third case is why the second one is asserted.
func TestASizeDowngradeRendersTheRetainedVolume(t *testing.T) {
	renderGB := func(shape map[string]any) int {
		t.Helper()
		spec := devSpec()
		spec.Shape = shape
		ms, err := New().Render(spec)
		if err != nil {
			t.Fatalf("render %v: %v", shape, err)
		}
		return renderedStorageGB(t, ms[0].YAML)
	}

	// Every catalog size, downgraded to `dev` while holding that size's storage.
	for size, includedGB := range catalogSizes(t) {
		if size == "dev" {
			continue
		}
		before := renderGB(map[string]any{"size": size, "storage_gb": includedGB})
		after := renderGB(map[string]any{"size": "dev", "storage_gb": includedGB})
		// THE CONTROL, per size rather than once at the end. Re-deriving from
		// the smaller requested size IS the shrink this test exists to catch; if
		// for some catalog size it is not smaller (an included_gb at or below
		// minVolumeGB collapses both to 10), the two assertions below are
		// vacuous for that size and must say so rather than pass quietly.
		if rederived := renderGB(map[string]any{"size": "dev", "storage_gb": 0}); rederived >= before {
			t.Errorf("re-deriving from `dev` yields %dGi against %s's retained %dGi — for THIS size "+
				"the shrink is not expressible, so the assertions below prove nothing about it",
				rederived, size, before)
			continue
		}
		if after < before {
			t.Errorf("%s(%dGi) downgraded to dev renders %dGi — a PVC cannot shrink, so the CSI "+
				"driver refuses this and the service is stranded outstanding forever",
				size, before, after)
		}
		if after != before {
			t.Errorf("%s -> dev renders %dGi, want the retained %dGi: the effective storage is "+
				"the already-provisioned capacity, and the price the customer is charged is "+
				"derived from exactly that", size, after, before)
		}
	}

}

// The ruling's convergence clause is NOT tested here. `Driver.Render` is a pure
// function of its Spec — five renders of one shape are five identical calls and
// prove only determinism. The real property is about successive CONVERGES, and
// it is measured one layer up, against what reaches the API server:
// render.TestSuccessiveConvergesNeverAskForASmallerVolume.
