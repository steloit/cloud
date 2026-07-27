// Package db owns the pgx pool and migrations (architecture §11: sqlc + pgx +
// golang-migrate; timestamped versions, never edited after apply). Migrations
// are embedded so the binary is self-migrating (alpha posture).
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	// The control database's floor is PostgreSQL 16, and it is asserted HERE
	// because a floor stated only in a comment is not a floor.
	//
	// US-3.8's override-expiry sweep uses pg_input_is_valid (16+). On 15 that
	// query fails on every tick with "function does not exist", the sweeper logs
	// an error and loops, no manual pin anywhere ever expires, and the only
	// symptom is a log line nobody is watching — a silent, permanent loss of the
	// property that makes a "temporary" pin temporary. Refusing to start is the
	// correct response to a database that cannot run our queries: a boot failure
	// is loud, and this class of degradation is not.
	// current_setting()::int, not SHOW: SHOW returns text, and scanning it into
	// an int fails on every server — which would have made this guard reject
	// PostgreSQL 16 too, i.e. refuse to start at all. Caught by the accept half
	// of TestConnectEnforcesThePostgres16Floor, which exists for exactly this.
	var version int
	if err := pool.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db version check: %w", err)
	}
	if version < 160000 {
		pool.Close()
		return nil, fmt.Errorf("db: PostgreSQL 16 or newer is required (server reports %d); "+
			"the override-expiry sweep needs pg_input_is_valid and would fail silently on this server", version)
	}
	return pool, nil
}

// Migrate applies pending migrations. Accepts a postgres:// url and rewrites
// the scheme for the pgx/v5 migrate driver.
func Migrate(url string) error {
	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("migrate source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, pgxURL(url))
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func pgxURL(url string) string {
	const pg = "postgres://"
	if len(url) > len(pg) && url[:len(pg)] == pg {
		return "pgx5://" + url[len(pg):]
	}
	return url
}
