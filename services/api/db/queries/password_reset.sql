-- T7.2: password reset tokens.

-- name: InsertResetToken :exec
INSERT INTO password_reset_tokens (id, user_id, token_hash, ciphertext, nonce, wrapped_dek, dek_nonce, kek_id, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetResetTokenByHash :one
SELECT * FROM password_reset_tokens WHERE token_hash = $1;

-- name: GetResetToken :one
SELECT * FROM password_reset_tokens WHERE id = $1;

-- name: ConsumeResetToken :execrows
-- Single-use: marks used only if not already used and not expired. 0 rows = the
-- token was already spent or has expired.
UPDATE password_reset_tokens SET used_at = now()
WHERE id = $1 AND used_at IS NULL AND expires_at > now();

-- name: UpdatePasswordHash :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: ListPendingResetEmails :many
-- Account-level email outbox: reset tokens still valid that have no terminal
-- email delivery yet. The reset-token row is the durable trigger (account auth
-- is not an org-spine fact); email_deliveries (keyed by the token id) is the
-- idempotency ledger, shared with the org-event outbox.
SELECT t.id FROM password_reset_tokens t
LEFT JOIN email_deliveries d ON d.event_id = t.id
WHERE t.expires_at > now()
  AND (d.id IS NULL OR (d.status NOT IN ('sent', 'skipped') AND d.attempts < 5))
ORDER BY t.created_at ASC
LIMIT 100;
