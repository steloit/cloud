-- T3.7: raw metering (D10). Append-only; rollup is T6.3.

-- O38: ON CONFLICT DO NOTHING, not an error. A caller retrying a metering edge is
-- behaving correctly — that is the whole point of the key — so a duplicate must be
-- a no-op it can treat as success, or the retry path becomes error handling and
-- nobody writes it. :execrows lets the caller tell "inserted" from "already there"
-- without either being a failure.
-- name: InsertUsageEvent :execrows
INSERT INTO usage_events (id, dedupe_key, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, quantity, detail)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (dedupe_key) DO NOTHING;

-- name: ListUsageEventsForOrg :many
SELECT * FROM usage_events WHERE org_id = $1 ORDER BY at;
