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
	flag.StringVar(&s.Namespace, "namespace", "demo--prod", "env namespace (proj--env)")
	flag.StringVar(&s.Cell, "cell", "cell-dev", "cell id")
	flag.StringVar(&s.GSAEmail, "gsa", "", "workload-identity SA for the DB pod")
	flag.StringVar(&s.WALBucket, "wal-bucket", "", "customer WAL bucket")
	size := flag.String("size", "dev", "shape size")
	flag.Parse()

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
