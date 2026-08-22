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
    OR n.created_at < (SELECT c.created_at FROM notifications c WHERE c.id = sqlc.narg('cursor') AND c.user_id = $1)
    OR (n.created_at = (SELECT c.created_at FROM notifications c WHERE c.id = sqlc.narg('cursor') AND c.user_id = $1) AND n.id < sqlc.narg('cursor'))
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

-- name: ListPendingBellEvents :many
-- US-10.3: the bell projection scan. A pure anti-join against bell_scanned —
-- deliberately WITHOUT a time bound.
--
-- `e.at >= <watermark>` would be wrong here, not merely slower: events.at
-- defaults to now(), which is TRANSACTION START time, so a write tx that opens
-- before a tick and commits after it lands below any watermark and is skipped
-- permanently and silently. An unscanned event simply has no ledger row, so it
-- is picked up whenever it becomes visible, however late that is.
--
-- Ordered by (at, id) because ids.New is random hex and `at` collides freely
-- under batch insert; the pair is total. Served by events_at_id_idx.
--
-- COST — MEASURED, not reasoned. Two earlier descriptions of this plan (an
-- index-probe walk from the oldest row) were written from inspection and were
-- both WRONG. EXPLAIN (ANALYZE) at 200k spine rows in steady state:
--
--   Parallel Hash Anti Join
--     -> Parallel Seq Scan on events
--     -> Parallel Hash -> Parallel Seq Scan on bell_scanned
--
-- events_at_id_idx is NOT used. There is no early exit: LIMIT sits above a Sort
-- above a COMPLETED anti-join, so the whole ledger is hashed every tick and the
-- first scaling wall is work_mem (the hash spills to disk), not CPU. Measured
-- latency: 10k rows 3.8ms · 110k 31ms · 610k 514ms — roughly linear, reaching
-- the 10s tick period near 10M events.
--
-- Consequence to settle BEFORE this ships: the migration takes a SHARE lock on
-- `events` — blocking every mutating API call — to build an index this query
-- never uses, then pays that index's write amplification on the platform's
-- hottest append path forever. Either the query is reshaped so the index earns
-- its lock, or the index does not ship.
--
-- The floor-less shape is still correct and must stay: an `at` bound silently
-- drops transactions that open before a tick and commit after it. The durable
-- fix is a ledger-derived min-unscanned cursor plus pruning behind it, which
-- bounds the scan without reintroducing that bug.
SELECT e.*
FROM events e
LEFT JOIN bell_scanned bs ON bs.event_id = e.id
WHERE bs.event_id IS NULL
ORDER BY e.at ASC, e.id ASC
LIMIT 100;

-- name: ClaimBellEvent :one
-- Claim an event for projection. Returns no row when another replica already
-- claimed it, so exactly one fans out.
--
-- Unlike ClaimWebhookDelivery this claim is held INSIDE the fan-out transaction
-- — which is only safe because the bell does no network I/O. A crash rolls the
-- claim back with the partial fan-out, so a later run re-notifies EVERY member
-- rather than leaving members 4..10 of a 10-member org silently un-rung.
--
-- The cost, stated plainly: a second replica BLOCKS on the speculative index
-- entry until the winner's tx ends, so replicas serialize per batch.
INSERT INTO bell_scanned (event_id)
VALUES ($1)
ON CONFLICT (event_id) DO NOTHING
RETURNING event_id;

-- name: MarkBellEventUnrenderable :exec
-- Flag a worthy-but-unrenderable event so a later ruling on its copy can be
-- applied retroactively (a targeted DELETE re-queues exactly these rows).
UPDATE bell_scanned SET unrenderable = true WHERE event_id = $1;

-- name: GetDeployNotificationContext :one
-- US-10.3: the presentation data N1's deploy title needs, resolved during
-- projection (founder ruling 2026-07-20, Option A: render once, persist the
-- rendered title — a notification is a historical fact, and a later rename must
-- not rewrite it).
--
-- Local reads only, inside the fan-out tx. actor_name is COALESCEd to '' rather
-- than substituted: an absent name makes the frame UNRENDERABLE, and the
-- projection skips it instead of inventing copy.
-- Org-fenced: the deployment must belong to the event's org. The subject always
-- does today, but an unfenced lookup would render another tenant's environment
-- name into a title fanned to this org's members. Fencing is architectural, not
-- conditional on the current call path being safe.
SELECT d.number,
       env.name AS env_name,
       COALESCE((SELECT u.name FROM users u WHERE u.id = @actor_id), '')::text AS actor_name
FROM deployments d
JOIN environments env ON env.id = d.env_id
JOIN projects p ON p.id = env.project_id
WHERE d.id = @deployment_id AND p.org_id = @org_id;
