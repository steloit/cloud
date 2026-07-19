package cli

// Shared API plumbing for the noun commands (T5.3): the authenticated
// generated client, the problem+json → three-line render, and the worn
// context echo. Commands render operations — behavior lives in the API.

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/steloit/cloud/apps/cli/internal/output"
	contracts "github.com/steloit/cloud/packages/contracts/go"
)

// client returns the authenticated generated client or exits with the
// sign-in remediation.
func (inv *Invocation) client() (*contracts.ClientWithResponses, int) {
	if inv.Config.Token == "" {
		fmt.Fprintln(inv.Stderr, "✕ not connected — run `steloit auth login`")
		return nil, ExitPermission
	}
	c, err := apiClient(inv.Config, inv.Config.Token)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return nil, ExitGeneric
	}
	return c, ExitOK
}

// fail renders a non-2xx API response as the §4 three lines and returns the
// mapped exit code.
func (inv *Invocation) fail(body []byte, resp *http.Response) int {
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	return output.Problem(inv.Stderr, body, status)
}

// needProject / needEnv enforce worn context with a way forward.
func (inv *Invocation) needOrg() (string, int) {
	if inv.Context.Org == "" {
		fmt.Fprintln(inv.Stderr, "✕ no organization in context — pass --org, or set a profile default")
		return "", ExitUsage
	}
	return inv.Context.Org, ExitOK
}

func (inv *Invocation) needProject() (string, int) {
	if inv.Context.Project == "" {
		fmt.Fprintln(inv.Stderr, "✕ no project in context — pass --project, or run `steloit init` in the repo")
		return "", ExitUsage
	}
	return inv.Context.Project, ExitOK
}

// resolveEnvID turns the worn context into an env id: explicit env_… ids
// pass through; names resolve via listEnvironments on the context project.
// Implicit-env rules (ADR-037 / US-5.1): at n=1 the env is never asked for;
// at n≥2 READ commands default to production (the echo says so) while
// STATE-CHANGING commands never guess — TTY prompts with the env list,
// non-TTY exits 2 with `pass --env`.
func (inv *Invocation) resolveEnvID(c *contracts.ClientWithResponses, mutating bool) (string, int) {
	env := inv.Context.Env
	if len(env) > 4 && env[:4] == "env_" {
		return env, ExitOK
	}
	project, code := inv.needProject()
	if code != ExitOK {
		return "", code
	}
	resp, err := c.ListEnvironmentsWithResponse(context.Background(), project)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return "", ExitGeneric
	}
	if resp.JSON200 == nil {
		return "", inv.fail(resp.Body, resp.HTTPResponse)
	}
	var envs []struct{ ID, Name string }
	if resp.JSON200.Data != nil {
		for _, e := range *resp.JSON200.Data {
			envs = append(envs, struct{ ID, Name string }{e.Id, e.Name})
		}
	}
	if env != "" { // explicitly worn (flag, repo link, or profile)
		for _, e := range envs {
			if e.Name == env || e.ID == env {
				return e.ID, ExitOK
			}
		}
		fmt.Fprintf(inv.Stderr, "✕ environment %q not found in project %s\n", env, project)
		return "", ExitNotFound
	}
	// implicit: n=1 → the one env, never asked (ADR-037)
	if len(envs) == 1 {
		return envs[0].ID, ExitOK
	}
	if len(envs) == 0 {
		fmt.Fprintf(inv.Stderr, "✕ project %s has no environments\n", project)
		return "", ExitNotFound
	}
	// n≥2: reads default to production; mutations never guess
	if !mutating {
		for _, e := range envs {
			if e.Name == "production" {
				return e.ID, ExitOK
			}
		}
		return envs[0].ID, ExitOK
	}
	if isTTY() {
		fmt.Fprintln(inv.Stderr, "This project has several environments:")
		for i, e := range envs {
			fmt.Fprintf(inv.Stderr, "  %d) %s\n", i+1, e.Name)
		}
		fmt.Fprint(inv.Stderr, "Which environment? ")
		sc := bufio.NewScanner(stdinReader)
		if sc.Scan() {
			pick := strings.TrimSpace(sc.Text())
			for i, e := range envs {
				if pick == fmt.Sprint(i+1) || pick == e.Name {
					return e.ID, ExitOK
				}
			}
		}
		fmt.Fprintln(inv.Stderr, "✕ no environment chosen")
		return "", ExitUsage
	}
	names := make([]string, len(envs))
	for i, e := range envs {
		names[i] = e.Name
	}
	fmt.Fprintf(inv.Stderr, "✕ several environments exist (%s) and this command changes state\n", strings.Join(names, ", "))
	fmt.Fprintln(inv.Stderr, "  → pass --env <name> — state-changing commands never guess (ADR-037)")
	return "", ExitUsage
}

// isTTY reports whether stdin is an interactive terminal. The test seam
// (stdinReader) counts as interactive only when it is not os.Stdin-backed.
func isTTY() bool {
	if stdinReader != os.Stdin {
		return ttyForTests
	}
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// ttyForTests lets tests exercise the prompt path.
var ttyForTests = false

// echo prints the worn context before state-changing output (cli.md §1).
func (inv *Invocation) echo() {
	if !inv.Quiet() && !inv.JSON() {
		if e := inv.Context.Echo(); e != "" {
			fmt.Fprintln(inv.Stdout, e)
		}
	}
}
