-- T3.7: raw metering (D10). Append-only; rollup is T6.3.

-- name: InsertUsageEvent :one
INSERT INTO usage_events (id, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, quantity, detail)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListUsageEventsForOrg :many
SELECT * FROM usage_events WHERE org_id = $1 ORDER BY at;
