-- T2.2: shared tokens table — personal tokens (tok_) + org API keys (key_).
-- org_id has NO FK yet: the orgs table lands with T3.2/T2.7; the FK is added
-- in that migration (forward reference documented, not enforced early).
CREATE TABLE tokens (
    id           text PRIMARY KEY,              -- tok_<hex> | key_<hex>
    kind         text NOT NULL CHECK (kind IN ('personal', 'org')),
    user_id      text REFERENCES users (id) ON DELETE CASCADE,
    org_id       text,
    name         text NOT NULL,
    scope        text NOT NULL CHECK (scope IN ('full', 'read_only')),
    prefix       text NOT NULL,                 -- shown in lists; never the secret
    token_hash   bytea NOT NULL UNIQUE,         -- sha256(secret); plaintext never stored
    expires_at   timestamptz,
    last_used_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    revoked_at   timestamptz,
    CHECK ((kind = 'personal' AND user_id IS NOT NULL) OR (kind = 'org' AND org_id IS NOT NULL))
);

CREATE INDEX tokens_user_id_idx ON tokens (user_id);
CREATE INDEX tokens_org_id_idx ON tokens (org_id);
