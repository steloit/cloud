-- T7.1: MFA (TOTP + recovery codes). The TOTP secret is envelope-encrypted
-- (secrets.Seal, AAD-bound to the user) — never plaintext at rest. Recovery
-- codes are stored as sha256 hashes, single-use.
ALTER TABLE users ADD COLUMN mfa_enabled boolean NOT NULL DEFAULT false;

CREATE TABLE mfa_totp (
    user_id      text PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    ciphertext   bytea NOT NULL,
    nonce        bytea NOT NULL,
    wrapped_dek  bytea NOT NULL,
    dek_nonce    bytea NOT NULL,
    kek_id       text NOT NULL,
    confirmed_at timestamptz,          -- NULL = enrolled but not yet verified
    last_step    bigint,               -- last consumed RFC 6238 step (replay guard)
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE recovery_codes (
    id         text PRIMARY KEY,       -- rec_<hex>
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash  bytea NOT NULL,         -- sha256(code); the plaintext is shown once
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX recovery_codes_user_idx ON recovery_codes (user_id) WHERE used_at IS NULL;
