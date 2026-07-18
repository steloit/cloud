-- T3.2: projects + environments. cell_id never leaves the store layer (D8).

-- name: CreateProject :one
INSERT INTO projects (id, org_id, name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: ListProjectsForOrg :many
SELECT p.*,
       (SELECT count(*) FROM environments e WHERE e.project_id = p.id) AS env_count
FROM projects p
WHERE p.org_id = $1
ORDER BY p.created_at;

-- name: CountProjects :one
SELECT count(*) FROM projects WHERE org_id = $1 AND deletion_scheduled_at IS NULL;

-- name: RenameProject :one
UPDATE projects SET name = $2 WHERE id = $1 RETURNING *;

-- name: ScheduleProjectDeletion :execrows
UPDATE projects SET deletion_scheduled_at = now()
WHERE id = $1 AND deletion_scheduled_at IS NULL;

-- name: CreateEnvironment :one
INSERT INTO environments (id, project_id, name, region_override, kind)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListEnvironments :many
SELECT * FROM environments WHERE project_id = $1 ORDER BY created_at;

-- name: CountEnvironments :one
SELECT count(*) FROM environments WHERE project_id = $1;

-- name: GetEnvironment :one
SELECT * FROM environments WHERE id = $1;

-- name: OrgForEnvironment :one
-- The events EnvResolver seam (T2.5) closes with this join.
SELECT p.org_id FROM environments e
JOIN projects p ON p.id = e.project_id
WHERE e.id = $1;
