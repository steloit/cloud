-- T10.4: the email delivery ledger + durable outbox (idempotent, bounded retry).

-- name: ClaimEmailDelivery :one
-- Claim the (event, recipient) as `pending` BEFORE sending — the row is written
-- first so a crash mid-send leaves an honest reclaimable pending, never a
-- phantom sent. Reclaims a prior pending/failed (crash recovery / transient
-- retry), bumping attempts; a `sent`/`skipped` row is terminal (no id → skip),
-- and attempts are capped so a permanent failure dead-letters instead of
-- hot-looping. Single-writer per (event,recipient) via the UNIQUE constraint.
INSERT INTO email_deliveries (id, event_id, org_id, recipient, template, template_version, provider, status, attempts)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', 1)
ON CONFLICT (event_id, recipient) DO UPDATE
    SET status = 'pending', attempts = email_deliveries.attempts + 1, updated_at = now()
    WHERE email_deliveries.status NOT IN ('sent', 'skipped')
      AND email_deliveries.attempts < 5
RETURNING id;

-- name: MarkEmailDeliverySent :exec
UPDATE email_deliveries SET status = 'sent', provider_id = $2, error = NULL, updated_at = now() WHERE id = $1;

-- name: MarkEmailDeliveryFailed :exec
UPDATE email_deliveries SET status = 'failed', error = $2, updated_at = now() WHERE id = $1;

-- name: RecordSkippedDelivery :exec
-- Terminal "nothing to send" (vanished invite / missing org) so the event drops
-- out of the scan instead of being re-resolved every poll (poison-event guard).
INSERT INTO email_deliveries (id, event_id, org_id, recipient, template, template_version, provider, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'skipped')
ON CONFLICT (event_id, recipient) DO NOTHING;

-- name: GetEmailDelivery :one
SELECT * FROM email_deliveries WHERE event_id = $1 AND recipient = $2;

-- name: ListPendingMailEvents :many
-- The durable outbox scan: spine events whose action triggers mail and which
-- have no TERMINAL delivery (sent/skipped) and are not a dead-lettered failure
-- (attempts exhausted). Pending (crashed mid-send) and transient-failed rows are
-- reclaimed; the spine + ledger ARE the outbox, so nothing is lost on restart.
SELECT e.* FROM events e
LEFT JOIN email_deliveries d ON d.event_id = e.id
WHERE e.action = ANY(@actions::text[])
  AND (d.id IS NULL OR (d.status NOT IN ('sent', 'skipped') AND d.attempts < 5))
ORDER BY e.at ASC
LIMIT 100;
