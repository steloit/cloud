package notify

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// PoolTxer is the production Txer: a pgx pool transaction exposed as a Store.
type PoolTxer struct{ pool *pgxpool.Pool }

// NewPoolTxer adapts a connection pool to the Txer the bell projection needs.
func NewPoolTxer(pool *pgxpool.Pool) PoolTxer { return PoolTxer{pool: pool} }

// WithTx runs fn in a transaction, committing on nil and rolling back on any
// error — including errClaimedElsewhere, where rolling back is the point.
func (p PoolTxer) WithTx(ctx context.Context, fn func(Store) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback after a successful Commit is a no-op, so this is safe as the
	// single unwind path for every branch below.
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(store.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
