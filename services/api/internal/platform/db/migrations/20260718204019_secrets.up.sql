-- T3.5: secrets — versioned, scoped, envelope-encrypted (D5: KMS envelope,
-- never invent crypto). Plaintext NEVER at rest: each row stores AES-256-GCM
-- ciphertext under a per-secret DEK, itself wrapped by the KEK (env-provided
-- until P1 lands GCP KMS behind the same seam). Internal-only in v1 (no CRUD
-- API in the contract — recorded finding).
CREATE TABLE secrets (
    id          text PRIMARY KEY,              -- sec_<hex>
    org_id      text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    project_id  text REFERENCES projects (id) ON DELETE CASCADE,
    env_id      text REFERENCES environments (id) ON DELETE CASCADE,
    name        text NOT NULL,
    version     int NOT NULL,
    ciphertext  bytea NOT NULL,
    nonce       bytea NOT NULL,
    wrapped_dek bytea NOT NULL,
    dek_nonce   bytea NOT NULL,
    kek_id      text NOT NULL,                 -- which KEK wrapped the DEK (rotation)
    created_by  text NOT NULL,                 -- usr_/tok_ or 'system'
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (org_id, project_id, env_id, name, version)
);

CREATE INDEX secrets_scope_idx ON secrets (org_id, name);
