// Command render-manifest prints the manifests the CNPG driver renders for a
// service, so a human (or a live-cell runbook) can apply exactly what the agent
// would apply and diff it against what the cluster accepts. Operational tool for
// the US-3.3 live evidence; it talks to nothing.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/steloit/cloud/services/cell-agent/internal/driver"
	"github.com/steloit/cloud/services/cell-agent/internal/driver/cnpg"
)

func main() {
	var s driver.Spec
	flag.StringVar(&s.Name, "name", "svc_demo", "service id")
	flag.StringVar(&s.Namespace, "namespace", "env-demo", "env namespace (env-<environment_id>, ADR-0012)")
	flag.StringVar(&s.Cell, "cell", "cell-dev", "cell id")
	flag.StringVar(&s.GSAEmail, "gsa", "", "workload-identity SA for the DB pod")
	flag.StringVar(&s.WALBucket, "wal-bucket", "", "customer WAL bucket")
	size := flag.String("size", "dev", "shape size")
	flag.Parse()

	if s.GSAEmail == "" || s.WALBucket == "" {
		// Without these the render is a valid-looking cluster with no backups and
		// an empty workload-identity annotation — a human following the runbook
		// would apply an un-backed-up database.
		fmt.Fprintln(os.Stderr, "render-manifest: -gsa and -wal-bucket are required")
		os.Exit(2)
	}
	s.Product, s.Intent, s.Instances = "postgres", "database", 1
	s.Shape = map[string]any{"size": *size}

	m, err := cnpg.New().Render(s)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, o := range m {
		fmt.Printf("---\n%s", o.YAML)
	}
}
