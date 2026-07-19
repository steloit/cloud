-- T12.1: policy authoring (CRUD + versioning). Evaluation queries live in
-- identity.sql (ListPoliciesForOrg); these are the authoring surface.

-- name: InsertPolicy :one
INSERT INTO policies (id, org_id, project_id, key, enforcement, rule, config, description, enforce_from, version, last_change_event)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1, $10)
RETURNING *;

-- name: GetPolicyByID :one
SELECT * FROM policies WHERE id = $1;

-- name: ListPoliciesPage :many
SELECT * FROM policies WHERE org_id = $1 ORDER BY created_at DESC, id;

-- name: UpdatePolicyRow :one
-- Bumps the version and stamps the audit link; the caller writes the matching
-- policy_versions history row in the same transaction.
UPDATE policies SET
    enforcement = $2,
    rule = $3,
    config = $4,
    description = $5,
    enforce_from = $6,
    version = version + 1,
    last_change_event = $7,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: InsertPolicyVersion :exec
INSERT INTO policy_versions (id, policy_id, version, key, enforcement, rule, config, description, enforce_from, change_event, changed_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ListPolicyVersions :many
SELECT * FROM policy_versions WHERE policy_id = $1 ORDER BY version DESC;

-- name: CountSameKeyPolicies :one
-- Conflict detection: a policy of the same key at the EXACT same scope (the
-- unique constraint's granularity). A project-scoped policy does NOT conflict
-- with an org-wide one of the same key — closest-wins is the intended override.
SELECT count(*) FROM policies
WHERE org_id = $1 AND key = $2
  AND project_id IS NOT DISTINCT FROM sqlc.narg('project_id');

-- name: ListSameKeyPolicies :many
-- For the dry-run preview: same-key policies at the same OR org-wide scope, so
-- the user sees what already governs (informational, broader than the 409).
SELECT * FROM policies
WHERE org_id = $1 AND key = $2
  AND (project_id IS NULL OR project_id IS NOT DISTINCT FROM sqlc.narg('project_id'));

-- name: CountOrgMembers :one
SELECT count(*) FROM members WHERE org_id = $1;

-- name: GetProjectOrg :one
-- Cross-org isolation: a policy's project_id must belong to the policy's org.
SELECT org_id FROM projects WHERE id = $1;

-- name: CountPolicyViolations30d :one
-- Warn-mode telemetry (G12): denials attributed to this policy key on the spine
-- in the trailing 30 days. The denial detail is "policy:<key> <reason>", so the
-- key is anchored with a trailing space — a prefix like 'ai' can't bleed into
-- 'ai-assistant'. The key charset is validated at authoring (no LIKE wildcards).
SELECT count(*) FROM events
WHERE org_id = $1
  AND action = 'authz.denied'
  AND at > now() - interval '30 days'
  AND detail->>'denied_by' LIKE 'policy:' || sqlc.arg('key') || ' %';

-- name: LinkPolicyEvent :exec
UPDATE policies SET last_change_event = $2 WHERE id = $1;

-- name: LinkPolicyVersionEvent :exec
UPDATE policy_versions SET change_event = $3 WHERE policy_id = $1 AND version = $2;
