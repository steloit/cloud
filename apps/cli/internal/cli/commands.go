package cli

// T5.1 commands: version + init (repo link). Nouns land with T5.3; auth with
// T5.2. The registry only ever contains cli.md §6 grammar.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func init() {
	register("version", "", "print the CLI version", func(inv *Invocation) int {
		if inv.JSON() {
			fmt.Fprintf(inv.Stdout, "{\"version\":%q}\n", Version)
			return ExitOK
		}
		fmt.Fprintf(inv.Stdout, "steloit %s\n", Version)
		return ExitOK
	})

	register("init", "", "bind this repo to a project (writes .steloit)", runInit)
}

// runInit — GOV-002 §3.5: binds the repo to a project; writes the context
// the flags would carry. `steloit init --project prj_… [--org] [--env]`.
func runInit(inv *Invocation) int {
	project := inv.Flags["project"]
	if project == "" && len(inv.Args) > 0 {
		project = inv.Args[0]
	}
	if project == "" {
		fmt.Fprintln(inv.Stderr, "steloit: init needs a project — `steloit init --project <prj_id>`")
		return ExitUsage
	}
	link := RepoLink{Project: project, Org: inv.Flags["org"], Env: inv.Flags["env"]}
	raw, err := json.MarshalIndent(link, "", "  ")
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	path := ".steloit"
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	abs, _ := filepath.Abs(path)
	if inv.Quiet() {
		fmt.Fprintln(inv.Stdout, project)
		return ExitOK
	}
	env := link.Env
	if env == "" {
		env = "production" // omitted = production forever (ADR-037)
	}
	fmt.Fprintf(inv.Stdout, "✓ linked %s → %s (env %s)\n", abs, project, env)
	fmt.Fprintln(inv.Stdout, "  commands here now resolve context from this link; flags still override")
	return ExitOK
}
