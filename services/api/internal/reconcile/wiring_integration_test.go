package reconcile_test

// US-1.3a: the producing half — createService/updateService/deleteService write
// desired state and bump generation so edits and deletes reach the cell. Proven
// end-to-end against real Postgres: after each, the reconcile poll returns the
// service as OUTSTANDING (and delete carries a deleting desired flag).

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/metering"
	"github.com/steloit/cloud/services/api/internal/provisioning"
	"github.com/steloit/cloud/services/api/internal/reconcile"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

func newProvisioning(t *testing.T, pool *pgxpool.Pool, q *store.Queries) *provisioning.Service {
	t.Helper()
	kek, err := secrets.NewEnvKEK("test-v1", testKEK)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := billing.Load()
	if err != nil {
		t.Fatal(err)
	}
	rec := events.NewRecorder(q, events.NewHub())
	return provisioning.NewService(pool, rec, secrets.NewVault(q, kek), metering.NewEmitter(q), plans)
}

func seedGraph(t *testing.T, pool *pgxpool.Pool, q *store.Queries) store.Environment {
	t.Helper()
	ctx := context.Background()
	for _, sql := range []string{
		`INSERT INTO orgs (id, name, slug) VALUES ('org_w', 'wco', 'wco') ON CONFLICT DO NOTHING`,
		`INSERT INTO projects (id, org_id, name, cell_id) VALUES ('prj_w', 'org_w', 'p', 'cell-0') ON CONFLICT DO NOTHING`,
		`INSERT INTO environments (id, project_id, name) VALUES ('env_w', 'prj_w', 'prod') ON CONFLICT DO NOTHING`,
	} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	env, err := q.GetEnvironment(ctx, "env_w")
	if err != nil {
		t.Fatal(err)
	}
	return env
}

// createWebSvc makes a service whose catalog shape PRICES an instance count, so
// a D22 manual pin is legal on it. postgres has no priced instance count and its
// pins are refused (US-3.8), so a pin test cannot use one.
func createWebSvc(t *testing.T, pool *pgxpool.Pool, q *store.Queries, prov *provisioning.Service, name string) store.Service {
	t.Helper()
	ctx := context.Background()
	env := seedGraph(t, pool, q)
	est := estimates.NewService(q)
	created, err := est.Create(ctx, "org_w", "env_w", []estimates.ShapeInput{
		{Product: "web", Name: name, Shape: map[string]any{"size": "standard-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := prov.CreateService(ctx, est, env, "org_w", provisioning.CreateServiceInput{
		Name: name, Product: "web", Shape: map[string]any{"size": "standard-1"},
		EstimateID: created.Row.ID, ActorID: "usr_w",
	})
	if err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	return svc
}

func createSvc(t *testing.T, pool *pgxpool.Pool, q *store.Queries, prov *provisioning.Service, name string) store.Service {
	t.Helper()
	ctx := context.Background()
	env := seedGraph(t, pool, q)
	est := estimates.NewService(q)
	created, err := est.Create(ctx, "org_w", "env_w", []estimates.ShapeInput{
		{Product: "postgres", Name: name, Shape: map[string]any{"size": "dev"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := prov.CreateService(ctx, est, env, "org_w", provisioning.CreateServiceInput{
		Name: name, Product: "postgres", Shape: map[string]any{"size": "dev"},
		EstimateID: created.Row.ID, ActorID: "usr_w",
	})
	if err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	return svc
}

func mustGet(t *testing.T, q *store.Queries, id string) store.Service {
	t.Helper()
	s, err := q.GetService(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func outstanding(t *testing.T, rec *reconcile.Service, id string) bool {
	t.Helper()
	out, err := rec.Desired(context.Background(), "cell-0", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range out {
		if s.ID == id {
			return true
		}
	}
	return false
}

func TestCreateServicePopulatesDesired(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	svc := createSvc(t, pool, q, prov, "db")

	var d map[string]any
	if err := json.Unmarshal(svc.Desired, &d); err != nil {
		t.Fatal(err)
	}
	if d["product"] != "postgres" {
		t.Fatalf("createService did not populate desired (still the '{}' default?): %s", svc.Desired)
	}
	rec := reconcile.New(q, prov)
	if !outstanding(t, rec, svc.ID) {
		t.Fatal("a freshly created service must be outstanding (observed 0 < generation 1)")
	}
}

func TestShapeEditBumpsGenerationAndBecomesOutstanding(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	svc := createSvc(t, pool, q, prov, "db2")
	rec := reconcile.New(q, prov)

	// Converge it: observed = generation, so it is no longer outstanding.
	if _, err := rec.Writeback(context.Background(), "cell-0", reconcile.Report{ServiceID: svc.ID, ObservedGeneration: svc.Generation, Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	if outstanding(t, rec, svc.ID) {
		t.Fatal("a converged service must not be outstanding before the edit")
	}
	// A scaling edit bumps generation and makes it outstanding again.
	edited, err := prov.UpdateService(context.Background(), mustGet(t, q, svc.ID), "org_w", "usr_w", nil, []byte(`{"mode":"auto"}`), nil)
	if err != nil {
		t.Fatalf("UpdateService: %v", err)
	}
	if edited.Generation <= svc.Generation {
		t.Fatalf("edit did not bump generation: %d → %d", svc.Generation, edited.Generation)
	}
	if !outstanding(t, rec, svc.ID) {
		t.Fatal("an edited service must be outstanding again")
	}
}

func TestDeleteWritesDeletingDesiredAndBecomesOutstanding(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	svc := createSvc(t, pool, q, prov, "db3")
	rec := reconcile.New(q, prov)

	if _, err := rec.Writeback(context.Background(), "cell-0", reconcile.Report{ServiceID: svc.ID, ObservedGeneration: svc.Generation, Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	if err := prov.DeleteService(context.Background(), mustGet(t, q, svc.ID), "org_w", "usr_w"); err != nil {
		t.Fatalf("DeleteService: %v", err)
	}
	row := mustGet(t, q, svc.ID)
	if row.Status != "deleting" {
		t.Fatalf("status not deleting: %q", row.Status)
	}
	var d map[string]any
	_ = json.Unmarshal(row.Desired, &d)
	if d["deleting"] != true {
		t.Fatalf("desired missing deleting flag: %s", row.Desired)
	}
	if !outstanding(t, rec, svc.ID) {
		t.Fatal("a deleting service must be outstanding so the cell converges the teardown")
	}
}

// A deleting service must not be editable: an edit would rewrite desired with
// deleting=false and re-outstand the row, cancelling the teardown. (US-1.3a
// review finding.)
func TestUpdateOnDeletingServiceIsRejected(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	svc := createSvc(t, pool, q, prov, "db4")
	rec := reconcile.New(q, prov)
	if _, err := rec.Writeback(context.Background(), "cell-0", reconcile.Report{ServiceID: svc.ID, ObservedGeneration: svc.Generation, Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	if err := prov.DeleteService(context.Background(), mustGet(t, q, svc.ID), "org_w", "usr_w"); err != nil {
		t.Fatal(err)
	}
	// An edit on the now-deleting service must be refused.
	_, err := prov.UpdateService(context.Background(), mustGet(t, q, svc.ID), "org_w", "usr_w", nil, []byte(`{"mode":"auto"}`), nil)
	if err == nil {
		t.Fatal("editing a deleting service must be rejected — it would cancel the teardown")
	}
	// The deleting desired doc must survive the rejected edit.
	var d map[string]any
	_ = json.Unmarshal(mustGet(t, q, svc.ID).Desired, &d)
	if d["deleting"] != true {
		t.Fatal("the rejected edit clobbered the deleting flag")
	}
}

// A no-op update still bumps generation and re-converges — pinned as intended
// (harmless under an idempotent renderer; noted so it is a decision, not a surprise).
func TestNoOpUpdateStillBumpsGeneration(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	svc := createSvc(t, pool, q, prov, "db5")
	edited, err := prov.UpdateService(context.Background(), svc, "org_w", "usr_w", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Generation <= svc.Generation {
		t.Fatalf("a no-op update is expected to still bump generation (idempotent re-converge): %d → %d", svc.Generation, edited.Generation)
	}
}

// An override (manual instance-pin) edit must reach the cell via desired — it is
// load-bearing capacity state. Regression for the review finding that override
// was dropped from the desired doc.
func TestOverrideEditReachesCell(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	svc := createWebSvc(t, pool, q, prov, "api6")
	rec := reconcile.New(q, prov)
	if _, err := rec.Writeback(context.Background(), "cell-0", reconcile.Report{ServiceID: svc.ID, ObservedGeneration: svc.Generation, Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	// Pin instances via override. `expires_at` is REQUIRED: D22 makes the pin
	// temporary, the API always stamps one, and a pin without an expiry is not
	// honoured — treating "unset" as "forever" is what made pins permanent.
	pin := fmt.Sprintf(`{"instances":5,"reason":"load","expires_at":%q}`,
		time.Now().Add(24*time.Hour).UTC().Format(time.RFC3339))
	edited, err := prov.UpdateService(context.Background(), mustGet(t, q, svc.ID), "org_w", "usr_w", nil, nil, []byte(pin))
	if err != nil {
		t.Fatalf("UpdateService(override): %v", err)
	}
	if edited.Generation <= svc.Generation {
		t.Fatal("override edit did not bump generation")
	}
	// The desired doc the cell renders from must carry the pin.
	var d map[string]any
	_ = json.Unmarshal(edited.Desired, &d)
	ov, ok := d["override"].(map[string]any)
	if !ok || ov["instances"] != float64(5) {
		t.Fatalf("override pin did not reach desired (the cell would ignore it): %s", edited.Desired)
	}
	if !outstanding(t, rec, svc.ID) {
		t.Fatal("an override edit must make the service outstanding")
	}
}

// The SQL fence in UpdateServiceShape (not just the Go pre-check) must reject an
// edit to a deleting row — this is the atomic backstop for the TOCTOU. Drives
// the generated query DIRECTLY so a stale (unfenced) generated const fails here.
func TestUpdateServiceShapeSQLFenceRejectsDeleting(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	svc := createSvc(t, pool, q, prov, "db7")
	rec := reconcile.New(q, prov)
	if _, err := rec.Writeback(context.Background(), "cell-0", reconcile.Report{ServiceID: svc.ID, ObservedGeneration: svc.Generation, Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	if err := prov.DeleteService(context.Background(), mustGet(t, q, svc.ID), "org_w", "usr_w"); err != nil {
		t.Fatal(err)
	}
	// Call the generated query directly, bypassing UpdateService's Go guard: the
	// SQL fence WHERE status <> 'deleting' must return zero rows for a deleting row.
	//
	// The generation MUST be the row's current one. US-3.8 added `AND generation
	// = $7` to this query, and while `Generation` was left at its zero value that
	// fence alone returned zero rows — so this test passed without the deleting
	// fence ever being consulted, and deleting `AND status <> 'deleting'` from
	// the query left it green. Every other route into this statement is stopped
	// by the Go pre-check in UpdateService, so this test is the ONLY owner of the
	// atomic backstop for that TOCTOU. Read the generation fresh: DeleteService
	// bumps it.
	deleting := mustGet(t, q, svc.ID)
	_, err := q.UpdateServiceShape(context.Background(), store.UpdateServiceShapeParams{
		ID: svc.ID, Generation: deleting.Generation,
		Scaling: []byte(`{"mode":"auto"}`), Desired: []byte(`{"product":"postgres"}`),
	})
	if !errorsIsNoRows(err) {
		t.Fatalf("the SQL fence must reject an edit to a deleting row (zero rows), got %v", err)
	}
	// ...and the same call at the same generation against a NON-deleting row must
	// succeed. Without this the two fences are not separable: a query that
	// rejected everything would satisfy the assertion above.
	live := createSvc(t, pool, q, prov, "db7-live")
	if _, err := q.UpdateServiceShape(context.Background(), store.UpdateServiceShapeParams{
		ID: live.ID, Generation: live.Generation,
		Scaling: []byte(`{"mode":"auto"}`), Desired: []byte(`{"product":"postgres"}`),
	}); err != nil {
		t.Fatalf("the same edit must succeed on a live row at the right generation: %v", err)
	}
}

func errorsIsNoRows(err error) bool { return err != nil && err.Error() == pgx.ErrNoRows.Error() }

// US-3.3: createService resolves and embeds the cell namespace (proj--env) in
// desired, so the cell-agent renders into the right namespace.
func TestCreateServiceResolvesNamespace(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	svc := createSvc(t, pool, q, prov, "db-ns")
	var d map[string]any
	_ = json.Unmarshal(svc.Desired, &d)
	ns, _ := d["namespace"].(string)
	// Derived from the ENV ID, not names: project/env names are unique only per
	// ORG, so a name-derived namespace would put two orgs' api/prod in the same
	// namespace — sharing D7's tenant isolation boundary. seedGraph uses env_w.
	if ns != "env-w" {
		t.Fatalf("namespace must derive from the immutable env id (want env-w): %q in %s", ns, svc.Desired)
	}
}

// The security regression: two DIFFERENT orgs may each have project "api" with
// env "prod" (projects are UNIQUE (org_id, name), not globally unique). A
// name-derived namespace would put both in the SAME Kubernetes namespace,
// sharing D7's tenant isolation boundary — default-deny NetworkPolicy,
// ResourceQuota, and CNPG's generated <cluster>-app credential Secrets.
func TestNamespacesDoNotCollideAcrossOrgs(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	ctx := context.Background()
	ex := func(sql string, args ...any) {
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// two orgs, each with project "api" / env "prod" — the DEFAULT case, not an edge
	for i, org := range []string{"org_x", "org_y"} {
		ex(`INSERT INTO orgs (id, name, slug) VALUES ($1,$1,$1) ON CONFLICT DO NOTHING`, org)
		ex(`INSERT INTO projects (id, org_id, name, cell_id) VALUES ($1,$2,'api','cell-0') ON CONFLICT DO NOTHING`,
			"prj_"+org, org)
		ex(`INSERT INTO environments (id, project_id, name) VALUES ($1,$2,'prod') ON CONFLICT DO NOTHING`,
			"env_"+org, "prj_"+org)
		_ = i
	}
	nsX, err := prov.ResolveNamespaceForTest(ctx, "env_org_x")
	if err != nil {
		t.Fatal(err)
	}
	nsY, err := prov.ResolveNamespaceForTest(ctx, "env_org_y")
	if err != nil {
		t.Fatal(err)
	}
	if nsX == nsY {
		t.Fatalf("two orgs' api/prod collided into one namespace %q — cross-tenant isolation breach", nsX)
	}
}

// US-3.3's headline control-plane AC, against real Postgres: a service walks
// provisioning → ready through the RECONCILER, and the metering span opens at
// ready — never before. (D10: metering starts at ready.)
func TestMeteringStartsAtReadyE2E(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	ctx := context.Background()
	svc := createSvc(t, pool, q, prov, "db-meter")
	rec := reconcile.New(q, prov)

	// BEFORE ready: the service exists and is outstanding — no usage event yet.
	var before int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM usage_events WHERE service_id=$1`, svc.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before != 0 {
		t.Fatalf("metering opened BEFORE ready (%d events) — D10 says never before", before)
	}

	// The agent reports ready (what CNPGRenderer returns once the cluster is
	// healthy — the live drill proved that mapping on a real operator).
	if _, err := rec.Writeback(ctx, "cell-0", reconcile.Report{
		ServiceID: svc.ID, ObservedGeneration: svc.Generation, Status: "ready",
	}); err != nil {
		t.Fatal(err)
	}

	// AT ready: exactly one span opened.
	var edges []string
	rows, err := pool.Query(ctx, `SELECT edge FROM usage_events WHERE service_id=$1 ORDER BY at`, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			t.Fatal(err)
		}
		edges = append(edges, e)
	}
	if len(edges) != 1 || edges[0] != "open" {
		t.Fatalf("expected exactly one edge='open' at ready, got %v", edges)
	}
	if mustGet(t, q, svc.ID).Status != "ready" {
		t.Fatal("service did not reach ready")
	}
}

// US-3.6 headline: a provisioning attempt that FAILS never bills. Metering opens
// only on the ready edge (D10), so a service that goes provisioning → failed
// must have ZERO usage events — and a subsequent retry that succeeds must open
// exactly ONE span, not two.
func TestFailedProvisioningNeverBills(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	ctx := context.Background()
	svc := createSvc(t, pool, q, prov, "db-fail")
	rec := reconcile.New(q, prov)

	// The cell reports the provisioning attempt failed.
	if _, err := rec.Writeback(ctx, "cell-0", reconcile.Report{
		ServiceID: svc.ID, ObservedGeneration: svc.Generation, Status: "failed",
	}); err != nil {
		t.Fatal(err)
	}
	if got := mustGet(t, q, svc.ID).Status; got != "failed" {
		t.Fatalf("status %q, want failed", got)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM usage_events WHERE service_id=$1`, svc.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a FAILED provisioning billed %d usage event(s) — D10: metering starts at ready, never before", n)
	}

	// Retry: failed → provisioning → ready must open EXACTLY ONE span.
	row := mustGet(t, q, svc.ID)
	if _, err := prov.Transition(ctx, row, "provisioning", "system", "system", "org_w"); err != nil {
		t.Fatal(err)
	}
	row = mustGet(t, q, svc.ID)
	if _, err := prov.Transition(ctx, row, "ready", "system", "system", "org_w"); err != nil {
		t.Fatal(err)
	}
	var edges []string
	rows, err := pool.Query(ctx, `SELECT edge FROM usage_events WHERE service_id=$1 ORDER BY at`, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			t.Fatal(err)
		}
		edges = append(edges, e)
	}
	if len(edges) != 1 || edges[0] != "open" {
		t.Fatalf("a failed→retry→ready sequence must open exactly one span, got %v", edges)
	}
}

// The money-losing sibling of "a failure never bills": a service that reaches
// ready and LATER fails must CLOSE its span. Without a close edge the span
// stays open and the customer is billed for a service that is not running —
// the same class of defect as billing a failure, in the opposite direction.
//
// The legal path is ready → degraded → failed (ADR-024; `ready` has no direct
// edge to `failed`). `degraded` is still a BILLING state, so the close must
// land on the degraded → failed edge, not earlier.
func TestFailureAfterReadyClosesTheSpan(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	ctx := context.Background()
	svc := createSvc(t, pool, q, prov, "db-close")

	row := mustGet(t, q, svc.ID)
	if _, err := prov.Transition(ctx, row, "ready", "system", "system", "org_w"); err != nil {
		t.Fatal(err)
	}
	row = mustGet(t, q, svc.ID)
	if _, err := prov.Transition(ctx, row, "degraded", "system", "system", "org_w"); err != nil {
		t.Fatal(err)
	}
	// degraded still bills, so nothing may have closed yet.
	if got := spanEdges(t, pool, svc.ID); len(got) != 1 || got[0] != "open" {
		t.Fatalf("ready→degraded produced %v, want [open] — degraded is still a billing state", got)
	}

	row = mustGet(t, q, svc.ID)
	if _, err := prov.Transition(ctx, row, "failed", "system", "system", "org_w"); err != nil {
		t.Fatal(err)
	}
	if got := spanEdges(t, pool, svc.ID); len(got) != 2 || got[0] != "open" || got[1] != "close" {
		t.Fatalf("degraded→failed produced %v, want [open close] — an unclosed span bills a service that is not running", got)
	}
}

func spanEdges(t *testing.T, pool *pgxpool.Pool, serviceID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT edge FROM usage_events WHERE service_id=$1 ORDER BY at`, serviceID)
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

// The clear's `status <> 'deleting'` fence, driven through the exact
// interleaving that reaches it.
//
// This fence was recorded as untestable defence-in-depth, on the reasoning that
// "every path that starts a delete also bumps generation, so the generation
// fence always fires first." That reasoning is FALSE, and DeleteService is its
// own counterexample: it does BumpServiceGeneration (gen→N+1) and only THEN
// Transition→deleting, via SetServiceStatus, which does not touch generation.
// Two non-transactional writes. A sweep that lists the row in between arrives at
// the clear with a generation that MATCHES — so without this fence the clear
// succeeds, rewriting desired WITHOUT deleting:true and bumping generation
// again, and the cell converges an in-flight teardown back into existence.
//
// The window DeleteService's own comment documents for crashes is equally open
// to the sweeper. Recording an arm as unreachable without checking the reason is
// how a live fence gets filed as decoration.
func TestTheClearFenceRejectsARowThatStartedDeletingMidSweep(t *testing.T) {
	pool, q := realDB(t)
	prov := newProvisioning(t, pool, q)
	ctx := context.Background()
	svc := createSvc(t, pool, q, prov, "db-midsweep")

	pin := `{"instances":3,"reason":"x","expires_at":"` +
		time.Now().Add(-time.Hour).UTC().Format(time.RFC3339) + `"}`
	if _, err := pool.Exec(ctx, `UPDATE services SET override = $2::jsonb WHERE id = $1`, svc.ID, pin); err != nil {
		t.Fatal(err)
	}

	// Drive DeleteService's TWO WRITES with the sweeper's read interposed between
	// them — which is the whole reachability claim, and asserting it is the point
	// of this test. An earlier version listed BEFORE the delete and hand-fed the
	// generation, which proved the fence fires but left "the sweeper can list a
	// row already carrying the post-bump generation" as prose. If those two
	// writes are ever made one transaction (a carried atomicity finding,
	// services.go), that version would still have passed while the comment it
	// supports became false — the same failure mode, one round later.
	before := mustGet(t, q, svc.ID)
	if _, err := q.BumpServiceGeneration(ctx, store.BumpServiceGenerationParams{
		ID: svc.ID, Desired: []byte(`{"product":"postgres","deleting":true}`)}); err != nil {
		t.Fatal(err)
	}
	// Write 1 done, write 2 not yet: status is still provisioning, so the sweeper
	// lists — and what it reads is the POST-BUMP generation.
	listed, err := q.ListExpiredOverrides(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var read *store.Service
	for i := range listed {
		if listed[i].ID == svc.ID {
			read = &listed[i]
		}
	}
	if read == nil {
		t.Fatal("a dead pin on a row mid-delete was not listed — if that is now intended, this fence is unreachable and the comment on it is wrong")
	}
	if read.Generation == before.Generation {
		t.Fatalf("the sweeper read generation %d, the same as before the bump — the window this fence covers depends on the read happening AFTER it", read.Generation)
	}
	// Write 2: status flips, generation untouched.
	if _, err := q.SetServiceStatus(ctx, store.SetServiceStatusParams{
		ID: svc.ID, Status: "provisioning", Status_2: "deleting"}); err != nil {
		t.Fatal(err)
	}
	after := mustGet(t, q, svc.ID)
	if after.Generation != read.Generation {
		t.Fatalf("the status flip moved generation (%d → %d); if SetServiceStatus now bumps, the generation fence alone would stop the sweeper and this fence's reason has changed",
			read.Generation, after.Generation)
	}
	if after.Status != "deleting" {
		t.Fatalf("status %q, want deleting", after.Status)
	}
	// The sweeper now clears using the generation IT read — which, because
	// SetServiceStatus does not bump, is still the row's generation.
	_, err = q.ClearExpiredOverride(ctx, store.ClearExpiredOverrideParams{
		ID: svc.ID, MonthlyEstimateCents: 100, Desired: []byte(`{"product":"web"}`),
		Generation: after.Generation,
	})
	if !errorsIsNoRows(err) {
		t.Fatalf("the clear was not refused on a deleting row (got %v) — it has just replaced the teardown's desired doc with one that does not say deleting, and bumped generation, so the cell will converge the service back into existence", err)
	}
	final := mustGet(t, q, svc.ID)
	var d map[string]any
	_ = json.Unmarshal(final.Desired, &d)
	if d["deleting"] != true {
		t.Fatalf("the teardown's desired doc was overwritten: %s", final.Desired)
	}
	if final.Override == nil {
		t.Fatal("the pin was cleared on a deleting row")
	}
}
