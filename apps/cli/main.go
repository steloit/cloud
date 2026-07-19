// steloit — the CLI (GOV-002 §3.5): a first-class product and a THIN client
// of the same /v1 API the console uses. No CLI-only capabilities, no
// console-only capabilities, ever. Commands render operations; behavior is
// owned by openapi.yaml.
package main

import (
	"os"

	"github.com/steloit/cloud/apps/cli/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
