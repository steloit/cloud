package reconcile_test

// US-1.3 step 5 (founder-gated): the generation guard proven against REAL
// PostgreSQL, not a fake. The unit tests mirror `MarkObserved`'s
// `WHERE generation >= $2` in an in-memory store — exactly the test shape that
// stays green while the real query is wrong. This drives the actual SQL through
// real migrations in a throwaway Postgres container.

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/metering"
	"github.com/steloit/cloud/services/api/internal/platform/db"
	"github.com/steloit/cloud/services/api/internal/provisioning"
	"github.com/steloit/cloud/services/api/internal/reconcile"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

const testKEK = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func realDB(t *testing.T) (*pgxpool.Pool, *store.Queries) {
	t.Helper()
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("app"), tcpostgres.WithUsername("app"), tcpostgres.WithPassword("app"),
		tcpostgres.BasicWaitStrategies(), tcpostgres.WithSQLDriver("pgx"))
	if err != nil {
		t.Skipf("container runtime unavailable (CI runs this): %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })
	url, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(url); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, store.New(pool)
}

// seedService inserts a minimal org→project→env→service graph and returns the
// service id. The reconciler columns default (generation=1, observed=0, cell-0).
func seedService(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	ctx := context.Background()
	ex := func(sql string, args ...any) {
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
	}
	ex(`INSERT INTO orgs (id, name) VALUES ('org_it', 'itco') ON CONFLICT DO NOTHING`)
	ex(`INSERT INTO projects (id, org_id, name, cell_id) VALUES ('prj_it', 'org_it', 'p', 'cell-0') ON CONFLICT DO NOTHING`)
	ex(`INSERT INTO environments (id, project_id, name) VALUES ('env_it', 'prj_it', 'prod') ON CONFLICT DO NOTHING`)
	ex(`INSERT INTO services (id, env_id, name, product, status, cell_id, desired)
	    VALUES ($1, 'env_it', $1, 'postgres', 'provisioning', 'cell-0', '{"product":"postgres"}')`, id)
}

func newReconciler(pool *pgxpool.Pool, q *store.Queries) (*reconcile.Service, error) {
	kek, err := secrets.NewEnvKEK("test-v1", testKEK)
	if err != nil {
		return nil, err
	}
	plans, err := billing.Load()
	if err != nil {
		return nil, err
	}
	rec := events.NewRecorder(q, events.NewHub())
	prov := provisioning.NewService(pool, rec, secrets.NewVault(q, kek), metering.NewEmitter(q), plans)
	return reconcile.New(q, prov), nil
}

// The founder-gated criterion: a stale report is REJECTED by real Postgres.
func TestGuardRejectsStaleAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedService(t, pool, "svc_real")

	// Bump desired to generation 3 (two edits: 1→2→3), so a report on an older
	// generation is genuinely stale.
	if _, err := q.BumpServiceGeneration(ctx, store.BumpServiceGenerationParams{ID: "svc_real", Desired: []byte(`{"v":2}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.BumpServiceGeneration(ctx, store.BumpServiceGenerationParams{ID: "svc_real", Desired: []byte(`{"v":3}`)}); err != nil {
		t.Fatal(err)
	}

	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}

	// A report ahead of desired (gen 9 > 3) is stale — the guard rejects it.
	if _, err := svc.Writeback(ctx, "cell-0", reconcile.Report{ServiceID: "svc_real", ObservedGeneration: 9, Status: "ready"}); err == nil {
		t.Fatal("real Postgres accepted a stale generation — the WHERE generation >= $2 guard is not enforcing")
	}
	// The row must be untouched: observed_generation still 0, status unchanged.
	row, err := q.GetService(ctx, "svc_real")
	if err != nil {
		t.Fatal(err)
	}
	if row.ObservedGeneration != 0 {
		t.Fatalf("a rejected report advanced observed_generation to %d", row.ObservedGeneration)
	}
	if row.Status != "provisioning" {
		t.Fatalf("a rejected report changed status to %q", row.Status)
	}
}

// A valid report drives a real transition and advances observed_generation in
// real Postgres — and, per ADR-024/D10, appends a real spine event.
func TestValidWritebackDrivesRealTransition(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedService(t, pool, "svc_ok")

	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Writeback(ctx, "cell-0", reconcile.Report{ServiceID: "svc_ok", ObservedGeneration: 1, Status: "ready"}); err != nil {
		t.Fatalf("valid writeback failed: %v", err)
	}
	row, err := q.GetService(ctx, "svc_ok")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "ready" {
		t.Fatalf("status not driven to ready: %q", row.Status)
	}
	if row.ObservedGeneration != 1 {
		t.Fatalf("observed_generation not advanced: %d", row.ObservedGeneration)
	}
	if !row.LastReconciledAt.Valid {
		t.Fatal("last_reconciled_at not stamped")
	}
	// D10: the edge must have appended a spine event.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE subject = 'svc_ok' AND action = 'service.ready'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected exactly one service.ready spine event, got %d", n)
	}
}

// The poll query filters on generation against real Postgres.
func TestDesiredPollFiltersAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedService(t, pool, "svc_poll")
	// desired at gen 1; bump to 2.
	if _, err := q.BumpServiceGeneration(ctx, store.BumpServiceGenerationParams{ID: "svc_poll", Desired: []byte(`{"v":2}`)}); err != nil {
		t.Fatal(err)
	}
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	// since_generation=2 → nothing (row is exactly at 2, filter is strictly >)
	got, err := svc.Desired(ctx, "cell-0", 2, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("since_generation=2 should exclude a gen-2 row (strict >), got %d", len(got))
	}
	// since_generation=1 → the row (gen 2 > 1)
	got, err = svc.Desired(ctx, "cell-0", 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "svc_poll" {
		t.Fatalf("since_generation=1 should return svc_poll, got %+v", got)
	}
}

// Belt-and-braces: the FK actually adopted cell-0 and heartbeat writes land.
func TestHeartbeatPersistsAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedService(t, pool, "svc_hb")
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if _, err := svc.Writeback(ctx, "cell-0", reconcile.Report{ServiceID: "svc_hb", ObservedGeneration: 1}); err != nil {
		t.Fatal(err)
	}
	cell, err := q.GetCell(ctx, "cell-0")
	if err != nil {
		t.Fatal(err)
	}
	if !cell.AgentLastSeenAt.Valid || cell.AgentLastSeenAt.Time.Before(before) {
		t.Fatal("heartbeat did not update agent_last_seen_at")
	}
}
