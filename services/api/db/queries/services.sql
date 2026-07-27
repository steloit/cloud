-- T3.3: services — desired state. Status writes go through the guarded
-- machine in Go; SetServiceStatus enforces the FROM state in SQL too.

-- name: InsertService :one
-- desired is the reconciler's input (US-1.3a): populated at creation from the
-- shape so the agent has a real document to render from. generation defaults to
-- 1, observed_generation to 0, so a fresh service is immediately outstanding.
INSERT INTO services (id, env_id, name, product, intent, shape, scaling, provisioning_steps, monthly_estimate_cents, estimate_id, desired)
VALUES (
    sqlc.arg('id'), sqlc.arg('env_id'), sqlc.arg('name'), sqlc.arg('product'), sqlc.arg('intent'),
    sqlc.arg('shape'), sqlc.arg('scaling'), sqlc.arg('provisioning_steps'),
    sqlc.arg('monthly_estimate_cents'), sqlc.arg('estimate_id'), coalesce(sqlc.narg('desired'), '{}'::jsonb)
)
RETURNING *;

-- name: GetService :one
SELECT * FROM services WHERE id = $1;

-- name: ListServicesForEnv :many
SELECT * FROM services WHERE env_id = $1 ORDER BY created_at;

-- name: CountServicesForEnvs :one
SELECT count(*) FROM services s
JOIN environments e ON e.id = s.env_id
WHERE e.project_id = $1 AND s.status <> 'deleting';

-- name: SetServiceStatus :one
UPDATE services SET status = $3, provisioning_steps = coalesce(sqlc.narg('steps'), provisioning_steps)
WHERE id = $1 AND status = $2
RETURNING *;

-- name: UpdateServiceShape :one
-- A desired-state edit (US-1.3a): rewrite desired and BUMP generation so the
-- service becomes outstanding (observed_generation < generation) and the cell
-- re-reconciles. Without the bump a converged service would never see the edit.
UPDATE services SET
    shape = coalesce(sqlc.narg('shape'), shape),
    scaling = coalesce(sqlc.narg('scaling'), scaling),
    override = sqlc.narg('override'),
    monthly_estimate_cents = coalesce(sqlc.narg('monthly_estimate_cents'), monthly_estimate_cents),
    desired = coalesce(sqlc.narg('desired'), desired),
    generation = generation + 1
WHERE id = $1 AND status <> 'deleting'
RETURNING *;

-- name: OrgForService :one
SELECT p.org_id FROM services s
JOIN environments e ON e.id = s.env_id
JOIN projects p ON p.id = e.project_id
WHERE s.id = $1;

-- name: ExpireManualOverrides :many
-- D22: a manual instance-pin auto-expires in 24h. Clearing it must BUMP
-- generation, or the cell keeps rendering the pinned count forever — the doc is
-- otherwise only rebuilt when someone edits the service, and a pin nobody
-- touches again would be permanent.
--
-- `desired` is rewritten by the caller (it owns the doc grammar); this returns
-- the rows so it can.
UPDATE services SET
    override = NULL,
    generation = generation + 1
WHERE override IS NOT NULL
  AND status <> 'deleting'
  AND (override->>'expires_at') IS NOT NULL
  AND (override->>'expires_at')::timestamptz <= now()
RETURNING *;

-- name: SetServiceDesired :one
-- Rewrite the desired doc WITHOUT bumping generation: the caller
-- (RunOverrideExpiry) already bumped it when it cleared the pin, and bumping
-- twice would leave the row outstanding after the cell converged.
UPDATE services SET desired = $2
WHERE id = $1 AND status <> 'deleting'
RETURNING *;

-- name: SetServiceMonthlyEstimate :one
-- Restore the unpinned price when a manual pin expires. No generation bump:
-- ExpireManualOverrides already bumped it when it cleared the pin.
UPDATE services SET monthly_estimate_cents = $2
WHERE id = $1 AND status <> 'deleting'
RETURNING *;
