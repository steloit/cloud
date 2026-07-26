-- T4.1: GitHub integration.

-- name: UpsertInstallation :one
INSERT INTO github_installations (id, org_id, installation_id, account_login)
VALUES ($1, $2, $3, $4)
ON CONFLICT (installation_id) DO UPDATE SET deleted_at = NULL, account_login = EXCLUDED.account_login
RETURNING *;

-- name: MarkInstallationDeleted :execrows
UPDATE github_installations SET deleted_at = now()
WHERE installation_id = $1 AND deleted_at IS NULL;

-- name: OrgForInstallation :one
SELECT org_id FROM github_installations WHERE installation_id = $1 AND deleted_at IS NULL;

-- name: CreateRepoLink :one
INSERT INTO repo_links (id, org_id, service_id, repo, branch)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: LinksForRepo :many
SELECT * FROM repo_links WHERE org_id = $1 AND repo = $2;

-- name: InsertDelivery :one
-- ON CONFLICT DO NOTHING makes redelivery idempotent; zero rows = duplicate.
INSERT INTO github_deliveries (id, delivery_id, event, action, repo, payload)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (delivery_id) DO NOTHING
RETURNING *;
