-- T2.7: org / member / invite governance. Slug is immutable — no query ever
-- updates it. Invite expiry is computed against expires_at, never cron-flipped.

-- name: CreateOrgFull :one
INSERT INTO orgs (id, slug, name, home_region)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateSubscription :one
INSERT INTO subscriptions (org_id) VALUES ($1) RETURNING *;

-- name: GetOrg :one
SELECT * FROM orgs WHERE id = $1;

-- name: ListOrgsForUser :many
SELECT o.* FROM orgs o
JOIN members m ON m.org_id = o.id
WHERE m.user_id = $1
ORDER BY o.created_at;

-- name: UpdateOrg :one
UPDATE orgs SET
    name = coalesce(sqlc.narg('name'), name),
    home_region = coalesce(sqlc.narg('home_region'), home_region)
WHERE id = $1
RETURNING *;

-- name: ScheduleOrgDeletion :execrows
UPDATE orgs SET deletion_scheduled_at = now()
WHERE id = $1 AND deletion_scheduled_at IS NULL;

-- name: ListMembers :many
SELECT m.id, m.org_id, m.user_id, m.role, m.created_at, u.email, u.name
FROM members m JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1
ORDER BY m.created_at;

-- name: GetMember :one
SELECT m.id, m.org_id, m.user_id, m.role, m.created_at, u.email, u.name
FROM members m JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1 AND m.id = $2;

-- name: CountMembers :one
SELECT count(*) FROM members WHERE org_id = $1;

-- name: UpdateMemberRole :one
UPDATE members SET role = $3 WHERE org_id = $1 AND id = $2 RETURNING *;

-- name: RemoveMember :one
DELETE FROM members WHERE org_id = $1 AND id = $2 RETURNING *;

-- name: IsMemberEmail :one
SELECT EXISTS (
    SELECT 1 FROM members m JOIN users u ON u.id = m.user_id
    WHERE m.org_id = $1 AND lower(u.email) = lower($2)
);

-- name: CreateInvite :one
INSERT INTO invites (id, org_id, email, role, inviter_id, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListInvites :many
SELECT * FROM invites WHERE org_id = $1 ORDER BY created_at DESC;

-- name: GetInvite :one
SELECT * FROM invites WHERE id = $1;

-- name: SetInviteStatus :execrows
UPDATE invites SET status = $2 WHERE id = $1 AND status = 'pending';

-- name: GetPendingInviteByEmail :one
SELECT * FROM invites
WHERE org_id = $1 AND lower(email) = lower($2) AND status = 'pending';

-- name: ListOrgTokens :many
SELECT * FROM tokens
WHERE kind = 'org' AND org_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- G6 removal semantics (design spec): "sessions + tokens revoked
-- immediately"; G8: "if the org removes you, every personal token dies with
-- the membership."

-- name: RevokeAllSessionsForUser :exec
UPDATE sessions SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: RevokeAllPersonalTokensForUser :exec
UPDATE tokens SET revoked_at = now()
WHERE kind = 'personal' AND user_id = $1 AND revoked_at IS NULL;
