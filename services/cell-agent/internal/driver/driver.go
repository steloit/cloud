// Package driver is the provisioner interface (T3.4, INF-001 A4): a service's
// desired state renders to the k8s/substrate manifests that realise it. One
// interface, per-product drivers behind it — CNPG (postgres) is the pioneer;
// E9 products (valkey, …) reuse the SAME Driver interface unchanged.
//
// This is the RENDER half of the reconciler's converge (US-1.3 §2). The actual
// server-side-apply is the cell-agent's Renderer seam, a later live-integration
// task — nothing here talks to a cluster, so it is deterministic and testable
// against ADR-0007's measured manifest shapes without provisioning anything.
//
// Substrate names (CNPG, pd, barman) legitimately appear in a driver's OUTPUT:
// translating grammar (product + shape) into substrate IS the driver's job. D8
// forbids substrate names in CUSTOMER surfaces, not in this internal plane.
package driver

import "time"

// Spec is the rendering input: product grammar plus the placement the control
// plane resolved (namespace, cell, workload-identity SA, WAL bucket). Grammar
// only — no substrate detail leaks in.
type Spec struct {
	Name      string         // service id → cluster/object name
	Namespace string         // proj--env
	Product   string         // postgres | valkey | web | worker
	Intent    string         // app | database | cache | …
	Shape     map[string]any // size, etc. (priced grammar)
	Instances int            // desired replica count (>=1)
	Cell      string         // cell id (placement)
	GSAEmail  string         // workload-identity GCP service account
	WALBucket string         // per-cell customer WAL bucket
}

// Manifest is one rendered object. Kind+Name identify it; YAML is the applyable
// body. A driver returns them in apply order (e.g. snapshot before the cluster
// that recovers from it).
type Manifest struct {
	Kind string
	Name string
	YAML []byte
}

// Manifests is an ordered apply set.
type Manifests []Manifest

// Patch is an in-place mutation (e.g. the hibernation annotation) — a target and
// a strategic-merge/JSON body, not a full object.
type Patch struct {
	Target    string // object name
	Namespace string
	Body      []byte
}

// Driver is the product-agnostic interface every managed product implements and
// E9 reuses UNCHANGED (the T3.4 headline AC). Render turns desired state into
// the manifests to apply.
type Driver interface {
	Product() string
	Render(Spec) (Manifests, error)
}

// BranchSource identifies the origin of a branch (the D2 primitive).
type BranchSource struct {
	Name           string // source cluster
	Namespace      string
	SnapshotName   string    // for SnapshotBranch: the VolumeSnapshot to take/recover
	Target         string    // the new (branch) cluster name
	HasArchivedWAL bool      // PITR requires a real archived-WAL basis (ADR-0007 F4)
	TargetTime     time.Time // PITR recovery target (derived from WAL, never wall-clock)
}

// BranchingDriver is the Postgres-specific extension — the D2 branch primitive
// (snapshot-branch, PITR-to-new-branch, hibernate/wake). A non-branching product
// (valkey) implements Driver but NOT this, which is exactly how the interface
// proves reusable across products with different capabilities.
type BranchingDriver interface {
	Driver
	SnapshotBranch(BranchSource) (Manifests, error)
	PITRBranch(BranchSource) (Manifests, error)
	Hibernate(cluster, namespace string) (Patch, error)
	Wake(cluster, namespace string) (Patch, error)
}
