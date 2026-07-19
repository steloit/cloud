-- T3.1: estimates. Acceptance (createService, T3.3) marks accepted_at and
-- checks env match + liveness — the estimate-before-provision law.

-- name: InsertEstimate :one
INSERT INTO estimates (id, org_id, env_id, services, lines, total_cents, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetEstimate :one
SELECT * FROM estimates WHERE id = $1;

-- name: AcceptEstimate :one
-- One-shot: only a live, unaccepted, env-matching estimate flips.
UPDATE estimates SET accepted_at = now()
WHERE id = $1 AND env_id = $2 AND accepted_at IS NULL AND expires_at > now()
RETURNING *;
