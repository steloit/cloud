package identity_test

// CK-M3 — "estimate-gated provisioning end-to-end".
//
// This is the checkpoint's first exit criterion, proved as a chain rather than
// asserted: the REAL `steloit` binary, driven against a REAL API server (the
// same httpapi.Chain main.go builds) on REAL Postgres, through estimate →
// approval → create → the reconciler's poll/writeback → `ready` → metered.
//
// SCOPE, stated so the checkpoint does not read as more than it is: the ready
// edge is driven through `reconcile.Desired`/`Writeback` — the control plane's
// half of the protocol, exactly what a cell-agent's report goes through. It is
// NOT a live cell: no agent process, no CNPG cluster. US-3.3 proved the cell
// half against real GKE; this proves the CLI seam and that a CLI-created row is
// reachable by a cell. The two-process live drill remains carried forward.
//
// Every layer below the CLI is already covered by its own tests. What nothing
// covered before this is the SEAM: that the binary a customer runs actually
// walks the estimate gate the API enforces, and that a service created that way
// reaches ready and opens exactly one billing span. Each of those is true in
// isolation; the checkpoint exists because "true in isolation" has been wrong
// before (US-3.3 shipped four defects a live cell could not see).
//
// The second exit criterion — canon arithmetic green against the estimate
// engine — is proved in full by internal/estimates.TestCanonArithmetic (every
// canon service priced through the real engine, each line, the $208 Σ anchor,
// the four equations). This file additionally prices the shape it just created
// through that same engine and runs CheckArithmetic, so the link is asserted
// here rather than merely cited.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/canon"
	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/reconcile"
)

// buildCLI compiles the real binary. Building rather than importing is
// deliberate: apps/cli is a separate module, and a test that imported its
// packages would prove the library works, not that the SHIPPED binary does.
//
// Hermetic only by COINCIDENCE, recorded so the fragility is visible: apps/cli's
// dependencies are already in services/api/go.mod at identical versions, so the
// module cache CI warms for this module already holds them. If those versions
// ever diverge, this build starts needing the network mid-test with an opaque
// failure. It also means a compile break in apps/cli surfaces first as a
// services/api test failure.
func buildCLI(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "steloit")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(root, "apps", "cli")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the CLI failed: %v\n%s", err, out)
	}
	return bin
}

// runCLI executes the binary against the live test server.
func runCLI(t *testing.T, bin, apiURL, cfgPath string, args ...string) (int, string, string) {
	return runCLIStdin(t, bin, apiURL, cfgPath, "", args...)
}

// runCLIStdin is runCLI with an answer for the interactive prompt.
func runCLIStdin(t *testing.T, bin, apiURL, cfgPath, stdin string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	cmd.Env = append(os.Environ(),
		"STELOIT_API_URL="+apiURL,
		"STELOIT_CONFIG="+cfgPath,
		"NO_COLOR=1",
	)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running the CLI failed: %v", err)
	}
	return code, stdout.String(), stderr.String()
}

// canonDBReports returns the price canon records for its `db-reports` service —
// a postgres dev instance with 10 GB, the shape this checkpoint creates.
//
// DERIVED, never retyped: a literal 2400 here would make the engine assertion
// below compare the engine to this test rather than to canon, which is exactly
// what the second exit criterion is about.
//
// wantPrice mirrors output.Money's whole-dollar rule, so the string compared
// against the CLI's screen is the one canon implies rather than a guess.
func canonDBReports(t *testing.T) (int64, string) {
	t.Helper()
	cw, err := canon.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range cw.Services {
		if s.Name == "db-reports" {
			if s.MonthlyEstimateCent <= 0 {
				t.Fatalf("canon db-reports is %d — a zeroed price would make every assertion below vacuous", s.MonthlyEstimateCent)
			}
			return s.MonthlyEstimateCent, fmt.Sprintf("$%d/mo", s.MonthlyEstimateCent/100)
		}
	}
	t.Fatal("canon has no db-reports service — this checkpoint prices against it")
	return 0, ""
}

func TestCKM3EstimateGatedProvisioningEndToEnd(t *testing.T) {
	wantCents, wantPrice := canonDBReports(t)
	// A checkpoint that SKIPS reads as `ok` — `go test -run CKM3` exits 0 with
	// "no tests to run" or a silent skip, so a milestone gate can be green on a
	// machine that never executed it. STELOIT_CHECKPOINT=1 (what the verify
	// block sets) turns that into a failure.
	if os.Getenv("STELOIT_CHECKPOINT") != "" {
		t.Cleanup(func() {
			if t.Skipped() {
				panic("CK-M3 SKIPPED while STELOIT_CHECKPOINT is set: the checkpoint did not run, so it cannot be green")
			}
		})
	}
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	bin := buildCLI(t)

	// --- an org, a project, an env, and a CLI token -------------------------
	ownerCk, ownerID := w.signupUser(t, "ckm3@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"ckm3co"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	orgRow, err := w.svc.GetOrg(ctx, org.Id)
	if err != nil {
		t.Fatal(err)
	}
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}

	resp, body = w.post(t, "/v1/me/tokens", `{"name":"cli","scope":"full"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createToken: %d %s", resp.StatusCode, body)
	}
	var tok struct{ Token string }
	_ = json.Unmarshal([]byte(body), &tok)
	if !strings.HasPrefix(tok.Token, "stp_") {
		t.Fatalf("no CLI token minted: %s", body)
	}

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	cfg, _ := json.Marshal(map[string]string{"token": tok.Token, "api_url": w.srv.URL})
	if err := os.WriteFile(cfgPath, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	// --- ORDERING, proved by construction: decline the prompt ---------------
	//
	// A substring check on final stdout cannot see ordering — the estimate
	// could be printed after the service exists and still appear in the output.
	// Declining proves it: the price is on screen and NOTHING was created, so
	// the estimate necessarily preceded the create.
	code, out, errOut := runCLIStdin(t, bin, w.srv.URL, cfgPath, "n\n",
		"db", "create", "orders", "--env", env.ID, "--size", "dev", "--storage", "10")
	if code != 0 {
		t.Fatalf("declining must exit 0 (%d)\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	assertEstimateShown(t, out, "the declined run", wantPrice)
	declinedSvcs, err := w.prov.ListServices(ctx, env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(declinedSvcs) != 0 {
		t.Fatalf("declining created %d service(s) — the price must be shown BEFORE anything provisions", len(declinedSvcs))
	}

	// --- accept: the estimate is SHOWN, at the price actually charged -------
	code, out, errOut = runCLI(t, bin, w.srv.URL, cfgPath,
		"db", "create", "orders", "--env", env.ID, "--size", "dev", "--storage", "10", "--yes")
	if code != 0 {
		t.Fatalf("db create failed (%d)\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	assertEstimateShown(t, out, "the accepted run", wantPrice)
	if !strings.Contains(out, "billing starts when the instance is ready") {
		t.Fatalf("the CLI did not state when billing starts:\n%s", out)
	}

	// --- exactly one service, and NOTHING billed yet ------------------------
	svcs, err := w.prov.ListServices(ctx, env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 1 {
		t.Fatalf("expected exactly 1 service, got %d", len(svcs))
	}
	svc := svcs[0]
	if svc.Status != "provisioning" {
		t.Fatalf("a freshly created service must be provisioning, got %q", svc.Status)
	}
	// What was PRICED is what was provisioned. Without this, a CLI that drops
	// a flag produces a self-consistent wrong price: the API only checks the
	// create against the CLI's own estimate, so `--storage 10` silently lost
	// yields a $19 db the operator asked $24 for.
	if svc.Product != "postgres" {
		t.Fatalf("product is %q, want postgres", svc.Product)
	}
	var shape map[string]any
	if err := json.Unmarshal(svc.Shape, &shape); err != nil {
		t.Fatal(err)
	}
	if shape["size"] != "dev" {
		t.Fatalf("size is %v, want dev — the shape the operator asked for did not reach the estimate", shape["size"])
	}
	if gb, _ := shape["storage_gb"].(float64); int(gb) != 10 {
		t.Fatalf("storage_gb is %v, want 10 — a dropped flag prices and provisions something else", shape["storage_gb"])
	}
	if svc.MonthlyEstimateCents != wantCents {
		t.Fatalf("the service is priced at %d, the operator was shown %d", svc.MonthlyEstimateCents, wantCents)
	}
	if n := spanCount(t, w, svc.ID); n != 0 {
		t.Fatalf("provisioning opened %d billing span(s) — D10: metering starts at ready, never before", n)
	}

	// The estimate is one-shot: the ACCEPTED create burned exactly one, at the
	// price shown. The DECLINED run must have left its own estimate unburned —
	// saying no must not cost the operator anything.
	var accepted, declined int
	var acceptedCents int64
	if err := w.pool.QueryRow(ctx,
		`SELECT count(*), coalesce(max(total_cents), 0) FROM estimates
		 WHERE env_id = $1 AND accepted_at IS NOT NULL`, env.ID).Scan(&accepted, &acceptedCents); err != nil {
		t.Fatal(err)
	}
	if accepted != 1 {
		t.Fatalf("%d estimate(s) accepted, want exactly 1 — the gate did not burn the estimate it approved", accepted)
	}
	if acceptedCents != wantCents {
		t.Fatalf("the accepted estimate is %d, the operator was shown %d", acceptedCents, wantCents)
	}
	if err := w.pool.QueryRow(ctx,
		`SELECT count(*) FROM estimates WHERE env_id = $1 AND accepted_at IS NULL`, env.ID).Scan(&declined); err != nil {
		t.Fatal(err)
	}
	if declined != 1 {
		t.Fatalf("%d unaccepted estimate(s), want exactly 1 (the declined run) — declining must not burn an estimate", declined)
	}

	// --- reconcile to ready THROUGH THE RECONCILER PLANE --------------------
	//
	// Not prov.Transition: that would prove the status machine, not that a
	// CLI-created row is reachable by a cell at all. A service the cell can
	// never poll would leave this checkpoint green while no customer database
	// ever reaches ready.
	rec := reconcile.New(store.New(w.pool), w.prov)
	outstanding, err := rec.Desired(ctx, "cell-0", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var found *reconcile.DesiredService
	for i := range outstanding.Services {
		if outstanding.Services[i].ID == svc.ID {
			found = &outstanding.Services[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("the CLI-created service is not in the cell's outstanding work (%d others) — no cell would ever provision it", len(outstanding.Services))
	}
	if found.Product != "postgres" {
		t.Fatalf("the desired doc names product %q", found.Product)
	}
	// The doc the cell renders from must carry what the operator PAID for.
	// Product alone is not enough: a doc that lost its shape builds a default
	// cluster, so the customer is billed for dev+10GB and gets something else.
	// Nothing else in the suite pins the shape in the desired doc.
	var desired map[string]any
	if err := json.Unmarshal(found.Desired, &desired); err != nil {
		t.Fatal(err)
	}
	if ns, _ := desired["namespace"].(string); ns == "" {
		t.Fatal("the desired doc has no namespace — the cell would render into an unresolved namespace")
	}
	dshape, _ := desired["shape"].(map[string]any)
	if dshape == nil {
		t.Fatalf("the desired doc carries no shape: %v", desired)
	}
	if dshape["size"] != "dev" {
		t.Fatalf("desired shape size is %v, want dev — the cell would not build what was priced", dshape["size"])
	}
	if gb, _ := dshape["storage_gb"].(float64); int(gb) != 10 {
		t.Fatalf("desired shape storage_gb is %v, want 10", dshape["storage_gb"])
	}
	if _, err := rec.Writeback(ctx, "cell-0", reconcile.Report{
		ServiceID: svc.ID, ObservedGeneration: found.Generation, Status: "ready",
	}); err != nil {
		t.Fatalf("writeback to ready: %v", err)
	}
	if got := mustStatus(t, w, svc.ID); got != "ready" {
		t.Fatalf("after the cell reported ready the service is %q", got)
	}

	// --- metered: exactly ONE open span, at the ready edge -------------------
	edges := spanEdgesFor(t, w, svc.ID)
	if len(edges) != 1 || edges[0] != "open" {
		t.Fatalf("reaching ready produced %v, want exactly [open] — one span, opened at the ready edge", edges)
	}
	// "Metered" means BILLABLE. A span opened at rate 0 is never invoiced, and
	// untagged usage cannot be attributed to an org.
	var meter, product, spanOrg, spanEnv string
	var rate int64
	if err := w.pool.QueryRow(ctx,
		`SELECT meter, product, rate_cents, org_id, env_id FROM usage_events WHERE service_id = $1`,
		svc.ID).Scan(&meter, &product, &rate, &spanOrg, &spanEnv); err != nil {
		t.Fatal(err)
	}
	if meter != "service_span" || product != "postgres" {
		t.Fatalf("span is meter=%q product=%q", meter, product)
	}
	if rate != wantCents {
		t.Fatalf("the span bills at %d, the operator was quoted %d — a span at the wrong rate is worse than no span", rate, wantCents)
	}
	if spanOrg != org.Id || spanEnv != env.ID {
		t.Fatalf("span tagged org=%q env=%q, want org=%q env=%q — untagged usage cannot be attributed", spanOrg, spanEnv, org.Id, env.ID)
	}

	// --- and the CLI can see what it created --------------------------------
	code, out, errOut = runCLI(t, bin, w.srv.URL, cfgPath, "db", "list", "--env", env.ID)
	if code != 0 {
		t.Fatalf("db list failed (%d): %s %s", code, out, errOut)
	}
	var listed bool
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "orders") && strings.Contains(ln, "ready") && strings.Contains(ln, wantPrice) {
			listed = true
		}
	}
	if !listed {
		t.Fatalf("no single row lists the database as ready at %s — name, state and price must describe the SAME service:\n%s", wantPrice, out)
	}

	// --- the prompt must ACCEPT, not merely refuse --------------------------
	//
	// Declining proves ordering and that "n" refuses. It cannot see a prompt
	// whose answer never takes: a CLI where only `--yes` works would pass every
	// assertion above while no human could create a database interactively —
	// and `--yes` is the scripted path, not the one the criterion is about.
	//
	// A SECOND env keeps this off the estimate bookkeeping asserted for env.ID.
	env2, err := w.prov.CreateEnvironment(ctx, prj, "staging", "", false, false, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	code, out, errOut = runCLIStdin(t, bin, w.srv.URL, cfgPath, "y\n",
		"db", "create", "orders2", "--env", env2.ID, "--size", "dev", "--storage", "10")
	if code != 0 {
		t.Fatalf("answering y must create (%d)\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	accepted2, err := w.prov.ListServices(ctx, env2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted2) != 1 {
		t.Fatalf("answering y created %d service(s) — the interactive prompt does not accept", len(accepted2))
	}
	var acceptedCents2 int64
	if err := w.pool.QueryRow(ctx,
		`SELECT coalesce(max(total_cents), 0) FROM estimates WHERE env_id = $1 AND accepted_at IS NOT NULL`,
		env2.ID).Scan(&acceptedCents2); err != nil {
		t.Fatal(err)
	}
	if acceptedCents2 != wantCents {
		t.Fatalf("the interactively accepted estimate is %d, want %d", acceptedCents2, wantCents)
	}

	// --- criterion 2, asserted HERE so removing the link fails the checkpoint
	line, err := estimates.Price(estimates.ShapeInput{
		Product: "postgres", Name: "db-reports", Shape: map[string]any{"size": "dev", "storage_gb": 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	if line.MonthlyCents.Int64() != wantCents {
		t.Fatalf("the engine prices the canon db-reports shape at %d, canon says %d", line.MonthlyCents, wantCents)
	}
	cw, err := canon.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cw.CheckArithmetic(); err != nil {
		t.Fatalf("canon arithmetic invariant broke: %v", err)
	}
}

// assertEstimateShown checks the price PER LINE, not per blob.
//
// A whole-output `Contains(out, "$24/mo")` is satisfied by either surface alone,
// so a total reading `$0/mo` above correct line items passes — and `$0/mo` on
// the total line is the number the operator actually reads before typing y.
// Likewise `Contains(out, "orders")` is satisfied by the post-create
// confirmation line, so it cannot fail for the reason it names. Requiring the
// price and the label on the SAME line closes both.
func assertEstimateShown(t *testing.T, out, which, wantPrice string) {
	t.Helper()
	var totalLine, itemLine bool
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "total") && strings.Contains(ln, wantPrice) {
			totalLine = true
		}
		if strings.Contains(ln, "orders") && strings.Contains(ln, wantPrice) {
			itemLine = true
		}
	}
	if !totalLine {
		t.Fatalf("%s: no total line carrying %s — that is the number the operator reads before approving:\n%s", which, wantPrice, out)
	}
	if !itemLine {
		t.Fatalf("%s: no per-service line carrying %s (a total alone does not say what is being bought):\n%s", which, wantPrice, out)
	}
}

func mustStatus(t *testing.T, w *world, id string) string {
	t.Helper()
	var s string
	if err := w.pool.QueryRow(context.Background(),
		`SELECT status FROM services WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func spanCount(t *testing.T, w *world, serviceID string) int {
	t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM usage_events WHERE service_id = $1`, serviceID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func spanEdgesFor(t *testing.T, w *world, serviceID string) []string {
	t.Helper()
	rows, err := w.pool.Query(context.Background(),
		`SELECT edge FROM usage_events WHERE service_id = $1 ORDER BY at`, serviceID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var edges []string
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			t.Fatal(err)
		}
		edges = append(edges, e)
	}
	return edges
}
