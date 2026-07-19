package cli

// T5.3: the alpha nouns — project · env · db · bind · deploy · token ·
// events · logs (+ dev stub). Every command is a rendering of an operation
// in openapi.yaml through the generated client and the shared output layer.
// The safety grammar (cli.md §3) is enforced here: estimate-first creates,
// --confirm <exact-name> for destruction, reveal-once for secrets.

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/steloit/cloud/apps/cli/internal/output"
	contracts "github.com/steloit/cloud/packages/contracts/go"
)

func init() {
	register("project", "create", "create a project (production env born with it)", runProjectCreate)
	register("project", "list", "projects in the org", runProjectList)
	register("project", "inspect", "one project", runProjectInspect)
	register("env", "create", "new environment in the project", runEnvCreate)
	register("env", "list", "environments with regions", runEnvList)
	register("env", "delete", "tear down an environment (--confirm <exact-name>)", runEnvDelete)
	register("db", "create", "PostgreSQL instance — estimate shown first", runDBCreate)
	register("db", "list", "databases in the env", func(inv *Invocation) int { return runServiceList(inv, "postgres") })
	register("db", "inspect", "one database", runServiceInspect)
	register("db", "destroy", "delete a database (--confirm <exact-name>)", runServiceDestroy)
	register("bind", "", "bind <source> <target> — credentials minted, $0", runBind)
	register("deploy", "", "deploy a service (--service, --sha)", runDeployCreate)
	register("deploy", "list", "immutable deployment history", runDeployList)
	register("deploy", "rollback", "redeploy the previous image", runDeployRollback)
	register("token", "create", "personal token — shown exactly once", runTokenCreate)
	register("token", "list", "your tokens (prefix + metadata only)", runTokenList)
	register("token", "revoke", "revoke a token by id", runTokenRevoke)
	register("key", "create", "org API key — explicit --permissions (least-privilege), shown once", runKeyCreate)
	register("key", "list", "org API keys (prefix + granted permissions)", runKeyList)
	register("key", "revoke", "revoke an org API key by id", runKeyRevoke)
	register("events", "", "the env's event spine (deploy markers included)", runEvents)
	register("logs", "", "query logs (shared query grammar)", runLogs)
	register("dev", "", "local dev with injected config (arrives with E4)", runDevStub)
	register("usage", "export", "the org's monthly meter report (B2)", runUsageExport)
}

func runUsageExport(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	org, code := inv.needOrg()
	if code != ExitOK {
		return code
	}
	params := &contracts.GetUsageParams{}
	if m := inv.Flags["month"]; m != "" {
		params.Month = &m
	}
	resp, err := c.GetUsageWithResponse(context.Background(), org, params)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON200 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	rep := resp.JSON200
	month := ""
	if rep.Month != nil {
		month = *rep.Month
	}
	fmt.Fprintf(inv.Stdout, "usage · %s\n", month)
	tab := output.NewTable("METER", "USED", "OVERAGE")
	if rep.Meters != nil {
		for _, m := range *rep.Meters {
			name, used, over := "", "0", "$0"
			if m.Meter != nil {
				name = *m.Meter
			}
			if m.Used != nil {
				used = strconv.FormatFloat(float64(*m.Used), 'f', -1, 32)
			}
			if m.OverageCents != nil {
				over = output.MoneyMonthly(int64(*m.OverageCents))
			}
			tab.Row(name, used, over)
		}
	}
	tab.Write(inv.Stdout)
	return ExitOK
}

// ---- project ----------------------------------------------------------------

func runProjectCreate(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	org, code := inv.needOrg()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit project create <name> [--region aws/…]")
		return ExitUsage
	}
	body := contracts.CreateProjectJSONRequestBody{Name: inv.Args[0]}
	if r := inv.Flags["region"]; r != "" {
		body.Region = &r
	}
	resp, err := c.CreateProjectWithResponse(context.Background(), org, body)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON201 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	p := resp.JSON201
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, p.Id)
		return ExitOK
	}
	fmt.Fprintf(inv.Stdout, "✓ %s created · %s · production environment born with it (ADR-037)\n", p.Name, p.Id)
	return ExitOK
}

func runProjectList(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	org, code := inv.needOrg()
	if code != ExitOK {
		return code
	}
	resp, err := c.ListProjectsWithResponse(context.Background(), org)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON200 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	tab := output.NewTable("ID", "NAME", "ENVS", "COST")
	var ids []string
	if resp.JSON200.Data != nil {
		for _, p := range *resp.JSON200.Data {
			envs, cost := 0, 0
			if p.EnvCount != nil {
				envs = *p.EnvCount
			}
			if p.MonthlyCostCents != nil {
				cost = *p.MonthlyCostCents
			}
			tab.Row(p.Id, p.Name, strconv.Itoa(envs), output.MoneyMonthly(int64(cost)))
			ids = append(ids, p.Id)
		}
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, ids...)
		return ExitOK
	}
	tab.Write(inv.Stdout)
	return ExitOK
}

func runProjectInspect(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	id := firstArgOr(inv, inv.Context.Project)
	if id == "" {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit project inspect <prj_id>")
		return ExitUsage
	}
	resp, err := c.GetProjectWithResponse(context.Background(), id)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON200 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	output.RawJSON(inv.Stdout, resp.Body) // inspect = the API shape, verbatim
	return ExitOK
}

// ---- env --------------------------------------------------------------------

func runEnvCreate(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	project, code := inv.needProject()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit env create <name> [--region aws/…]")
		return ExitUsage
	}
	body := contracts.CreateEnvironmentJSONRequestBody{Name: inv.Args[0]}
	if r := inv.Flags["region"]; r != "" {
		body.Region = &r
	}
	resp, err := c.CreateEnvironmentWithResponse(context.Background(), project, body)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON201 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	e := resp.JSON201
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, e.Id)
		return ExitOK
	}
	fmt.Fprintf(inv.Stdout, "✓ %s created · %s · region %s\n", e.Name, e.Id, renderRegion(e.Region))
	return ExitOK
}

func runEnvList(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	project, code := inv.needProject()
	if code != ExitOK {
		return code
	}
	resp, err := c.ListEnvironmentsWithResponse(context.Background(), project)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON200 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	tab := output.NewTable("ID", "NAME", "REGION")
	var ids []string
	if resp.JSON200.Data != nil {
		for _, e := range *resp.JSON200.Data {
			tab.Row(e.Id, e.Name, renderRegion(e.Region))
			ids = append(ids, e.Id)
		}
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, ids...)
		return ExitOK
	}
	tab.Write(inv.Stdout)
	return ExitOK
}

// runEnvDelete — U6 grammar: blast radius via the API's named blockers;
// --confirm <exact-name>; the implicit env refuses with the project-deletion
// remediation (rendered from the API's 409).
func runEnvDelete(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit env delete <name> --confirm <exact-name>")
		return ExitUsage
	}
	name := inv.Args[0]
	if inv.Flags["confirm"] != name {
		fmt.Fprintf(inv.Stderr, "✕ tearing down %s deletes its services' data · final backups are taken first (restorable 30 d)\n", name)
		fmt.Fprintf(inv.Stderr, "  → re-run with --confirm %s (the exact name; there is no --force)\n", name)
		return ExitUsage
	}
	project, code := inv.needProject()
	if code != ExitOK {
		return code
	}
	listResp, err := c.ListEnvironmentsWithResponse(context.Background(), project)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if listResp.JSON200 == nil {
		return inv.fail(listResp.Body, listResp.HTTPResponse)
	}
	envID := ""
	if listResp.JSON200.Data != nil {
		for _, e := range *listResp.JSON200.Data {
			if e.Name == name || e.Id == name {
				envID = e.Id
			}
		}
	}
	if envID == "" {
		fmt.Fprintf(inv.Stderr, "✕ environment %q not found in project %s\n", name, project)
		return ExitNotFound
	}
	delResp, err := c.DeleteEnvironmentWithResponse(context.Background(), envID)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if delResp.HTTPResponse == nil || delResp.HTTPResponse.StatusCode != 202 {
		var resp0 = delResp.HTTPResponse
		return inv.fail(delResp.Body, resp0)
	}
	fmt.Fprintf(inv.Stdout, "· %s tearing down · final backups recorded (restorable 30 d)\n", name)
	return ExitOK
}

// renderRegion: display `aws · ap-south-1` from flag value `aws/ap-south-1`.
func renderRegion(v string) string { return strings.Replace(v, "/", " · ", 1) }

// ---- db (the estimate-first create, cli.md §3) ------------------------------

func runDBCreate(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit db create <name> [--size dev|standard|performance] [--storage <GB>] [--ha]")
		return ExitUsage
	}
	envID, code := inv.resolveEnvID(c, true)
	if code != ExitOK {
		return code
	}
	name := inv.Args[0]
	shape := map[string]any{"size": flagOr(inv, "size", "dev")}
	if st := inv.Flags["storage"]; st != "" {
		n, err := strconv.Atoi(st)
		if err != nil {
			fmt.Fprintln(inv.Stderr, "✕ --storage takes gigabytes, e.g. --storage 10")
			return ExitUsage
		}
		shape["storage_gb"] = n
	}
	if inv.Flags["ha"] == "true" {
		shape["ha"] = true
	}

	// 1) estimate — ALWAYS shown; --yes accepts a SHOWN estimate, nothing skips seeing it
	product := contracts.Postgres
	estBody := contracts.CreateEstimateJSONRequestBody{
		Env:      &envID,
		Services: &[]contracts.ServiceShapeInput{{Product: product, Name: &name, Shape: &shape}},
	}
	estResp, err := c.CreateEstimateWithResponse(context.Background(), &contracts.CreateEstimateParams{}, estBody)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if estResp.JSON200 == nil {
		return inv.fail(estResp.Body, estResp.HTTPResponse)
	}
	est := estResp.JSON200
	inv.echo()
	if !inv.Quiet() && !inv.JSON() {
		for _, l := range est.Lines {
			lineName, cents := "", 0
			if l.Name != nil {
				lineName = *l.Name
			}
			if l.MonthlyCents != nil {
				cents = *l.MonthlyCents
			}
			fmt.Fprintf(inv.Stdout, "  %-14s %s\n", lineName, output.MoneyMonthly(int64(cents)))
		}
		fmt.Fprintf(inv.Stdout, "  %-14s %s · billing starts when the instance is ready — not now\n", "total", output.MoneyMonthly(int64(est.MonthlyTotalCents)))
	}
	if inv.Flags["yes"] != "true" {
		fmt.Fprintf(inv.Stderr, "Create %s at %s? [y/N] ", name, output.MoneyMonthly(int64(est.MonthlyTotalCents)))
		sc := bufio.NewScanner(stdinReader)
		if !sc.Scan() || !strings.EqualFold(strings.TrimSpace(sc.Text()), "y") {
			fmt.Fprintln(inv.Stderr, "· nothing created")
			return ExitOK
		}
	}

	// 2) create with the accepted estimate — the API enforces the gate
	svcBody := contracts.CreateServiceJSONRequestBody{
		Name: name, Product: product, EstimateId: est.Id, Shape: &shape,
	}
	svcResp, err := c.CreateServiceWithResponse(context.Background(), envID, &contracts.CreateServiceParams{}, svcBody)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if svcResp.JSON201 == nil {
		return inv.fail(svcResp.Body, svcResp.HTTPResponse)
	}
	svc := svcResp.JSON201
	if inv.JSON() {
		output.RawJSON(inv.Stdout, svcResp.Body)
		return ExitOK
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, svc.Id)
		return ExitOK
	}
	fmt.Fprintf(inv.Stdout, "%s %s · %s\n", output.Mark(string(svc.Status)), svc.Name, svc.Id)
	return ExitOK
}

func runServiceList(inv *Invocation, product string) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	envID, code := inv.resolveEnvID(c, false)
	if code != ExitOK {
		return code
	}
	resp, err := c.ListServicesWithResponse(context.Background(), envID)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON200 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	tab := output.NewTable("ID", "NAME", "STATUS", "COST")
	var ids []string
	if resp.JSON200.Data != nil {
		for _, s := range *resp.JSON200.Data {
			if product != "" && string(s.Product) != product {
				continue
			}
			cost := 0
			if s.MonthlyEstimateCents != nil {
				cost = *s.MonthlyEstimateCents
			}
			tab.Row(s.Id, s.Name, output.Mark(string(s.Status)), output.MoneyMonthly(int64(cost)))
			ids = append(ids, s.Id)
		}
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, ids...)
		return ExitOK
	}
	inv.echo()
	tab.Write(inv.Stdout)
	return ExitOK
}

func runServiceInspect(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit db inspect <svc_id>")
		return ExitUsage
	}
	resp, err := c.GetServiceWithResponse(context.Background(), inv.Args[0])
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON200 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	output.RawJSON(inv.Stdout, resp.Body)
	return ExitOK
}

// runServiceDestroy — the §3 destructive grammar: blast radius stated, then
// --confirm <exact-name>. No --force exists.
func runServiceDestroy(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit db destroy <svc_id> --confirm <exact-name>")
		return ExitUsage
	}
	id := inv.Args[0]
	getResp, err := c.GetServiceWithResponse(context.Background(), id)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if getResp.JSON200 == nil {
		return inv.fail(getResp.Body, getResp.HTTPResponse)
	}
	name := getResp.JSON200.Name
	if inv.Flags["confirm"] != name {
		fmt.Fprintf(inv.Stderr, "✕ destroying %s deletes data · a final backup is taken first (restorable 30 d)\n", name)
		fmt.Fprintf(inv.Stderr, "  → re-run with --confirm %s (the exact name; there is no --force)\n", name)
		return ExitUsage
	}
	delResp, err := c.DeleteServiceWithResponse(context.Background(), id)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if delResp.HTTPResponse.StatusCode != 202 {
		return inv.fail(delResp.Body, delResp.HTTPResponse)
	}
	fmt.Fprintf(inv.Stdout, "· %s deleting · final backup will be recorded (restorable 30 d)\n", name)
	return ExitOK
}

// ---- bind (verb-first) ------------------------------------------------------

func runBind(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) < 2 {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit bind <source-svc-id> <target-svc-id> [--scope read_only|read_write]")
		return ExitUsage
	}
	body := contracts.CreateBindingJSONRequestBody{Target: inv.Args[1]}
	if s := inv.Flags["scope"]; s != "" {
		scope := contracts.CreateBindingJSONBodyScope(s)
		body.Scope = &scope
	}
	resp, err := c.CreateBindingWithResponse(context.Background(), inv.Args[0], body)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON201 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	b := resp.JSON201
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, b.Id)
		return ExitOK
	}
	envVar := ""
	if b.EnvVars != nil {
		for k := range *b.EnvVars {
			envVar = k
		}
	}
	fmt.Fprintf(inv.Stdout, "✓ bound · %s injected · credentials minted · $0 · effective next deploy\n", envVar)
	return ExitOK
}

// ---- deploy -----------------------------------------------------------------

func runDeployCreate(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	envID, code := inv.resolveEnvID(c, true)
	if code != ExitOK {
		return code
	}
	service := inv.Flags["service"]
	if service == "" {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit deploy --service <svc_id> [--sha <git-sha>]")
		return ExitUsage
	}
	body := contracts.CreateDeploymentJSONRequestBody{Service: &service}
	if sha := inv.Flags["sha"]; sha != "" {
		body.GitSha = &sha
	}
	resp, err := c.CreateDeploymentWithResponse(context.Background(), envID, body)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON201 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	d := resp.JSON201
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, d.Id)
		return ExitOK
	}
	inv.echo()
	num := 0
	if d.Number != nil {
		num = *d.Number
	}
	fmt.Fprintf(inv.Stdout, "◌ deployment #%d %s · %s\n", num, string(d.State), d.Id)
	return ExitOK
}

func runDeployList(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	envID, code := inv.resolveEnvID(c, false)
	if code != ExitOK {
		return code
	}
	resp, err := c.ListDeploymentsWithResponse(context.Background(), envID)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON200 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	tab := output.NewTable("#", "ID", "SHA", "STATE", "ACTOR")
	var ids []string
	if resp.JSON200.Data != nil {
		for _, d := range *resp.JSON200.Data {
			num, sha := "", ""
			if d.Number != nil {
				num = strconv.Itoa(*d.Number)
			}
			if d.GitSha != nil {
				sha = *d.GitSha
			}
			tab.Row(num, d.Id, sha, string(d.State), d.Actor)
			ids = append(ids, d.Id)
		}
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, ids...)
		return ExitOK
	}
	tab.Write(inv.Stdout)
	return ExitOK
}

func runDeployRollback(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit deploy rollback <dep_id>")
		return ExitUsage
	}
	resp, err := c.RollbackDeploymentWithResponse(context.Background(), inv.Args[0])
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON201 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	d := resp.JSON201
	sha := ""
	if d.GitSha != nil {
		sha = *d.GitSha
	}
	fmt.Fprintf(inv.Stdout, "✓ rollback created · redeploys %s · migrations don't auto-revert\n", sha)
	return ExitOK
}

// ---- token (reveal-once, U7) ------------------------------------------------

func runTokenCreate(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit token create <name> [--scope read_only|full]")
		return ExitUsage
	}
	body := contracts.CreatePersonalTokenJSONRequestBody{Name: inv.Args[0]}
	if s := inv.Flags["scope"]; s != "" {
		scope := contracts.TokenInputScope(s)
		body.Scope = &scope
	}
	resp, err := c.CreatePersonalTokenWithResponse(context.Background(), body)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON201 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	t := resp.JSON201
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body) // --json is the same reveal, once
		return ExitOK
	}
	fmt.Fprintf(inv.Stdout, "%s\n", t.Token)
	fmt.Fprintln(inv.Stderr, "shown once — we store only a hash; copy it now")
	return ExitOK
}

func runTokenList(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	resp, err := c.ListPersonalTokensWithResponse(context.Background())
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON200 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	tab := output.NewTable("ID", "NAME", "PREFIX", "SCOPE")
	var ids []string
	if resp.JSON200.Data != nil {
		for _, t := range *resp.JSON200.Data {
			tab.Row(t.Id, t.Name, t.Prefix, string(t.Scope))
			ids = append(ids, t.Id)
		}
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, ids...)
		return ExitOK
	}
	tab.Write(inv.Stdout)
	return ExitOK
}

func runTokenRevoke(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "✕ usage: steloit token revoke <tok_id>")
		return ExitUsage
	}
	resp, err := c.RevokePersonalTokenWithResponse(context.Background(), inv.Args[0])
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.HTTPResponse.StatusCode != 204 {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	fmt.Fprintln(inv.Stdout, "✓ revoked — live requests with it stop now")
	return ExitOK
}

// ---- org API keys (G8 / ADR-0007) -------------------------------------------

func runKeyCreate(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	org, code := inv.needOrg()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "\u2715 usage: steloit key create <name> --permissions <p1,p2,\u2026> [--scope read_only|full]")
		return ExitUsage
	}
	permsFlag := inv.Flags["permissions"]
	if permsFlag == "" {
		fmt.Fprintln(inv.Stderr, "\u2715 an org key grants only the permissions you list \u2014 pass --permissions <matrix perms> (least-privilege, ADR-0007)")
		return ExitUsage
	}
	perms := strings.Split(permsFlag, ",")
	for i := range perms {
		perms[i] = strings.TrimSpace(perms[i])
	}
	body := contracts.CreateApiKeyJSONRequestBody{Name: inv.Args[0], Permissions: &perms}
	if s := inv.Flags["scope"]; s != "" {
		scope := contracts.TokenInputScope(s)
		body.Scope = &scope
	}
	resp, err := c.CreateApiKeyWithResponse(context.Background(), org, body)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON201 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	fmt.Fprintln(inv.Stdout, resp.JSON201.Token)
	fmt.Fprintln(inv.Stderr, "shown once \u2014 we store only a hash; copy it now")
	return ExitOK
}

func runKeyList(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	org, code := inv.needOrg()
	if code != ExitOK {
		return code
	}
	resp, err := c.ListApiKeysWithResponse(context.Background(), org)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON200 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	tab := output.NewTable("ID", "NAME", "PREFIX", "PERMISSIONS")
	var ids []string
	if resp.JSON200.Data != nil {
		for _, k := range *resp.JSON200.Data {
			perms := ""
			if k.Permissions != nil {
				perms = strings.Join(*k.Permissions, " ")
			}
			tab.Row(k.Id, k.Name, k.Prefix, perms)
			ids = append(ids, k.Id)
		}
	}
	if inv.Quiet() {
		output.Quiet(inv.Stdout, ids...)
		return ExitOK
	}
	tab.Write(inv.Stdout)
	return ExitOK
}

func runKeyRevoke(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	org, code := inv.needOrg()
	if code != ExitOK {
		return code
	}
	if len(inv.Args) == 0 {
		fmt.Fprintln(inv.Stderr, "\u2715 usage: steloit key revoke <key_id>")
		return ExitUsage
	}
	resp, err := c.RevokeApiKeyWithResponse(context.Background(), org, inv.Args[0])
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.HTTPResponse == nil || resp.HTTPResponse.StatusCode != 204 {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	fmt.Fprintln(inv.Stdout, "\u2713 revoked \u2014 live requests with it stop now")
	return ExitOK
}

// ---- events / logs ----------------------------------------------------------

func runEvents(inv *Invocation) int {
	c, code := inv.client()
	if code != ExitOK {
		return code
	}
	envID, code := inv.resolveEnvID(c, false)
	if code != ExitOK {
		return code
	}
	params := &contracts.ListEventsParams{}
	if k := inv.Flags["kind"]; k != "" {
		params.Kind = &k
	}
	resp, err := c.ListEventsWithResponse(context.Background(), envID, params)
	if err != nil {
		fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
		return ExitGeneric
	}
	if resp.JSON200 == nil {
		return inv.fail(resp.Body, resp.HTTPResponse)
	}
	if inv.JSON() {
		output.RawJSON(inv.Stdout, resp.Body)
		return ExitOK
	}
	tab := output.NewTable("AT", "KIND", "ACTION", "ACTOR")
	if resp.JSON200.Data != nil {
		for _, e := range *resp.JSON200.Data {
			action := ""
			if e.Action != nil {
				action = *e.Action
			}
			tab.Row(e.At.Format("15:04:05"), e.Kind, action, e.Actor)
		}
	}
	tab.Write(inv.Stdout)
	if inv.Flags["f"] == "true" {
		// live tail (T5.4): resume-capable SSE from the last listed point
		if err := inv.tailEvents(context.Background(), envID, inv.Flags["kind"], -1, func(f sseFrame) error {
			if inv.JSON() {
				fmt.Fprintln(inv.Stdout, f.Data)
				return nil
			}
			renderEventFrame(inv.Stdout, f)
			return nil
		}); err != nil {
			fmt.Fprintf(inv.Stderr, "steloit: %v\n", err)
			return ExitGeneric
		}
	}
	return ExitOK
}

// runLogs is a thin render of queryLogs — the backend lands with E6; until
// then the API's own answer (404) renders honestly. The grammar string is
// passed verbatim (shared with the console bar and alert rules).
func runLogs(inv *Invocation) int {
	fmt.Fprintln(inv.Stderr, "✕ logs need the observability pipeline (E6) — not available yet")
	fmt.Fprintln(inv.Stderr, "  → `steloit events` shows the env's lifecycle spine today")
	return ExitNotFound
}

// runDevStub — GOV-002 §3.5 names `steloit dev`; the machinery (local
// containers / tunneled services with injected config) rides E4's deploy
// work. Shown, not hidden — with the reason (cli.md §7).
func runDevStub(inv *Invocation) int {
	fmt.Fprintln(inv.Stderr, "✕ `steloit dev` arrives with the deploy epic (E4) — local runs with injected config")
	fmt.Fprintln(inv.Stderr, "  → until then: `steloit db list` + your own runner; bindings inject at deploy")
	return ExitNotFound
}

// ---- helpers ----------------------------------------------------------------

func firstArgOr(inv *Invocation, fallback string) string {
	if len(inv.Args) > 0 {
		return inv.Args[0]
	}
	return fallback
}

func flagOr(inv *Invocation, name, def string) string {
	if v := inv.Flags[name]; v != "" {
		return v
	}
	return def
}
