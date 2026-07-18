-- T3.5: secrets — internal-only service; rows carry ciphertext, never
-- plaintext. Reads take the latest version for a scope+name.

-- name: InsertSecret :one
INSERT INTO secrets (id, org_id, project_id, env_id, name, version, ciphertext, nonce, wrapped_dek, dek_nonce, kek_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: LatestSecret :one
SELECT * FROM secrets
WHERE org_id = $1
  AND project_id IS NOT DISTINCT FROM sqlc.narg('project_id')::text
  AND env_id IS NOT DISTINCT FROM sqlc.narg('env_id')::text
  AND name = $2
ORDER BY version DESC
LIMIT 1;

-- name: LatestSecretVersion :one
SELECT coalesce(max(version), 0)::int FROM secrets
WHERE org_id = $1
  AND project_id IS NOT DISTINCT FROM sqlc.narg('project_id')::text
  AND env_id IS NOT DISTINCT FROM sqlc.narg('env_id')::text
  AND name = $2;

-- name: DeleteSecret :execrows
-- All versions of a scope+name (consumed by unbind/rotation flows).
DELETE FROM secrets
WHERE org_id = $1
  AND project_id IS NOT DISTINCT FROM sqlc.narg('project_id')::text
  AND env_id IS NOT DISTINCT FROM sqlc.narg('env_id')::text
  AND name = $2;
