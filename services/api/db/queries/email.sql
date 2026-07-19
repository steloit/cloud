-- T10.4: the email delivery ledger (idempotent by event_id + recipient).

-- name: ClaimEmailDelivery :one
-- Claim the (event, recipient) pair BEFORE sending: if a row already exists
-- (the event was re-processed), no id is returned and the caller skips the send
-- — the claim is the idempotency gate, the UNIQUE constraint the backstop.
INSERT INTO email_deliveries (id, event_id, org_id, recipient, template, template_version, provider, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'sent')
ON CONFLICT (event_id, recipient) DO NOTHING
RETURNING id;

-- name: MarkEmailDeliverySent :exec
UPDATE email_deliveries SET status = 'sent', provider_id = $2, error = NULL WHERE id = $1;

-- name: MarkEmailDeliveryFailed :exec
UPDATE email_deliveries SET status = 'failed', error = $2 WHERE id = $1;

-- name: GetEmailDelivery :one
SELECT * FROM email_deliveries WHERE event_id = $1 AND recipient = $2;

-- name: ListPendingMailEvents :many
-- The durable outbox scan: spine events whose action triggers mail but which
-- have no delivery yet. The spine + delivery ledger ARE the outbox — no row is
-- lost on restart, and Dispatch is idempotent, so re-scanning is safe.
SELECT e.* FROM events e
LEFT JOIN email_deliveries d ON d.event_id = e.id
WHERE e.action = ANY(@actions::text[]) AND d.id IS NULL
ORDER BY e.at ASC
LIMIT 100;
