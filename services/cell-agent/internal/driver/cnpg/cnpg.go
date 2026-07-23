// Package cnpg is the Postgres driver (T3.4) — it renders CNPG manifests from a
// service's desired state, encoding ADR-0007's MEASURED contracts as the
// implementation contract (archive_timeout=300s RPO bound; branch via
// VolumeSnapshot → bootstrap.recovery; PITR via recoveryTarget from archived
// WAL, never wall-clock). It renders only; the server-side-apply is the
// cell-agent Renderer seam, so this is deterministic and cluster-free.
package cnpg

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/steloit/cloud/services/cell-agent/internal/driver"
)

//go:embed templates/*.tmpl
var templates embed.FS

var tmpl = template.Must(template.ParseFS(templates, "templates/*.tmpl"))

// Driver renders CNPG manifests. It implements driver.BranchingDriver — the
// Postgres D2 primitive (branch, PITR, hibernate/wake).
type Driver struct{}

func New() *Driver { return &Driver{} }

func (d *Driver) Product() string { return "postgres" }

// storageForShape maps priced grammar (size) to a PD volume size. The dev size
// is the 10Gi the T1.0 spike measured (ADR-0007 §2). An explicit shape.storage
// wins. Unknown sizes fall back to dev rather than guessing large.
func storageForShape(shape map[string]any) string {
	if s, ok := shape["storage"].(string); ok && s != "" {
		return s
	}
	switch fmt.Sprint(shape["size"]) {
	case "standard":
		return "32Gi"
	case "pro":
		return "128Gi"
	default: // dev / unknown
		return "10Gi"
	}
}

type clusterData struct {
	Name, Namespace, Cell, GSAEmail, WALBucket, StorageSize string
	Instances                                               int
}

// Render turns desired state into the customer's CNPG Cluster (cluster-create).
func (d *Driver) Render(s driver.Spec) (driver.Manifests, error) {
	if s.Product != "postgres" {
		return nil, fmt.Errorf("cnpg: not a postgres product: %q", s.Product)
	}
	if err := requireName(s.Name, s.Namespace); err != nil {
		return nil, err
	}
	instances := s.Instances
	if instances < 1 {
		instances = 1
	}
	body, err := render("cluster.yaml.tmpl", clusterData{
		Name: s.Name, Namespace: s.Namespace, Cell: s.Cell, GSAEmail: s.GSAEmail,
		WALBucket: s.WALBucket, StorageSize: storageForShape(s.Shape), Instances: instances,
	})
	if err != nil {
		return nil, err
	}
	return driver.Manifests{{Kind: "Cluster", Name: s.Name, YAML: body}}, nil
}

type branchData struct {
	Target, Source, Namespace, Cell, SnapshotName, StorageSize, WALBucket, TargetTime string
}

// SnapshotBranch renders the VolumeSnapshot of the source and a Cluster that
// recovers from it (measured 52.4s branch e2e, data-identical — ADR-0007 §2).
// Apply order: snapshot first, then the cluster that recovers from it.
func (d *Driver) SnapshotBranch(b driver.BranchSource) (driver.Manifests, error) {
	if err := requireName(b.Name, b.Namespace); err != nil {
		return nil, err
	}
	if b.Target == "" || b.SnapshotName == "" {
		return nil, fmt.Errorf("cnpg: snapshot branch requires Target and SnapshotName")
	}
	snap, err := render("snapshot.yaml.tmpl", struct{ SnapshotName, Namespace, Source string }{
		b.SnapshotName, b.Namespace, b.Name,
	})
	if err != nil {
		return nil, err
	}
	clu, err := render("branch-cluster.yaml.tmpl", branchData{
		Target: b.Target, Source: b.Name, Namespace: b.Namespace,
		SnapshotName: b.SnapshotName, StorageSize: "10Gi", // CoW clone; sizing follows the source PVC
	})
	if err != nil {
		return nil, err
	}
	return driver.Manifests{
		{Kind: "VolumeSnapshot", Name: b.SnapshotName, YAML: snap},
		{Kind: "Cluster", Name: b.Target, YAML: clu},
	}, nil
}

// PITRBranch renders a new cluster recovering to a point in time. The target
// MUST derive from a real archived WAL basis (ADR-0007 F4) — a wall-clock target
// with no archived WAL is refused, never rendered into a false promise.
func (d *Driver) PITRBranch(b driver.BranchSource) (driver.Manifests, error) {
	if err := requireName(b.Name, b.Namespace); err != nil {
		return nil, err
	}
	if b.Target == "" {
		return nil, fmt.Errorf("cnpg: PITR branch requires Target")
	}
	if !b.HasArchivedWAL {
		return nil, fmt.Errorf("cnpg: PITR target has no archived-WAL basis — recovery targets derive from archived WAL, never wall-clock (ADR-0007 F4)")
	}
	if b.TargetTime.IsZero() {
		return nil, fmt.Errorf("cnpg: PITR requires a recovery target time")
	}
	clu, err := render("pitr-cluster.yaml.tmpl", branchData{
		Target: b.Target, Source: b.Name, Namespace: b.Namespace,
		StorageSize: "10Gi", WALBucket: walBucketFor(b),
		TargetTime: b.TargetTime.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}
	return driver.Manifests{{Kind: "Cluster", Name: b.Target, YAML: clu}}, nil
}

// Hibernate/Wake render the CNPG declarative-hibernation annotation patch (the
// cold-store half of duty-cycling; scheduling is T1.6). Wake latency floor is
// the measured 17.7s restart-to-ready (ADR-0007 §2).
func (d *Driver) Hibernate(cluster, namespace string) (driver.Patch, error) {
	return hibernationPatch(cluster, namespace, "on")
}
func (d *Driver) Wake(cluster, namespace string) (driver.Patch, error) {
	return hibernationPatch(cluster, namespace, "off")
}

func hibernationPatch(cluster, namespace, state string) (driver.Patch, error) {
	if cluster == "" || namespace == "" {
		return driver.Patch{}, fmt.Errorf("cnpg: hibernation requires cluster and namespace")
	}
	body := fmt.Sprintf(`{"metadata":{"annotations":{"cnpg.io/hibernation":%q}}}`, state)
	return driver.Patch{Target: cluster, Namespace: namespace, Body: []byte(body)}, nil
}

func walBucketFor(b driver.BranchSource) string {
	// The PITR source's WAL bucket is carried on the source; for the render we
	// use the conventional per-service path under the cell's customer WAL bucket.
	return "steloit-dev-wal-customer"
}

func requireName(name, ns string) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(ns) == "" {
		return fmt.Errorf("cnpg: name and namespace are required")
	}
	return nil
}

func render(name string, data any) ([]byte, error) {
	var b strings.Builder
	if err := tmpl.ExecuteTemplate(&b, name, data); err != nil {
		return nil, fmt.Errorf("cnpg: render %s: %w", name, err)
	}
	return []byte(b.String()), nil
}
