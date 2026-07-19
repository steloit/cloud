-- T12.4: templates (ADR-021 frozen copies).

-- name: InsertTemplate :one
INSERT INTO templates (id, org_id, name, visibility, source_project, source_env, contents, required_inputs, monthly_estimate_cents, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetTemplate :one
SELECT * FROM templates WHERE id = $1;

-- name: ListTemplates :many
SELECT * FROM templates WHERE org_id = $1 ORDER BY updated_at DESC;

-- name: UpdateTemplate :one
-- version increments on every edit; contents NULL = keep (coalesce).
UPDATE templates SET
    name = coalesce(sqlc.narg('name'), name),
    visibility = coalesce(sqlc.narg('visibility'), visibility),
    contents = coalesce(sqlc.narg('contents'), contents),
    required_inputs = coalesce(sqlc.narg('required_inputs'), required_inputs),
    monthly_estimate_cents = coalesce(sqlc.narg('monthly_estimate_cents'), monthly_estimate_cents),
    version = version + 1,
    updated_by = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteTemplate :execrows
-- ADR-021: deleting the recipe doesn't touch the meals — nothing references
-- templates, so this is a plain delete, never a cascade into instantiations.
DELETE FROM templates WHERE id = $1 AND org_id = $2;

-- name: BumpTemplateUsage :exec
UPDATE templates SET used_by_count = used_by_count + 1 WHERE id = $1;

-- name: ProjectByName :one
SELECT * FROM projects WHERE org_id = $1 AND name = $2 AND deletion_scheduled_at IS NULL;

-- name: EnvByName :one
SELECT * FROM environments WHERE project_id = $1 AND name = $2;

-- name: ListBindingsForEnv :many
-- capture input: every binding whose SOURCE service lives in the env.
SELECT b.* FROM bindings b
JOIN services s ON s.id = b.source_id
WHERE s.env_id = $1 AND b.status <> 'revoked';

-- name: DeleteBindingsForProject :exec
-- Instantiation COMPENSATION only (T12.4): bindings RESTRICT service deletion,
-- so a failed half-instantiation clears them before the project cascade.
DELETE FROM bindings b USING services s, environments e
WHERE b.source_id = s.id AND s.env_id = e.id AND e.project_id = $1;

-- name: HardDeleteProject :exec
-- Instantiation COMPENSATION only: cascade removes envs -> services -> secrets.
-- Normal deletion is the scheduled soft path — never this.
DELETE FROM projects WHERE id = $1;
