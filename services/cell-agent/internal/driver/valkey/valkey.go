// Package valkey is the second-product proof for T3.4 — it implements the SAME
// driver.Driver interface as CNPG, UNCHANGED, without the branching extension.
// A cache is provision-on-add, idle-suspend (A5.1); it does not snapshot-branch
// or PITR, so it satisfies Driver but NOT BranchingDriver. That asymmetry is the
// point: the interface is reusable across products with different capabilities.
//
// This is deliberately minimal (T3.4 out-of-scope: valkey feature-completeness).
// It exists to hold the abstraction honest — if the interface only fit CNPG, it
// was a CNPG API, not a provisioner interface.
package valkey

import (
	"fmt"

	"github.com/steloit/cloud/services/cell-agent/internal/driver"
)

type Driver struct{}

func New() *Driver { return &Driver{} }

func (d *Driver) Product() string { return "valkey" }

// Render turns a cache service's desired state into its Deployment/Service
// manifests. Minimal by design — one StatefulSet-shaped object is enough to
// prove the interface renders a non-Postgres product.
func (d *Driver) Render(s driver.Spec) (driver.Manifests, error) {
	if s.Product != "valkey" {
		return nil, fmt.Errorf("valkey: not a valkey product: %q", s.Product)
	}
	if s.Name == "" || s.Namespace == "" {
		return nil, fmt.Errorf("valkey: name and namespace are required")
	}
	body := fmt.Sprintf(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: %s
  namespace: %s
  labels:
    steloit.dev/cell: %s
spec:
  replicas: 1
  serviceName: %s
  template:
    spec:
      containers:
        - name: valkey
          image: valkey/valkey:8-alpine
`, s.Name, s.Namespace, s.Cell, s.Name)
	return driver.Manifests{{Kind: "StatefulSet", Name: s.Name, YAML: []byte(body)}}, nil
}
