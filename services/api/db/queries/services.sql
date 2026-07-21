-- T3.3: services — desired state. Status writes go through the guarded
-- machine in Go; SetServiceStatus enforces the FROM state in SQL too.

-- name: InsertService :one
-- desired is the reconciler's input (US-1.3a): populated at creation from the
-- shape so the agent has a real document to render from. generation defaults to
-- 1, observed_generation to 0, so a fresh service is immediately outstanding.
INSERT INTO services (id, env_id, name, product, intent, shape, scaling, provisioning_steps, monthly_estimate_cents, estimate_id, desired)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetService :one
SELECT * FROM services WHERE id = $1;

-- name: MarkServiceDeleting :one
-- Delete's desired-state half (US-1.3a): set the deleting desired doc and bump
-- generation so the service becomes outstanding and the cell converges the
-- teardown, reporting gone. The status transition to 'deleting' is separate
-- (SetServiceStatus, via the guarded machine).
UPDATE services SET desired = $2, generation = generation + 1
WHERE id = $1
RETURNING *;

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
WHERE id = $1
RETURNING *;

-- name: OrgForService :one
SELECT p.org_id FROM services s
JOIN environments e ON e.id = s.env_id
JOIN projects p ON p.id = e.project_id
WHERE s.id = $1;
