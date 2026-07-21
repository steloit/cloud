package reconcile_test

// US-1.3a: the producing half — createService/updateService/deleteService write
// desired state and bump generation so edits and deletes reach the cell. Proven
// end-to-end against real Postgres: after each, the reconcile poll returns the
// service as OUTSTANDING (and delete carries a deleting desired flag).

import (
	"context"
	"encoding/json"
	"testing"

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
	svc := createSvc(t, pool, q, prov, "db6")
	rec := reconcile.New(q, prov)
	if _, err := rec.Writeback(context.Background(), "cell-0", reconcile.Report{ServiceID: svc.ID, ObservedGeneration: svc.Generation, Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	// Pin instances via override.
	edited, err := prov.UpdateService(context.Background(), mustGet(t, q, svc.ID), "org_w", "usr_w", nil, nil, []byte(`{"instances":5,"reason":"load"}`))
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
	_, err := q.UpdateServiceShape(context.Background(), store.UpdateServiceShapeParams{
		ID: svc.ID, Scaling: []byte(`{"mode":"auto"}`), Desired: []byte(`{"product":"postgres"}`),
	})
	if !errorsIsNoRows(err) {
		t.Fatalf("the SQL fence must reject an edit to a deleting row (zero rows), got %v", err)
	}
}

func errorsIsNoRows(err error) bool { return err != nil && err.Error() == pgx.ErrNoRows.Error() }
