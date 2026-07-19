-- T4.3: deployments. number assignment locks the env row's history via the
-- unique (env_id, number) — the insert retries on collision in code.

-- name: NextDeploymentNumber :one
SELECT coalesce(max(number), 0)::int + 1 FROM deployments WHERE env_id = $1;

-- name: InsertDeployment :one
INSERT INTO deployments (id, number, env_id, service_id, git_sha, actor, promoted_from, rollback_of)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetDeployment :one
SELECT * FROM deployments WHERE id = $1;

-- name: ListDeploymentsForEnv :many
SELECT * FROM deployments WHERE env_id = $1 ORDER BY number DESC;

-- name: PreviousDeployment :one
-- The most recent EARLIER deployment of the same service that carried an
-- image (reached live or was itself later rolled back — i.e. it worked).
SELECT * FROM deployments
WHERE env_id = $1 AND service_id = $2 AND number < $3
  AND state IN ('live', 'rolled_back')
ORDER BY number DESC
LIMIT 1;

-- name: SetDeploymentState :one
UPDATE deployments SET state = $3 WHERE id = $1 AND state = $2 RETURNING *;
