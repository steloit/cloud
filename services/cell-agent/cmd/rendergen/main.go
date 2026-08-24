// rendergen prints tenancy.Render's output for a namespace, so live verification
// applies EXACTLY what the agent would apply. Hand-written YAML in a runbook
// proves the runbook, not the renderer.
package main

import (
	"fmt"
	"os"

	"github.com/steloit/cloud/services/cell-agent/internal/driver/tenancy"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: rendergen <namespace> <cell> <apiserver-cidr>")
		os.Exit(2)
	}
	ms, err := tenancy.Render(tenancy.Spec{
		Namespace:     os.Args[1],
		Cell:          os.Args[2],
		APIServerCIDR: os.Args[3],
		Quota:         tenancy.Quota{CPU: "8", Memory: "16Gi", Storage: "100Gi"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}
	for _, m := range ms {
		fmt.Println("---")
		fmt.Print(string(m.YAML))
	}
}
