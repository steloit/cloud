package reconcile_test

// US-1.3 step 5 (founder-gated): the generation guard proven against REAL
// PostgreSQL, not a fake. The unit tests mirror `MarkObserved`'s exact-match
// guard in an in-memory store — exactly the test shape that stays green while
// the real query is wrong. These drive the actual SQL through real migrations
// in a throwaway Postgres container. Both directions of the guard are covered:
// a BEHIND report (the AC's literal scenario) and an impossible AHEAD report.
//
// Runs in CI (Docker present). Locally it needs a reachable daemon —
// DOCKER_HOST set for colima. With no runtime it SKIPS locally and FAILS in CI,
// because CI sets STELOIT_REQUIRE_CONTAINERS (O23): a skip is not a pass, and in
// CI the two are indistinguishable at the job level.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/metering"
	"github.com/steloit/cloud/services/api/internal/platform/db"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
	"github.com/steloit/cloud/services/api/internal/platform/testenv"
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
		testenv.SkipOrFail(t, err) // skip locally, FAIL in CI — see the package doc
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
	ex(`INSERT INTO orgs (id, name, slug) VALUES ('org_it', 'itco', 'itco') ON CONFLICT DO NOTHING`)
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
	return reconcile.New(q, prov, reconcile.WithRecorder(rec)), nil
}

// The founder-gated criterion, the AC's LITERAL scenario: the agent reports
// generation N while desired has moved to N+1 (it converged the old desired),
// against REAL Postgres. Must be rejected and the row left unchanged.
func TestBehindReportRejectedAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedService(t, pool, "svc_behind") // generation 1, observed 0
	// Desired moves to generation 2 while the agent is still converging gen 1.
	if _, err := q.BumpServiceGeneration(ctx, store.BumpServiceGenerationParams{ID: "svc_behind", Desired: []byte(`{"v":2}`)}); err != nil {
		t.Fatal(err)
	}
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	// The agent reports observed_generation=1 (the old desired it converged).
	if _, err := svc.Writeback(ctx, "cell-0", reconcile.Report{ServiceID: "svc_behind", ObservedGeneration: 1, Status: "ready"}); err == nil {
		t.Fatal("real Postgres accepted a BEHIND report — the exact-match guard is not enforcing")
	}
	row, err := q.GetService(ctx, "svc_behind")
	if err != nil {
		t.Fatal(err)
	}
	if row.ObservedGeneration != 0 {
		t.Fatalf("a behind report advanced observed_generation to %d", row.ObservedGeneration)
	}
	if row.Status != "provisioning" {
		t.Fatalf("a behind report drove status to %q off stale desired", row.Status)
	}
}

// The poll returns OUTSTANDING work only — a service whose report has landed
// drops out (proven against real Postgres, not the fake).
func TestOutstandingWorkPollAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedService(t, pool, "svc_ow") // gen 1, observed 0 → outstanding
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Desired(ctx, "cell-0", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Services) != 1 || got.Services[0].ID != "svc_ow" {
		t.Fatalf("a fresh service (observed 0 < gen 1) must be outstanding, got %+v", got.Services)
	}
	// Report it converged; observed advances to 1; it must drop out.
	if _, err := svc.Writeback(ctx, "cell-0", reconcile.Report{ServiceID: "svc_ow", ObservedGeneration: 1, Status: "ready"}); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Desired(ctx, "cell-0", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Services) != 0 {
		t.Fatalf("a reported service must drop out of the outstanding set, got %+v", got.Services)
	}
}

// An impossible AHEAD report is also rejected by real Postgres.
func TestAheadReportRejectedAgainstRealPostgres(t *testing.T) {
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

	// A report ahead of desired (gen 9 > 3) is impossible — the exact-match guard rejects it.
	if _, err := svc.Writeback(ctx, "cell-0", reconcile.Report{ServiceID: "svc_real", ObservedGeneration: 9, Status: "ready"}); err == nil {
		t.Fatal("real Postgres accepted an ahead generation — the WHERE generation = $2 guard is not enforcing")
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
	if len(got.Services) != 0 {
		t.Fatalf("since_generation=2 should exclude a gen-2 row (strict >), got %d", len(got.Services))
	}
	// since_generation=1 → the row (gen 2 > 1)
	got, err = svc.Desired(ctx, "cell-0", 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Services) != 1 || got.Services[0].ID != "svc_poll" {
		t.Fatalf("since_generation=1 should return svc_poll, got %+v", got.Services)
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
	// Compare against the DB clock (SELECT now()), not the host clock — a colima
	// VM clock can drift from the host and flake a host-vs-container comparison.
	var before time.Time
	if err := pool.QueryRow(ctx, "SELECT now()").Scan(&before); err != nil {
		t.Fatal(err)
	}
	// svc_hb is `provisioning`, so an observation-only ack does not finish the
	// generation (the cell said nothing about status and the row is mid-apply).
	// The heartbeat runs FIRST precisely so an agent that is alive but reporting
	// something the machine will not accept still counts as seen.
	if _, err := svc.Writeback(ctx, "cell-0", reconcile.Report{ServiceID: "svc_hb", ObservedGeneration: 1}); !errors.Is(err, reconcile.ErrNotConverged) {
		t.Fatalf("err = %v, want ErrNotConverged", err)
	}
	cell, err := q.GetCell(ctx, "cell-0")
	if err != nil {
		t.Fatal(err)
	}
	if !cell.AgentLastSeenAt.Valid || cell.AgentLastSeenAt.Time.Before(before) {
		t.Fatal("heartbeat did not update agent_last_seen_at")
	}
}

// Concurrent identical writebacks apply the status edge EXACTLY ONCE against
// real Postgres — the guarantee rests on SetServiceStatus's WHERE status = $2
// FROM-guard, which the unit-test fake only approximates. Fire N reports of the
// same edge; exactly one drives it, and exactly one spine event is appended.
func TestConcurrentWritebackAppliesOnceAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedService(t, pool, "svc_conc") // gen 1, observed 0, provisioning
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Writeback(ctx, "cell-0", reconcile.Report{ServiceID: "svc_conc", ObservedGeneration: 1, Status: "ready"})
		}()
	}
	wg.Wait()
	row, err := q.GetService(ctx, "svc_conc")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "ready" {
		t.Fatalf("final status %q, want ready", row.Status)
	}
	var events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE subject='svc_conc' AND action='service.ready'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("the provisioning→ready edge fired %d times under concurrency, want exactly 1", events)
	}
}

// The founder-gated guard lives in TWO places now: a Go pre-check in Writeback
// AND the SQL WHERE generation = $2 in MarkObserved. The pre-check shadows the
// SQL in the Writeback path, so this drives MarkObserved DIRECTLY to keep the
// SQL half mutation-live — deleting `AND generation = $2` must make THIS fail.
func TestMarkObservedSQLGuardDirectAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedService(t, pool, "svc_sql") // generation 1, observed 0
	if _, err := q.BumpServiceGeneration(ctx, store.BumpServiceGenerationParams{ID: "svc_sql", Desired: []byte(`{"v":2}`)}); err != nil {
		t.Fatal(err) // now generation 2
	}
	// Behind (1) and ahead (9) must both be rejected by the SQL guard alone.
	for _, bad := range []int64{1, 9} {
		_, err := q.MarkObserved(ctx, store.MarkObservedParams{ID: "svc_sql", ObservedGeneration: bad})
		if err == nil {
			t.Fatalf("MarkObserved SQL guard accepted generation %d (current is 2) — WHERE generation = $2 is not enforcing", bad)
		}
	}
	// The current generation (2) is accepted and advances observed.
	row, err := q.MarkObserved(ctx, store.MarkObservedParams{ID: "svc_sql", ObservedGeneration: 2})
	if err != nil {
		t.Fatalf("MarkObserved rejected the current generation: %v", err)
	}
	if row.ObservedGeneration != 2 {
		t.Fatalf("observed_generation not advanced to 2: %d", row.ObservedGeneration)
	}
}

// THE BILLING ARGUMENT, MEASURED — not asserted in a comment.
//
// The whole reason `ready` + `failed` → `degraded` must not converge is that
// `degraded` BILLS and `degraded → failed` is the ONLY edge that emits a
// metering `close`. Every unit test for that path runs against a fake
// Transitioner that emits no spans at all, so the claim was pinned nowhere.
//
// This drives the real provisioning.Service and the real metering emitter
// against real Postgres, across the whole two-hop path, and counts the spans the
// invoice would actually be built from.
func TestABrokenReadyServiceStopsBillingAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedService(t, pool, "svc_bill")

	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	spans := func() []string {
		// ORDER BY at alone: `id` is a random `use_<hex>` and would be a coin
		// flip, not a tiebreak. Today the two edges land in separate Writeback
		// transactions so `at` (transaction timestamp) separates them; the
		// strict-increase assertion below is what would catch a future emitter
		// that writes both in one transaction, rather than letting the order
		// silently become arbitrary and flake in CI only.
		rows, err := pool.Query(ctx, `SELECT edge, at FROM usage_events
		    WHERE service_id = 'svc_bill' AND meter = 'service_span' ORDER BY at`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		var prev time.Time
		for rows.Next() {
			var e string
			var at time.Time
			if err := rows.Scan(&e, &at); err != nil {
				t.Fatal(err)
			}
			if !prev.IsZero() && !at.After(prev) {
				t.Fatalf("span %q shares a timestamp with its predecessor — the [open, close] "+
					"order is then arbitrary and this assertion is a coin flip", e)
			}
			prev = at
			out = append(out, e)
		}
		return out
	}

	// provisioning → ready opens the span, and finishes generation 1.
	if _, err := svc.Writeback(ctx, "cell-0", reconcile.Report{
		ServiceID: "svc_bill", ObservedGeneration: 1, Status: "ready"}); err != nil {
		t.Fatalf("open: %v", err)
	}
	if got := spans(); len(got) != 1 || got[0] != "open" {
		t.Fatalf("spans after reaching ready = %v, want [open]", got)
	}

	// A PATCH bumps the generation, which is what makes a READY service
	// reachable by the agent again: ListDesiredForCell selects on
	// observed_generation < generation and has no status filter, and
	// UpdateServiceShape bumps for any status but `deleting`. Without this the
	// break below is unreachable through the ordinary flow AND the assertions on
	// observed_generation are answered by the value the FIRST hop already left
	// there — a green that means nothing.
	if _, err := pool.Exec(ctx,
		`UPDATE services SET generation = 2 WHERE id = 'svc_bill'`); err != nil {
		t.Fatal(err)
	}

	// The cluster breaks. The agent reports `failed`, which ADR-024 does not
	// allow from `ready`; the machine routes it to `degraded` — which still
	// BILLS, so this hop must NOT finish the generation.
	if _, err := svc.Writeback(ctx, "cell-0", reconcile.Report{
		ServiceID: "svc_bill", ObservedGeneration: 2, Status: "failed"}); !errors.Is(err, reconcile.ErrNotConverged) {
		t.Fatalf("hop 1: err = %v, want ErrNotConverged", err)
	}
	row, err := q.GetService(ctx, "svc_bill")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", row.Status)
	}
	if row.ObservedGeneration != 1 {
		t.Fatalf("observed_generation = %d, want 1 (still behind generation 2): advancing at "+
			"`degraded` drops the row out of ListDesiredForCell on a BILLING state, and nothing "+
			"observes the cluster again — it bills indefinitely", row.ObservedGeneration)
	}
	if got := spans(); len(got) != 1 {
		t.Fatalf("spans at degraded = %v — degraded still bills, so nothing may close here", got)
	}

	// Still broken on the next tick. degraded → failed is the ONLY edge that
	// closes the span, and it is reachable only because hop 1 stayed outstanding.
	if _, err := svc.Writeback(ctx, "cell-0", reconcile.Report{
		ServiceID: "svc_bill", ObservedGeneration: 2, Status: "failed"}); err != nil {
		t.Fatalf("hop 2: %v", err)
	}
	row, err = q.GetService(ctx, "svc_bill")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "failed" {
		t.Fatalf("status = %q, want failed — the span never closes", row.Status)
	}
	if row.ObservedGeneration != 2 {
		t.Fatalf("observed_generation = %d, want 2 once converged", row.ObservedGeneration)
	}
	if got := spans(); len(got) != 2 || got[1] != "close" {
		t.Fatalf("spans = %v, want [open close] — an unrecoverable database that never stops "+
			"billing is the defect this two-hop path exists to prevent", got)
	}

	// D10: both edges are on the spine, and both came from the cell converging.
	for _, action := range []string{"service.degraded", "service.failed"} {
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM events WHERE subject = 'svc_bill' AND action = $1`, action).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("expected exactly one %s spine event, got %d", action, n)
		}
	}
}

// EVERY STATUS THE MAPPING CAN EMIT IS ONE THE DATABASE ACCEPTS.
//
// ObservedStatus is a pure function over strings, so a destination outside the
// services CHECK constraint would only ever surface as a 500 at the UPDATE. This
// walks the full (from × observed) domain against the real constraint.
func TestEveryMappedDestinationSatisfiesTheRealCheckConstraint(t *testing.T) {
	pool, _ := realDB(t)
	ctx := context.Background()
	seedService(t, pool, "svc_check")

	// The vocabulary comes from the one definition, and this test is also what
	// pins that definition to the REAL CHECK constraint.
	froms := provisioning.StatusVocabulary()
	observeds := append(provisioning.StatusVocabulary(), "gone", "", "not-a-status")
	tried := 0
	for _, from := range froms {
		for _, observed := range observeds {
			to, ok := provisioning.ObservedStatus(from, observed).Edge()
			if !ok {
				continue
			}
			tried++
			if _, err := pool.Exec(ctx,
				`UPDATE services SET status = $1 WHERE id = 'svc_check'`, to); err != nil {
				t.Errorf("from %q observing %q yields %q, which the services CHECK constraint "+
					"refuses (%v) — the mapping would turn a cell report into a 500",
					from, observed, to, err)
			}
		}
	}
	// EXACT, not a floor. `if tried == 0` is the same shape as the `if checked <
	// 30` guard round 3 deleted for being unable to see what was never
	// enumerated: one surviving edge would keep it green. These are the eight
	// edges the mapping can emit — provisioning+{ready,failed}, ready+{degraded,
	// failed}, degraded+{ready,failed}, failed+{provisioning,ready}.
	if tried != 8 {
		t.Fatalf("exercised %d edges, want exactly 8 — if the mapping gained or lost an edge, "+
			"this test must be updated deliberately, not silently", tried)
	}
}

// seedEnv creates an environment in the integration project, optionally with a
// service in a given state. Returns the environment id.
func seedEnv(t *testing.T, pool *pgxpool.Pool, envID string) {
	t.Helper()
	ctx := context.Background()
	ex := func(sql string, args ...any) {
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
	}
	ex(`INSERT INTO orgs (id, name, slug) VALUES ('org_it', 'itco', 'itco') ON CONFLICT DO NOTHING`)
	ex(`INSERT INTO projects (id, org_id, name, cell_id) VALUES ('prj_it', 'org_it', 'p', 'cell-0') ON CONFLICT DO NOTHING`)
	ex(`INSERT INTO environments (id, project_id, name) VALUES ($1, 'prj_it', $1) ON CONFLICT DO NOTHING`, envID)
}

func seedEnvService(t *testing.T, pool *pgxpool.Pool, envID, svcID, status string, gen, observed int64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO services (id, env_id, name, product, status, cell_id, desired, generation, observed_generation)
		 VALUES ($1, $2, $1, 'postgres', $3, 'cell-0', '{"product":"postgres"}', $4, $5)`,
		svcID, envID, status, gen, observed); err != nil {
		t.Fatalf("seed service: %v", err)
	}
}

func scheduleEnvDeletion(t *testing.T, pool *pgxpool.Pool, envID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE environments SET deletion_scheduled_at = now() WHERE id = $1`, envID); err != nil {
		t.Fatalf("schedule deletion: %v", err)
	}
}

func advertisedEnvs(t *testing.T, svc *reconcile.Service) []string {
	t.Helper()
	st, err := svc.Desired(context.Background(), "cell-0", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range st.Environments {
		out = append(out, e.ID)
	}
	return out
}

// AN ENVIRONMENT IS NOT ADVERTISED UNTIL ITS SERVICES ARE ACTUALLY GONE.
//
// THIS IS THE SAFETY GATE, and it is the reason the whole feature is not a
// data-loss bug. Deleting a namespace deletes everything in it, so advertising
// an environment while a CNPG cluster is still terminating would destroy the
// database before US-3.5's final backup.
//
// `DeleteEnvironment` only requires every service to have REACHED `deleting`,
// which is NOT the same as torn down — so the gate lives in the query, and it
// asserts the stronger condition: status `deleting` AND observed_generation
// caught up (the cell converged the teardown and reported it).
func TestAnEnvironmentIsNotAdvertisedWhileAServiceSurvivesAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	seedEnv(t, pool, "env_gate")
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}

	// A live service. Scheduling deletion must not make the environment eligible.
	seedEnvService(t, pool, "env_gate", "svc_live", "ready", 1, 1)
	scheduleEnvDeletion(t, pool, "env_gate")
	if got := advertisedEnvs(t, svc); len(got) != 0 {
		t.Fatalf("advertised %v while a READY service is still in the environment — the "+
			"namespace teardown would delete a running database", got)
	}

	// `deleting` but NOT yet converged: the cell has been told, and has not
	// finished. This is precisely the window DeleteEnvironment's own check
	// cannot see, and the one that would destroy a terminating cluster.
	if _, err := pool.Exec(context.Background(),
		`UPDATE services SET status = 'deleting', generation = 2, observed_generation = 1
		 WHERE id = 'svc_live'`); err != nil {
		t.Fatal(err)
	}
	if got := advertisedEnvs(t, svc); len(got) != 0 {
		t.Fatalf("advertised %v while a service is mid-teardown (observed 1 < generation 2) — "+
			"the final backup has not been taken yet", got)
	}

	// Converged: the teardown is confirmed, so the environment is eligible.
	if _, err := pool.Exec(context.Background(),
		`UPDATE services SET observed_generation = 2 WHERE id = 'svc_live'`); err != nil {
		t.Fatal(err)
	}
	got := advertisedEnvs(t, svc)
	if len(got) != 1 || got[0] != "env_gate" {
		t.Fatalf("advertised %v, want [env_gate] once every service is torn down", got)
	}
}

// An environment with NO services is eligible as soon as it is scheduled — the
// gate is "nothing survives", not "something was torn down".
func TestAnEmptyEnvironmentIsAdvertisedImmediatelyAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	seedEnv(t, pool, "env_empty")
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	if got := advertisedEnvs(t, svc); len(got) != 0 {
		t.Fatalf("advertised %v before any deletion was scheduled", got)
	}
	scheduleEnvDeletion(t, pool, "env_empty")
	got := advertisedEnvs(t, svc)
	if len(got) != 1 || got[0] != "env_empty" {
		t.Fatalf("advertised %v, want [env_empty]", got)
	}
}

// THE NAMESPACE ON THE WIRE IS THE ONE THE CONTROL PLANE RESOLVED — the same
// derivation the service path uses, not a second one. US-3.3a shipped a second
// derivation and it named nothing the control plane knew.
func TestTheAdvertisedNamespaceMatchesTheResolvedOneAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	seedEnv(t, pool, "env_9f3c1a2b")
	scheduleEnvDeletion(t, pool, "env_9f3c1a2b")
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.Desired(context.Background(), "cell-0", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Environments) != 1 {
		t.Fatalf("want one environment, got %d", len(st.Environments))
	}
	if got, want := st.Environments[0].Namespace, provisioning.NamespaceForEnv("env_9f3c1a2b"); got != want {
		t.Fatalf("advertised namespace %q, want %q — the agent would delete the wrong thing "+
			"or nothing at all", got, want)
	}
	if st.Environments[0].Namespace != "env-9f3c1a2b" {
		t.Fatalf("namespace = %q, want env-9f3c1a2b (ADR-0012: sanitize(env id))",
			st.Environments[0].Namespace)
	}
}

// A CONFIRMED TEARDOWN STOPS BEING ADVERTISED, and confirming twice is a
// refusal rather than a second teardown.
func TestConfirmingAnEnvironmentTeardownIsIdempotentAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	seedEnv(t, pool, "env_conf")
	scheduleEnvDeletion(t, pool, "env_conf")
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if got := advertisedEnvs(t, svc); len(got) != 1 {
		t.Fatalf("precondition: advertised %v", got)
	}
	if err := svc.ConfirmEnvironmentTeardown(ctx, "cell-0", "env_conf"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if got := advertisedEnvs(t, svc); len(got) != 0 {
		t.Fatalf("still advertised %v after confirmation — the agent would delete it every tick", got)
	}
	var tornDown bool
	if err := pool.QueryRow(ctx,
		`SELECT torn_down_at IS NOT NULL FROM environments WHERE id = 'env_conf'`).Scan(&tornDown); err != nil {
		t.Fatal(err)
	}
	if !tornDown {
		t.Fatal("torn_down_at was not stamped")
	}
	// A replay is refused, not a silent success: the row is not outstanding.
	if err := svc.ConfirmEnvironmentTeardown(ctx, "cell-0", "env_conf"); !errors.Is(err, reconcile.ErrTeardownNotOutstanding) {
		t.Fatalf("replay: err = %v, want ErrTeardownNotOutstanding", err)
	}
	// And so is an environment nobody scheduled — a confirmation must not be
	// able to invent a teardown.
	seedEnv(t, pool, "env_unsched")
	if err := svc.ConfirmEnvironmentTeardown(ctx, "cell-0", "env_unsched"); !errors.Is(err, reconcile.ErrTeardownNotOutstanding) {
		t.Fatalf("unscheduled: err = %v, want ErrTeardownNotOutstanding", err)
	}
}

// A RECONCILER TOKEN CANNOT CONFIRM A TEARDOWN ON A CELL IT DOES NOT OWN, and
// the refusal does not leak that the environment exists.
func TestAnEnvironmentOnAnotherCellIs404AgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedEnv(t, pool, "env_far")
	scheduleEnvDeletion(t, pool, "env_far")
	// Move the project to a different cell.
	if _, err := pool.Exec(ctx, `INSERT INTO cells (id, region, status) VALUES ('cell-9', 'eu', 'active') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE projects SET cell_id = 'cell-9' WHERE id = 'prj_it'`); err != nil {
		t.Fatal(err)
	}
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	if got := advertisedEnvs(t, svc); len(got) != 0 {
		t.Fatalf("cell-0's poll returned %v from cell-9 — a cell must not see another's work", got)
	}
	if err := svc.ConfirmEnvironmentTeardown(ctx, "cell-0", "env_far"); !errors.Is(err, reconcile.ErrUnknownEnvironment) {
		t.Fatalf("err = %v, want ErrUnknownEnvironment (404, never 403 — existence is not leaked)", err)
	}
}

// A CONFIRMATION THAT ARRIVES AFTER THE ENVIRONMENT STOPPED BEING ELIGIBLE IS
// REFUSED — because `torn_down_at` is a ONE-WAY LATCH.
//
// The agent polls, then deletes, then confirms. If a service appears in that
// window, a confirmation without the poll's fence latches an environment that is
// no longer eligible: it is never advertised again, so when that service is
// later deleted its namespace leaks FOREVER — the leak this task exists to
// close, reintroduced through the back door.
//
// (`CreateService` now refuses to create into a scheduled environment, so this
// is defence in depth — but the fence is what makes the window safe rather than
// merely narrow, and the two guards fail independently.)
func TestAConfirmationIsRefusedIfTheEnvironmentBecameIneligibleAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedEnv(t, pool, "env_race")
	scheduleEnvDeletion(t, pool, "env_race")
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	// The poll sees it: nothing is inside.
	if got := advertisedEnvs(t, svc); len(got) != 1 || got[0] != "env_race" {
		t.Fatalf("precondition: advertised %v", got)
	}
	// ...and then something appears inside it, before the confirmation lands.
	seedEnvService(t, pool, "env_race", "svc_late", "provisioning", 1, 0)

	if err := svc.ConfirmEnvironmentTeardown(ctx, "cell-0", "env_race"); !errors.Is(err, reconcile.ErrTeardownNotOutstanding) {
		t.Fatalf("err = %v, want ErrTeardownNotOutstanding — latching here strands the "+
			"environment: never advertised again, namespace leaks forever", err)
	}
	var torn bool
	if err := pool.QueryRow(ctx,
		`SELECT torn_down_at IS NOT NULL FROM environments WHERE id = 'env_race'`).Scan(&torn); err != nil {
		t.Fatal(err)
	}
	if torn {
		t.Fatal("torn_down_at was latched for an environment that is no longer eligible")
	}
	// And it is not advertised while that service is alive, so the agent stops
	// asking — the state is consistent, not stuck.
	if got := advertisedEnvs(t, svc); len(got) != 0 {
		t.Fatalf("still advertised %v with a live service inside", got)
	}
}

// The teardown's completion lands on the SPINE (D10). DeleteEnvironment records
// `env.deletion_scheduled`; without this the feed shows a teardown that starts
// and never finishes.
func TestAConfirmedTeardownIsRecordedOnTheSpineAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedEnv(t, pool, "env_spine")
	scheduleEnvDeletion(t, pool, "env_spine")
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmEnvironmentTeardown(ctx, "cell-0", "env_spine"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM events WHERE subject = 'env_spine' AND action = 'env.torn_down'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected exactly one env.torn_down spine event, got %d", n)
	}
	// A refused replay must not append a second one.
	_ = svc.ConfirmEnvironmentTeardown(ctx, "cell-0", "env_spine")
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM events WHERE subject = 'env_spine' AND action = 'env.torn_down'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("a refused replay appended another spine event (%d)", n)
	}
}

// NOTHING NEW GOES INTO AN ENVIRONMENT THAT IS BEING TORN DOWN.
//
// `DeleteEnvironment` enforces "nothing live is in here" at SCHEDULE time and
// nothing preserved it afterwards, so a service created later sat inside a
// namespace the agent had been told to delete. Mirrors CreateProject's org check.
func TestCreateServiceRefusesAScheduledEnvironmentAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	seedEnv(t, pool, "env_sched")
	scheduleEnvDeletion(t, pool, "env_sched")

	env, err := q.GetEnvironment(ctx, "env_sched")
	if err != nil {
		t.Fatal(err)
	}
	if !env.DeletionScheduledAt.Valid {
		t.Fatal("precondition: deletion is not scheduled")
	}
	kek, err := secrets.NewEnvKEK("test-v1", testKEK)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := billing.Load()
	if err != nil {
		t.Fatal(err)
	}
	prov := provisioning.NewService(pool, events.NewRecorder(q, events.NewHub()),
		secrets.NewVault(q, kek), metering.NewEmitter(q), plans)

	_, err = prov.CreateService(ctx, nil, env, "org_it", provisioning.CreateServiceInput{
		Name: "db", EstimateID: "est_x",
	})
	if err == nil {
		t.Fatal("a service was created into an environment scheduled for deletion")
	}
	// It must be MY conflict, not the estimate lookup failing first — the input
	// deliberately carries a bogus estimate id, so `err != nil` alone would be
	// satisfied by an unrelated refusal a few lines further down.
	c, ok := errors.AsType[problem.Carrier](err)
	if !ok {
		t.Fatalf("not a problem-carrying error: %v", err)
	}
	p := c.Problem()
	if p.Status != 409 {
		t.Fatalf("status = %d, want 409", p.Status)
	}
	var named bool
	for _, r := range p.Reasons {
		if strings.Contains(r.Message, "scheduled for deletion") {
			named = true
		}
	}
	if !named {
		t.Fatalf("the refusal is a 409 for some OTHER reason: %+v", p.Reasons)
	}
}

// EVERY NON-`deleting` STATUS BLOCKS THE TEARDOWN — swept over the whole
// vocabulary, not the two statuses that happened to be convenient.
//
// The gate was only ever exercised against `ready` and `deleting`, and two
// mutations survived that: `NOT IN ('deleting','failed')` and
// `NOT IN ('deleting','provisioning')`. Both are real harm — a `failed` service
// still owns a PVC with customer data, and a `provisioning` one is mid-create —
// and a sibling mutation dying says nothing about the other members of the set.
//
// The list is DERIVED from the status vocabulary, so a status added to the
// machine joins this sweep automatically.
func TestEveryNonDeletingStatusBlocksTheEnvironmentTeardownAgainstRealPostgres(t *testing.T) {
	for _, status := range provisioning.StatusVocabulary() {
		if status == "deleting" {
			continue
		}
		t.Run(status, func(t *testing.T) {
			pool, q := realDB(t)
			seedEnv(t, pool, "env_sweep")
			// observed == generation: fully converged, so the ONLY thing that can
			// block the teardown is the status itself.
			seedEnvService(t, pool, "env_sweep", "svc_"+status, status, 1, 1)
			scheduleEnvDeletion(t, pool, "env_sweep")
			svc, err := newReconciler(pool, q)
			if err != nil {
				t.Fatal(err)
			}
			if got := advertisedEnvs(t, svc); len(got) != 0 {
				t.Fatalf("a %q service did not block the teardown (advertised %v) — its "+
					"namespace would be deleted with its PVC still in it", status, got)
			}
			// POSITIVE CONTROL: the same row, moved to a converged `deleting`,
			// must release it. Without this the test would pass for a gate that
			// blocks everything.
			if _, err := pool.Exec(context.Background(),
				`UPDATE services SET status = 'deleting' WHERE id = $1`, "svc_"+status); err != nil {
				t.Fatal(err)
			}
			if got := advertisedEnvs(t, svc); len(got) != 1 {
				t.Fatalf("a converged `deleting` service still blocks the teardown (advertised %v)", got)
			}
		})
	}
}

// THE ENVIRONMENT POLL RESPECTS ITS LIMIT AND ITS ORDER, and touches the
// heartbeat on a confirmation.
//
// All three were unasserted: `LIMIT $2` → `GREATEST($2,100000)`, `ORDER BY
// deletion_scheduled_at, id` → `ORDER BY id DESC`, and deleting
// TouchCellHeartbeat from ConfirmEnvironmentTeardown all survived.
func TestTheEnvironmentPollPagesAndOrdersAgainstRealPostgres(t *testing.T) {
	pool, q := realDB(t)
	ctx := context.Background()
	for _, id := range []string{"env_c", "env_a", "env_b"} {
		seedEnv(t, pool, id)
		scheduleEnvDeletion(t, pool, id)
		// Distinct schedule times, so the ORDER BY has something to order by.
		if _, err := pool.Exec(ctx,
			`UPDATE environments SET deletion_scheduled_at = now() WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
	}
	svc, err := newReconciler(pool, q)
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int32{1, 2, 100} {
		st, err := svc.Desired(ctx, "cell-0", 0, limit)
		if err != nil {
			t.Fatal(err)
		}
		want := int(limit)
		if want > 3 {
			want = 3
		}
		if len(st.Environments) != want {
			t.Fatalf("limit %d returned %d environments, want %d", limit, len(st.Environments), want)
		}
	}
	// Oldest scheduled first: the environment waiting longest is served first.
	st, err := svc.Desired(ctx, "cell-0", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if st.Environments[0].ID != "env_c" {
		t.Errorf("first environment = %q, want env_c (scheduled first) — the poll is not "+
			"ordered by how long each has been waiting", st.Environments[0].ID)
	}

	// The heartbeat rides the confirmation, as it rides the writeback.
	var before, after time.Time
	if err := pool.QueryRow(ctx, `SELECT agent_last_seen_at FROM cells WHERE id='cell-0'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmEnvironmentTeardown(ctx, "cell-0", "env_c"); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT agent_last_seen_at FROM cells WHERE id='cell-0'`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !after.After(before) {
		t.Error("a confirmation did not touch the cell heartbeat")
	}
}
