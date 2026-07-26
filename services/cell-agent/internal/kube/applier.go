// Package kube is the cell-agent's apply seam (US-3.3): the boundary between
// rendered manifests and a real Kubernetes API. The Renderer (internal/render)
// depends only on the Applier interface, so convergence is provable against a
// fake without a cluster; the real client (Phase B) plugs in behind the same
// interface. This is the "server-side-apply" half of the reconciler's converge
// (e1-substrate-design.md §2 step 2).
package kube

import "context"

// Applier server-side-applies rendered objects to a cell's Kubernetes API and
// observes the resulting resource status. Apply is idempotent (SSA semantics);
// Observe reads a CNPG Cluster's phase and returns it verbatim for the Renderer
// to map to the reconciler vocabulary.
type Applier interface {
	// Apply server-side-applies each object in `manifests` into `namespace`.
	// Idempotent: applying the same objects again is a no-op (SSA reconciles).
	Apply(ctx context.Context, namespace string, manifests [][]byte) error
	// Observe returns the CNPG Cluster's phase string (e.g. "Cluster in healthy
	// state") for `name` in `namespace`, or "" if it does not exist yet.
	Observe(ctx context.Context, namespace, name string) (phase string, err error)
	// Delete removes one object by KIND and name (teardown). Idempotent.
	// The kind is required: routing every delete to /clusters/ 404s for a
	// ScheduledBackup and silently orphans it.
	Delete(ctx context.Context, namespace, kind, name string) error
}
