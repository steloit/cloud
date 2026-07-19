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
UPDATE mfa_totp SET confirmed_at = now() WHERE user_id = $1;

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
