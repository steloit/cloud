-- T3.6: bindings. Credentials are in Secrets via secret_ref, never in rows.

-- name: InsertBinding :one
INSERT INTO bindings (id, source_id, target_type, target_id, intent, scope, secret_ref)
VALUES ($1, $2, 'service', $3, $4, $5, $6)
RETURNING *;

-- name: GetBinding :one
SELECT * FROM bindings WHERE id = $1;

-- name: ListBindingsForSource :many
SELECT * FROM bindings WHERE source_id = $1 AND status <> 'revoked' ORDER BY created_at;

-- name: RevokeBinding :one
UPDATE bindings SET status = 'revoked', rotated_at = now()
WHERE id = $1 AND status <> 'revoked'
RETURNING *;

-- name: ActiveBindingsToTarget :many
-- U6 dependents: services that will knowingly break if the target dies.
SELECT b.id, b.source_id, s.name AS source_name
FROM bindings b JOIN services s ON s.id = b.source_id
WHERE b.target_id = $1 AND b.status <> 'revoked';
