-- T7.1: MFA.

-- name: UpsertTotpEnrollment :exec
INSERT INTO mfa_totp (user_id, ciphertext, nonce, wrapped_dek, dek_nonce, kek_id, confirmed_at)
VALUES ($1, $2, $3, $4, $5, $6, NULL)
ON CONFLICT (user_id) DO UPDATE SET
    ciphertext = EXCLUDED.ciphertext, nonce = EXCLUDED.nonce,
    wrapped_dek = EXCLUDED.wrapped_dek, dek_nonce = EXCLUDED.dek_nonce,
    kek_id = EXCLUDED.kek_id, confirmed_at = NULL, created_at = now();

-- name: GetTotp :one
SELECT * FROM mfa_totp WHERE user_id = $1;

-- name: ConfirmTotp :exec
-- Confirms enrollment and records the step used to confirm, so that same code
-- can't be replayed at the first login (single-use within the window).
UPDATE mfa_totp SET confirmed_at = now(), last_step = $2 WHERE user_id = $1;

-- name: AdvanceTotpStep :execrows
-- Consumes a TOTP step atomically: succeeds (1 row) only if this step is newer
-- than the last consumed one. 0 rows = the code was already used (replay).
UPDATE mfa_totp SET last_step = $2
WHERE user_id = $1 AND (last_step IS NULL OR last_step < $2);

-- name: SetMfaEnabled :exec
UPDATE users SET mfa_enabled = $2 WHERE id = $1;

-- name: DeleteTotp :exec
DELETE FROM mfa_totp WHERE user_id = $1;

-- name: DeleteRecoveryCodes :exec
DELETE FROM recovery_codes WHERE user_id = $1;

-- name: InsertRecoveryCode :exec
INSERT INTO recovery_codes (id, user_id, code_hash) VALUES ($1, $2, $3);

-- name: ConsumeRecoveryCode :execrows
UPDATE recovery_codes SET used_at = now()
WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL;
