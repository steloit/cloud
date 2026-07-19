-- T10.3: notification routing matrix — bell store, per-user prefs, org webhooks,
-- and the webhook delivery outbox (same claim-before-send shape as email).

-- name: ListOrgMemberRecipients :many
-- The bell/email recipient set for an org event: every member, with the email
-- address the email route needs. Per-member prefs gate the actual routing.
SELECT m.user_id, u.email FROM members m JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1;

-- name: InsertNotification :one
-- The bell route: a per-user projection of a spine event. Idempotent per
-- (user, event) so a re-routed event never double-rings the bell.
INSERT INTO notifications (id, user_id, event_id, kind, title, body, link)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ListNotifications :many
-- Newest-first bell history for a user, keyset-paginated on (created_at, id) —
-- ids are random hex (not time-sortable), so the cursor resolves the last row's
-- created_at and pages strictly older, with id as the stable tiebreak.
SELECT n.* FROM notifications n
WHERE n.user_id = $1
  AND (sqlc.narg('unread')::bool IS NOT TRUE OR n.read = false)
  AND (
    sqlc.narg('cursor')::text IS NULL
    OR n.created_at < (SELECT c.created_at FROM notifications c WHERE c.id = sqlc.narg('cursor'))
    OR (n.created_at = (SELECT c.created_at FROM notifications c WHERE c.id = sqlc.narg('cursor')) AND n.id < sqlc.narg('cursor'))
  )
ORDER BY n.created_at DESC, n.id DESC
LIMIT $2;

-- name: MarkNotificationsRead :exec
-- Bulk mark-read, scoped to the owner: ids the caller doesn't own are untouched
-- (WHERE user_id) — never a 404 that leaks another user's id space.
UPDATE notifications SET read = true
WHERE user_id = $1 AND id = ANY(@ids::text[]);

-- name: GetNotificationPrefs :one
SELECT * FROM notification_prefs WHERE user_id = $1;

-- name: UpsertNotificationPrefs :one
INSERT INTO notification_prefs (user_id, channels, quiet_hours)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
    SET channels = EXCLUDED.channels, quiet_hours = EXCLUDED.quiet_hours, updated_at = now()
RETURNING *;

-- name: CreateWebhook :one
INSERT INTO webhooks (id, org_id, url, events, ciphertext, nonce, wrapped_dek, dek_nonce, kek_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListWebhooks :many
SELECT * FROM webhooks WHERE org_id = $1 ORDER BY created_at DESC;

-- name: GetWebhook :one
SELECT * FROM webhooks WHERE id = $1;

-- name: ListActiveWebhooksForOrg :many
-- The webhook route: active webhooks in the org whose filter matches the event
-- kind (an empty filter matches every kind).
SELECT * FROM webhooks
WHERE org_id = $1 AND status = 'active'
  AND (cardinality(events) = 0 OR @kind::text = ANY(events));

-- name: ClaimWebhookDelivery :one
-- Claim the (webhook, event) as `pending` BEFORE POSTing — written first so a
-- crash mid-send leaves an honest reclaimable pending, never a phantom sent.
-- A `sent`/`skipped` row is terminal (no id → skip); attempts capped so a
-- permanent failure dead-letters instead of hot-looping.
INSERT INTO webhook_deliveries (webhook_id, event_id, id, status, attempts)
VALUES ($1, $2, $3, 'pending', 1)
ON CONFLICT (webhook_id, event_id) DO UPDATE
    SET status = 'pending', attempts = webhook_deliveries.attempts + 1, updated_at = now()
    WHERE webhook_deliveries.status NOT IN ('sent', 'skipped')
      AND webhook_deliveries.attempts < 5
RETURNING id;

-- name: MarkWebhookDeliverySent :exec
UPDATE webhook_deliveries SET status = 'sent', status_code = $2, error = NULL, updated_at = now() WHERE id = $1;

-- name: MarkWebhookDeliveryFailed :exec
UPDATE webhook_deliveries SET status = 'failed', status_code = $2, error = $3, updated_at = now() WHERE id = $1;

-- name: ListPendingWebhookEvents :many
-- The durable outbox scan: spine events whose kind an active webhook subscribes
-- to and which have no TERMINAL delivery for that webhook (sent/skipped) and are
-- not dead-lettered. The spine + ledger ARE the outbox (lossless on restart).
SELECT e.*, w.id AS webhook_id
FROM events e
JOIN webhooks w
  ON w.org_id = e.org_id AND w.status = 'active'
 AND e.at >= w.created_at -- never backfill events that predate the webhook
 AND (cardinality(w.events) = 0 OR e.kind = ANY(w.events))
LEFT JOIN webhook_deliveries d ON d.webhook_id = w.id AND d.event_id = e.id
WHERE d.id IS NULL OR (d.status NOT IN ('sent', 'skipped') AND d.attempts < 5)
ORDER BY e.at ASC
LIMIT 100;
