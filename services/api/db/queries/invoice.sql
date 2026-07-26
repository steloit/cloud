-- T11.3: invoices.

-- name: UpsertInvoiceForPeriod :one
-- Idempotent monthly close: one invoice per (org, period). A re-close returns
-- the EXISTING row unchanged (ON CONFLICT DO NOTHING → then the caller reads it)
-- — the meter is frozen once, never double-billed.
INSERT INTO invoices (id, org_id, period, status, total_cents, lines, tax, closed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (org_id, period) DO NOTHING
RETURNING *;

-- name: GetInvoiceForPeriod :one
SELECT * FROM invoices WHERE org_id = $1 AND period = $2;

-- name: ListInvoices :many
SELECT * FROM invoices WHERE org_id = $1 ORDER BY period DESC;

-- name: SetInvoiceStatus :one
UPDATE invoices SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;
