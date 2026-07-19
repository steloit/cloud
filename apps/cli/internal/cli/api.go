package cli

// Shared API plumbing for the noun commands (T5.3): the authenticated
// generated client, the problem+json → three-line render, and the worn
// context echo. Commands render operations — behavior lives in the API.

import (
	"context"
	"fmt"
	"net/http"

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
// pass through; names resolve via listEnvironments on the context project;
// omitted = production forever (ADR-037).
func (inv *Invocation) resolveEnvID(c *contracts.ClientWithResponses) (string, int) {
	env := inv.Context.Env
	if len(env) > 4 && env[:4] == "env_" {
		return env, ExitOK
	}
	project, code := inv.needProject()
	if code != ExitOK {
		return "", code
	}
	name := env
	if name == "" {
		name = "production"
	}
	resp, err := c.ListEnvironmentsWithResponse(context.Background(), project)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return "", ExitGeneric
	}
	if resp.JSON200 == nil {
		return "", inv.fail(resp.Body, resp.HTTPResponse)
	}
	if resp.JSON200.Data != nil {
		for _, e := range *resp.JSON200.Data {
			if e.Name == name || e.Id == env {
				return e.Id, ExitOK
			}
		}
	}
	fmt.Fprintf(inv.Stderr, "✕ environment %q not found in project %s\n", name, project)
	return "", ExitNotFound
}

// echo prints the worn context before state-changing output (cli.md §1).
func (inv *Invocation) echo() {
	if !inv.Quiet() && !inv.JSON() {
		if e := inv.Context.Echo(); e != "" {
			fmt.Fprintln(inv.Stdout, e)
		}
	}
}
