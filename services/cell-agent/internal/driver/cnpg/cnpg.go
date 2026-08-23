// Package cnpg is the Postgres driver (T3.4) — it renders CNPG manifests from a
// service's desired state, encoding ADR-0007's MEASURED contracts as the
// implementation contract (archive_timeout=300s RPO bound; branch via
// VolumeSnapshot → bootstrap.recovery; PITR via recoveryTarget from archived
// WAL, never wall-clock). It renders only; the server-side-apply is the
// cell-agent Renderer seam, so this is deterministic and cluster-free.
package cnpg

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
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

// includedFloorGB is the storage each priced size includes, in GB. It MIRRORS
// docs' catalog (`services/api/internal/estimates/pricing.json` → postgres.sizes
// [*].included_gb) and must not drift from it: `TestEveryCatalogSizeRendersAt
// LeastItsIncludedStorage` reads that file and fails if a size is missing here
// or floored below what its base price includes. The cell-agent is a separate
// module and must not import the API's pricing table, so the binding is a test
// rather than a shared constant.
//
// `dev` includes 0 GB, but a 0Gi PVC is not a thing: minVolumeGB is the 10Gi the
// T1.0 spike measured (ADR-0007 §2).
var includedFloorGB = map[string]int{
	"dev":         0,
	"standard":    50,
	"performance": 50,
}

const minVolumeGB = 10

// storageForShape sizes the PVC from the PRICED storage_gb.
//
// It previously read `shape["storage"]`, a key that is not in the API's closed
// shape schema (`estimates.shapeSchema` allows `storage_gb`), so the API
// rejected it and the driver never saw it — the priced value was never read and
// a customer billed for 78 GB got the size-derived default. `pro` was dead code
// (no such size) and `performance`, the most expensive size, had no case at all
// and fell through to the smallest volume.
//
// The floor is belt-and-braces on top of the API resolving an unset storage_gb
// to included_gb: whatever arrives, a size never renders below what its own base
// price includes.
func storageForShape(shape map[string]any) (string, error) {
	// ABSENT is not UNKNOWN. The API's closed schema defaults size to "dev"
	// (estimates.shapeSchema), so a shape that never names one is contractually
	// dev — matching that is following the contract, not guessing. A size that
	// IS named and is not in the catalog is drift, and must be loud.
	size := "dev"
	if v, ok := shape["size"].(string); ok && v != "" {
		size = v
	} else if raw, present := shape["size"]; present && raw != nil {
		size = fmt.Sprint(raw)
	}
	floor, known := includedFloorGB[size]
	if !known {
		return "", fmt.Errorf("cnpg: unknown postgres size %q — add it to includedFloorGB "+
			"alongside the catalog entry; falling through to a default silently "+
			"under-provisions the size that was actually sold", size)
	}
	gb := floor
	if v, ok := asGB(shape["storage_gb"]); ok && v > gb {
		gb = v
	}
	if gb < minVolumeGB {
		gb = minVolumeGB
	}
	return fmt.Sprintf("%dGi", gb), nil
}

// asGB accepts the numeric shapes a JSON round-trip can produce. The desired doc
// reaches the agent as JSON, so an int written by the control plane arrives as a
// float64 — reading only int would silently ignore every storage_gb on the wire,
// which is the defect class this task exists to close.
func asGB(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	}
	return 0, false
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
	name := dnsName(s.Name)
	storageSize, err := storageForShape(s.Shape)
	if err != nil {
		return nil, err
	}
	cluster, err := render("cluster.yaml.tmpl", clusterData{
		Name: name, Namespace: s.Namespace, Cell: s.Cell, GSAEmail: s.GSAEmail,
		WALBucket: s.WALBucket, StorageSize: storageSize, Instances: instances,
	})
	if err != nil {
		return nil, err
	}
	// ADR-0007 F3: WAL archiving alone is NOT restorable — a recovery bootstrap
	// needs a base backup in the object store. The nightly ScheduledBackup with
	// immediate:true lands the first backup at apply time, so the cluster (and
	// any PITR branch off it) is restorable from day one, not eventually.
	backup, err := render("scheduled-backup.yaml.tmpl", clusterData{Name: name, Namespace: s.Namespace})
	if err != nil {
		return nil, err
	}
	return driver.Manifests{
		{Kind: "Cluster", Name: name, YAML: cluster},
		{Kind: "ScheduledBackup", Name: name + "-nightly", YAML: backup},
	}, nil
}

type branchData struct {
	Target, Source, Namespace, Cell, SnapshotName, StorageSize, WALBucket, GSAEmail, TargetTime string
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
		dnsName(b.SnapshotName), b.Namespace, dnsName(b.Name),
	})
	if err != nil {
		return nil, err
	}
	clu, err := render("branch-cluster.yaml.tmpl", branchData{
		Target: dnsName(b.Target), Source: dnsName(b.Name), Namespace: b.Namespace, Cell: b.Cell,
		SnapshotName: dnsName(b.SnapshotName), StorageSize: "10Gi", // CoW clone; sizing follows the source PVC
	})
	if err != nil {
		return nil, err
	}
	return driver.Manifests{
		{Kind: "VolumeSnapshot", Name: dnsName(b.SnapshotName), YAML: snap},
		{Kind: "Cluster", Name: dnsName(b.Target), YAML: clu},
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
	if b.WALBucket == "" || b.GSAEmail == "" {
		return nil, fmt.Errorf("cnpg: PITR requires the source WAL bucket and GSA — the recovering pod reads the source's archive via workload identity")
	}
	clu, err := render("pitr-cluster.yaml.tmpl", branchData{
		Target: dnsName(b.Target), Source: dnsName(b.Name), Namespace: b.Namespace, Cell: b.Cell,
		StorageSize: "10Gi", WALBucket: b.WALBucket, GSAEmail: b.GSAEmail,
		TargetTime: b.TargetTime.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}
	return driver.Manifests{{Kind: "Cluster", Name: dnsName(b.Target), YAML: clu}}, nil
}

// Hibernate/Wake render the CNPG declarative-hibernation annotation patch (the
// cold-store half of duty-cycling; scheduling is T1.6). Measured wake latency
// is 8.0s (ADR-0007 row 9, declarative hibernation off → accepting connections).
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

var dnsInvalid = regexp.MustCompile(`[^a-z0-9-]`)

// dnsName maps a service id (svc_<hex>, underscores) to an RFC1123 object name
// (k8s rejects underscores/uppercase). Deterministic and reversible enough for
// labels to carry the original id when that is needed.
func dnsName(id string) string {
	n := dnsInvalid.ReplaceAllString(strings.ToLower(id), "-")
	n = strings.Trim(n, "-")
	if n == "" {
		n = "svc"
	}
	return n
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
