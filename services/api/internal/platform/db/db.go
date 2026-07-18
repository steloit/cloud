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
