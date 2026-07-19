-- name: CreateDashboard :one
INSERT INTO dashboards (id, org_id, name, scope, visibility, owner_id, layout)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetDashboard :one
SELECT * FROM dashboards WHERE id = $1;

-- name: ListDashboardsForOrg :many
-- Visibility is applied in the handler (owner-aware); this returns the org's
-- dashboards newest-first, optionally filtered to one scope string.
SELECT * FROM dashboards
WHERE org_id = $1 AND ($2::text = '' OR scope = $2)
ORDER BY created_at DESC
LIMIT $3;

-- name: UpdateDashboard :one
UPDATE dashboards
SET name = COALESCE(sqlc.narg('name'), name),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    layout = COALESCE(sqlc.narg('layout'), layout)
WHERE id = $1
RETURNING *;

-- name: DeleteDashboard :exec
DELETE FROM dashboards WHERE id = $1;

-- name: AddWidget :one
INSERT INTO dashboard_widgets (id, dashboard_id, source, query, viz, pos)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListWidgetsForDashboard :many
SELECT * FROM dashboard_widgets WHERE dashboard_id = $1 ORDER BY created_at;

-- name: GetWidget :one
SELECT * FROM dashboard_widgets WHERE id = $1 AND dashboard_id = $2;

-- name: DeleteWidget :exec
DELETE FROM dashboard_widgets WHERE id = $1 AND dashboard_id = $2;
