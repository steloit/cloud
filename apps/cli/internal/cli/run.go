// Package cli is the steloit command surface: `steloit <noun> <verb>`
// routing with the context bar as flags (cli.md §1). Exit codes are the §4
// contract; output modes are the §2 contract (T5.5 owns the full layer).
package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Version is stamped by the release build (-ldflags "-X ...").
var Version = "dev"

// stdinReader is a seam so tests can feed the paste flow.
var stdinReader io.Reader = os.Stdin

// Exit codes — cli.md §4, verbatim.
const (
	ExitOK         = 0
	ExitGeneric    = 1
	ExitUsage      = 2
	ExitPermission = 3
	ExitNotFound   = 4
	ExitConflict   = 5
	ExitPayment    = 6
	ExitRateLimit  = 7
)

// Invocation carries everything a command needs.
type Invocation struct {
	Args    []string // positional args after noun+verb
	Flags   map[string]string
	Context Context // resolved org/project/env + source
	Config  *Config
	Stdout  io.Writer
	Stderr  io.Writer
}

// JSON / Quiet report the output mode (cli.md §2).
func (inv *Invocation) JSON() bool  { return inv.Flags["json"] == "true" }
func (inv *Invocation) Quiet() bool { return inv.Flags["quiet"] == "true" }

type command struct {
	noun, verb string // verb "" = noun-only command (init, version, deploy)
	summary    string
	run        func(inv *Invocation) int
}

var registry []command

// register wires a command at package init; nouns/verbs come from cli.md §6
// — the registry never invents grammar.
func register(noun, verb, summary string, run func(inv *Invocation) int) {
	registry = append(registry, command{noun: noun, verb: verb, summary: summary, run: run})
}

// Run parses `steloit <noun> [verb] [args] [flags]`, resolves context, and
// dispatches. Flags may appear anywhere (context flags travel with the crumb).
func Run(argv []string, stdout, stderr io.Writer) int {
	positional, flags, err := splitArgs(argv)
	if err != nil {
		fmt.Fprintf(stderr, "steloit: %v\n", err)
		return ExitUsage
	}
	if len(positional) == 0 || flags["help"] == "true" {
		usage(stdout)
		if len(positional) == 0 && flags["help"] != "true" {
			return ExitUsage
		}
		return ExitOK
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(stderr, "steloit: config: %v\n", err)
		return ExitGeneric
	}
	cctx := ResolveContext(flags, ".", cfg)

	noun := positional[0]
	var cmd *command
	var rest []string
	for i := range registry {
		c := &registry[i]
		if c.noun != noun {
			continue
		}
		if c.verb == "" && cmd == nil {
			cmd, rest = c, positional[1:]
		}
		if len(positional) > 1 && c.verb == positional[1] {
			cmd, rest = c, positional[2:]
			break
		}
	}
	if cmd == nil {
		fmt.Fprintf(stderr, "steloit: unknown command %q — run `steloit --help`\n", strings.Join(positional, " "))
		return ExitUsage
	}
	inv := &Invocation{Args: rest, Flags: flags, Context: cctx, Config: cfg, Stdout: stdout, Stderr: stderr}
	return cmd.run(inv)
}

// splitArgs separates positionals from --flags (--flag value | --flag=value;
// bare boolean flags: --json --quiet --yes --help -f).
func splitArgs(argv []string) ([]string, map[string]string, error) {
	boolFlags := map[string]bool{"json": true, "quiet": true, "yes": true, "help": true, "f": true, "compare": true, "ha": true}
	var pos []string
	flags := map[string]string{}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if !strings.HasPrefix(a, "-") {
			pos = append(pos, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			flags[name[:eq]] = name[eq+1:]
			continue
		}
		if boolFlags[name] {
			flags[name] = "true"
			continue
		}
		if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
			return nil, nil, fmt.Errorf("flag --%s needs a value", name)
		}
		flags[name] = argv[i+1]
		i++
	}
	return pos, flags, nil
}

func usage(w io.Writer) {
	fmt.Fprintf(w, "steloit %s — the Steloit CLI (thin client of /v1)\n\n", Version)
	fmt.Fprintln(w, "usage: steloit <noun> <verb> [args] [--project <p>] [--env <e>] [--org <o>] [flags]")
	fmt.Fprintln(w, "\ncommands:")
	sorted := make([]command, len(registry))
	copy(sorted, registry)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].noun != sorted[j].noun {
			return sorted[i].noun < sorted[j].noun
		}
		return sorted[i].verb < sorted[j].verb
	})
	for _, c := range sorted {
		name := c.noun
		if c.verb != "" {
			name += " " + c.verb
		}
		fmt.Fprintf(w, "  %-18s %s\n", name, c.summary)
	}
	fmt.Fprintln(w, "\nflags: --json (raw API shapes) · --quiet (ids only) · NO_COLOR honored")
}
