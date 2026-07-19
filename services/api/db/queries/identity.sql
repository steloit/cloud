-- name: CreateUser :one
INSERT INTO users (id, email, password_hash, name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower($1);

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (id, user_id, token_hash, device, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetActiveSessionByTokenHash :one
SELECT * FROM sessions
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now() WHERE id = $1;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: CountActiveSessionsForUser :one
SELECT count(*) FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: CreateToken :one
INSERT INTO tokens (id, kind, user_id, org_id, name, scope, prefix, token_hash, expires_at, permissions)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListPersonalTokens :many
SELECT * FROM tokens
WHERE kind = 'personal' AND user_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: GetActiveTokenByHash :one
SELECT * FROM tokens
WHERE token_hash = $1 AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now());

-- name: RevokePersonalToken :execrows
UPDATE tokens SET revoked_at = now()
WHERE id = $1 AND user_id = $2 AND kind = 'personal' AND revoked_at IS NULL;

-- name: TouchTokenUsed :exec
UPDATE tokens SET last_used_at = now() WHERE id = $1;

-- name: AddMember :one
INSERT INTO members (id, org_id, user_id, role) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetMemberRole :one
SELECT role FROM members WHERE org_id = $1 AND user_id = $2;

-- name: ListPoliciesForOrg :many
SELECT * FROM policies WHERE org_id = $1;

-- name: ListActiveSessionsForUser :many
SELECT * FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
ORDER BY last_seen_at DESC;

-- name: RevokeSessionOwned :execrows
-- Owner-scoped: revoking another user's session id is indistinguishable
-- from a missing one (404).
UPDATE sessions SET revoked_at = now()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- name: OwnedSoleOwnerOrgs :many
-- Orgs this user owns where removing them would leave ZERO live owners — i.e.
-- no OTHER owner whose account isn't itself scheduled for deletion. A
-- deletion-scheduled co-owner does not keep an org alive (their membership is
-- on its way out), so it can't unblock this user's deletion (orphan guard).
SELECT o.id, o.name FROM orgs o
JOIN members m ON m.org_id = o.id AND m.user_id = $1 AND m.role = 'owner'
WHERE NOT EXISTS (
    SELECT 1 FROM members m2
    JOIN users u2 ON u2.id = m2.user_id
    WHERE m2.org_id = o.id AND m2.role = 'owner'
      AND m2.user_id <> $1
      AND u2.deletion_scheduled_at IS NULL
);


-- name: ScheduleAccountDeletion :execrows
UPDATE users SET deletion_scheduled_at = now()
WHERE id = $1 AND deletion_scheduled_at IS NULL;

-- name: RemoveOwnMembership :one
DELETE FROM members WHERE org_id = $1 AND user_id = $2 RETURNING *;

-- name: OrgsForMember :many
SELECT org_id FROM members WHERE user_id = $1;

-- name: RevokeOrgKey :execrows
-- Org-scoped: revoking a key from another org's id is a 404 (no cross-org).
UPDATE tokens SET revoked_at = now()
WHERE id = $1 AND org_id = $2 AND kind = 'org' AND revoked_at IS NULL;
