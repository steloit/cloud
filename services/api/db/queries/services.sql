-- T3.3: services — desired state. Status writes go through the guarded
-- machine in Go; SetServiceStatus enforces the FROM state in SQL too.

-- name: InsertService :one
INSERT INTO services (id, env_id, name, product, intent, shape, scaling, provisioning_steps, monthly_estimate_cents, estimate_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
UPDATE services SET
    shape = coalesce(sqlc.narg('shape'), shape),
    scaling = coalesce(sqlc.narg('scaling'), scaling),
    override = sqlc.narg('override'),
    monthly_estimate_cents = coalesce(sqlc.narg('monthly_estimate_cents'), monthly_estimate_cents)
WHERE id = $1
RETURNING *;

-- name: OrgForService :one
SELECT p.org_id FROM services s
JOIN environments e ON e.id = s.env_id
JOIN projects p ON p.id = e.project_id
WHERE s.id = $1;
