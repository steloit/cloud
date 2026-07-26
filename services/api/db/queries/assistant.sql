-- name: CreateThread :one
INSERT INTO assistant_threads (id, org_id, user_id, title, context, attached_insight)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListThreadsForUserInOrg :many
-- Threads the user owns in ONE org, newest first. The disable gate is applied
-- at the org level BEFORE this runs (a disabled org's threads are hidden, not
-- filtered here — they simply are not queried).
SELECT * FROM assistant_threads
WHERE user_id = $1 AND org_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: GetThreadForUser :one
-- Ownership-fenced fetch (the viewer must own the thread).
SELECT * FROM assistant_threads WHERE id = $1 AND user_id = $2;

-- name: CreateMessage :one
INSERT INTO assistant_messages (id, thread_id, role, content, evidence, proposal_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListMessagesForThread :many
SELECT * FROM assistant_messages WHERE thread_id = $1 ORDER BY created_at;
