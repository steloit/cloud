-- The events spine (T2.5). Keyset pagination over (at, id) — offset drifts
-- under append. Every query is org-fenced; cross-org cursors yield nothing.

-- name: AppendEvent :one
INSERT INTO events (id, org_id, kind, via, actor, action, subject, detail)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListOrgEventsDesc :many
SELECT * FROM events
WHERE org_id = $1
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'))
  AND (sqlc.narg('actor')::text IS NULL OR actor = sqlc.narg('actor'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('before_at')::timestamptz IS NULL OR (at, id) < (sqlc.narg('before_at')::timestamptz, sqlc.narg('before_id')::text))
ORDER BY at DESC, id DESC
LIMIT $2;

-- name: ListOrgEventsAsc :many
-- SSE replay: everything strictly AFTER the cursor, oldest first.
SELECT * FROM events
WHERE org_id = $1
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'))
  AND (sqlc.narg('after_at')::timestamptz IS NULL OR (at, id) > (sqlc.narg('after_at')::timestamptz, sqlc.narg('after_id')::text))
ORDER BY at ASC, id ASC
LIMIT $2;
