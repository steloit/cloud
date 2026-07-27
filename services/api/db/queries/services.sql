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

-- name: ListExpiredOverrides :many
-- D22: pins past their 24h expiry. SELECT only — the caller computes the new
-- desired doc and base price, then clears everything in ONE statement, because
-- a clear that commits before the doc is rewritten leaves the row outstanding
-- with a stale pinned doc: the cell polls it, renders the pin, and MarkObserved
-- succeeds — after which nothing bumps generation again and the pin renders
-- forever.
--
-- A pin with NO expires_at is expired BY DEFINITION: "unset" must not mean
-- "forever", and such a row is otherwise unreachable by any sweep.
--
-- The timestamp cast is GUARDED: one row with a malformed expires_at would
-- otherwise abort the whole statement, and the sweep would then fail on every
-- tick, silently, for every customer.
SELECT * FROM services
WHERE override IS NOT NULL
  AND status <> 'deleting'
  AND (
        (override->>'expires_at') IS NULL
     OR (override->>'expires_at') !~ '^\d{4}-\d{2}-\d{2}T'
     OR ((override->>'expires_at') ~ '^\d{4}-\d{2}-\d{2}T'
         AND (override->>'expires_at')::timestamptz <= now())
  );

-- name: ClearExpiredOverride :one
-- Clear the pin, restore the unpinned price, rewrite desired, and bump
-- generation — ATOMICALLY. Fenced on `override IS NOT NULL` so a concurrent
-- edit that already cleared it is a no-op rather than a double bump.
UPDATE services SET
    override = NULL,
    monthly_estimate_cents = $2,
    desired = $3,
    generation = generation + 1
WHERE id = $1 AND override IS NOT NULL AND status <> 'deleting'
RETURNING *;

