-- T7.2: single-use password reset tokens. The token is sha256-hashed for O(1)
-- constant-time lookup+verify AND envelope-sealed (secrets.Seal, AAD-bound) so
-- the async email dispatcher can recover the plaintext to build the reset link
-- without the secret ever sitting in plaintext at rest (the TOTP-secret pattern,
-- T7.1). Short-lived + single-use bound the exposure.
CREATE TABLE password_reset_tokens (
    id          text PRIMARY KEY,           -- prt_<hex>; the reset event's subject
    user_id     text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  bytea NOT NULL UNIQUE,       -- sha256(token) — lookup + verify
    ciphertext  bytea NOT NULL,              -- sealed plaintext token (for the email link)
    nonce       bytea NOT NULL,
    wrapped_dek bytea NOT NULL,
    dek_nonce   bytea NOT NULL,
    kek_id      text NOT NULL,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX password_reset_user_idx ON password_reset_tokens (user_id);
