package db_test

import (
	"context"
	"strings"
	"testing"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/steloit/cloud/services/api/internal/platform/db"
)

// runPG starts a throwaway server at the given image and returns its URL.
func runPG(t *testing.T, image string) string {
	t.Helper()
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx, image,
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
}
