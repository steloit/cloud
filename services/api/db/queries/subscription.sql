-- T11.2: subscription lifecycle state machine.

-- name: GetSubscription :one
SELECT * FROM subscriptions WHERE org_id = $1;

-- name: SetSubscriptionState :one
-- The single write for every transition, with COMPARE-AND-SWAP on status: the
-- update lands only if the row is still in the status the caller read
-- (@expected). A concurrent transition (a payment webhook racing the dunning
-- worker, or worker-vs-worker across replicas) changes status first, so this
-- returns 0 rows → the caller's decision is stale → it re-reads, never clobbers.
-- Prevents the lost-update that would silently re-suspend a customer who paid.
UPDATE subscriptions SET
    status = $2,
    plan = $3,
    dunning_started_at = $4,
    next_retry_at = $5,
    plan_ends_at = $6,
    trial_ends_at = $7,
    updated_at = now()
WHERE org_id = $1 AND status = sqlc.arg('expected')
RETURNING *;

-- name: ListSubscriptionsToAdvance :many
-- The daily lifecycle sweep: subs whose time-driven state may have advanced — an
-- expired trial, a dunning track, or a due cancel-at-anchor. AdvanceLifecycle is
-- idempotent (CAS-guarded), so re-scanning across replicas is safe.
SELECT org_id FROM subscriptions
WHERE (status = 'trial' AND trial_ends_at IS NOT NULL AND trial_ends_at <= now())
   OR (pending_plan IS NOT NULL AND pending_plan_at IS NOT NULL AND pending_plan_at <= now())
   OR status IN ('grace', 'provisioning_paused')
   OR (status = 'cancelled_at_anchor' AND plan_ends_at IS NOT NULL AND plan_ends_at <= now())
ORDER BY updated_at ASC
LIMIT 500;

-- name: SetPendingPlan :one
-- A clean downgrade schedules at the anchor (B4) — never immediate. Guarded:
-- never onto a cancelled row (the wind-down owns the anchor — review H1/M1);
-- no row back = the row left the schedulable states mid-flight.
UPDATE subscriptions SET pending_plan = $2, pending_plan_at = $3, updated_at = now()
WHERE org_id = $1 AND status <> 'cancelled_at_anchor'
RETURNING *;

-- name: ClearPendingPlan :exec
UPDATE subscriptions SET pending_plan = NULL, pending_plan_at = NULL, updated_at = now()
WHERE org_id = $1;

-- name: SyncOrgPlan :exec
-- subscriptions.plan is the billing master; orgs.plan is what every plan gate
-- reads — they must NEVER diverge (Q9 finding: cancel-at-anchor previously
-- dropped subscriptions.plan to free while orgs.plan stayed paid).
UPDATE orgs SET plan = $2 WHERE id = $1;

-- name: ApplyPendingPlan :one
-- ATOMIC anchor apply (review: a status-only CAS let a stale sweep read revert
-- a just-committed upgrade): plan moves to pending_plan and pending clears in
-- ONE statement, conditional on the pending value still being the one that is
-- due — a concurrent upgrade's ClearPendingPlan makes this a no-row no-op.
UPDATE subscriptions
SET plan = pending_plan, pending_plan = NULL, pending_plan_at = NULL, updated_at = now()
WHERE org_id = $1 AND pending_plan IS NOT NULL AND pending_plan_at IS NOT NULL AND pending_plan_at <= $2
RETURNING *;
