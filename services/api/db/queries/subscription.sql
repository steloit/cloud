-- T11.2: subscription lifecycle state machine.

-- name: GetSubscription :one
SELECT * FROM subscriptions WHERE org_id = $1;

-- name: SetSubscriptionState :one
-- The single write for every transition: status + the derived timestamps, set
-- atomically. The state machine computes these; the DB just persists them.
UPDATE subscriptions SET
    status = $2,
    plan = $3,
    dunning_started_at = $4,
    next_retry_at = $5,
    plan_ends_at = $6,
    trial_ends_at = $7,
    updated_at = now()
WHERE org_id = $1
RETURNING *;

-- name: ListSubscriptionsToAdvance :many
-- The daily lifecycle sweep: subs in a dunning track (grace/provisioning_paused)
-- or with a due cancel-at-anchor. AdvanceLifecycle is idempotent, so re-scanning
-- is safe.
SELECT org_id FROM subscriptions
WHERE status IN ('grace', 'provisioning_paused')
   OR (status = 'cancelled_at_anchor' AND plan_ends_at IS NOT NULL AND plan_ends_at <= now())
ORDER BY updated_at ASC
LIMIT 500;
