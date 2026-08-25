package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/steloit/cloud/services/api/internal/platform/db"
	"github.com/steloit/cloud/services/api/internal/platform/testenv"
)

// runPG starts a throwaway server at the given image and returns its URL.
func runPG(t *testing.T, image string) string {
	t.Helper()
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx, image,
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
	// O34: prove this port is served by THIS container's Postgres before anything
	// uses it. Under colima the VM picks the host port and nothing reserves it, so
	// a long-lived macOS listener can already own it — measured: rapportd holding
	// *:54167 since Aug 17 answered one of our containers with `01 00 00 00`.
	testenv.RequirePostgresPeer(t, url)
	return url
}

// The PostgreSQL 16 floor, asserted against a real PostgreSQL 15 — because a
// floor stated only in a comment is not a floor.
//
// US-3.8's override-expiry sweep uses pg_input_is_valid (16+). On an older
// server that query fails on every tick, the sweeper logs an error and loops,
// NO manual pin anywhere expires, and the only symptom is a log line nobody is
// watching. That is the exact silent-permanent-failure shape this task exists
// to remove, so Connect refuses to start rather than run degraded.
//
// Both directions are asserted: without the accept case, a Connect that refused
// every server would satisfy the refuse case.
func TestConnectEnforcesThePostgres16Floor(t *testing.T) {
	t.Run("refuses 15, and says what is required", func(t *testing.T) {
		pool, err := db.Connect(context.Background(), runPG(t, "postgres:15-alpine"))
		if err == nil {
			pool.Close()
			t.Fatal("Connect accepted PostgreSQL 15 — the override-expiry sweep would fail on every tick and no pin would ever expire, with a log line as the only symptom")
		}
		// A refusal hands the caller NOTHING. Two arms return an error here — the
		// version query failing and the version being too low — and only the
		// first asserted this, so `return pool, err` on the second arm survived.
		if pool != nil {
			t.Fatal("Connect returned a live pool alongside a refusal; the caller sees an error and leaks a pool it was never told about")
		}
		// An operator reading this at boot needs to know what to change, so the
		// refusal names the requirement rather than just failing.
		if !strings.Contains(err.Error(), "PostgreSQL 16") {
			t.Fatalf("the refusal must name the required version, got: %v", err)
		}
	})

	t.Run("accepts 16", func(t *testing.T) {
		pool, err := db.Connect(context.Background(), runPG(t, "postgres:16-alpine"))
		if err != nil {
			t.Fatalf("Connect rejected PostgreSQL 16, which IS the floor: %v", err)
		}
		pool.Close()
	})

	// The FLOOR itself, not "some 16.x". Without this, `< 160000` can be
	// weakened to `<= 160000` — refusing PostgreSQL 16.0 exactly — and both
	// other subtests stay green, because the images they run are 16.14 and 15.
	// Unreachable in production today (the CNPG manifest pins 16.6), but a
	// boundary nothing asserts is a boundary that drifts.
	t.Run("accepts exactly 16.0, the floor", func(t *testing.T) {
		pool, err := db.Connect(context.Background(), runPG(t, "postgres:16.0-alpine"))
		if err != nil {
			t.Fatalf("Connect rejected PostgreSQL 16.0, which is exactly the floor: %v", err)
		}
		pool.Close()
	})
}

// A server that cannot ANSWER the version question is refused too.
//
// The floor's failure mode has two arms, and only one of them was covered: the
// comparison, and the query. Making the version query's error path return the
// pool anyway (`return pool, nil`) left the whole suite green — so a database
// that cannot report its version would have booted unchecked, which is the same
// silent degradation the floor exists to prevent. An unverifiable floor is not
// a satisfied floor.
func TestConnectRefusesAServerThatCannotReportItsVersion(t *testing.T) {
	url := runPG(t, "postgres:16-alpine")
	ctx := context.Background()

	// Take current_setting away from a non-superuser and connect as them.
	// Superusers bypass ACLs, so the role has to be ordinary.
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE ROLE lowly LOGIN PASSWORD 'lowly'`,
		`GRANT CONNECT ON DATABASE app TO lowly`,
		`REVOKE EXECUTE ON FUNCTION pg_catalog.current_setting(text) FROM PUBLIC`,
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			admin.Close()
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	admin.Close()

	lowURL := strings.Replace(url, "://app:app@", "://lowly:lowly@", 1)
	if lowURL == url {
		t.Fatalf("could not rewrite the connection string for the restricted role: %s", url)
	}
	pool, err := db.Connect(ctx, lowURL)
	if err == nil {
		pool.Close()
		t.Fatal("Connect accepted a server whose version it could not read — the floor is unverified, and an unverified floor is not a satisfied floor")
	}
	if pool != nil {
		t.Fatal("Connect returned a live pool alongside an error; the caller will leak it")
	}
	if !strings.Contains(err.Error(), "version check") {
		t.Fatalf("the refusal must say the version check failed, got: %v", err)
	}
}
