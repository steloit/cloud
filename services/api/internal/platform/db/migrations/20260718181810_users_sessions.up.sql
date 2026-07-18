-- T2.1: identity foundation (models.md: users, sessions).
CREATE TABLE users (
    id            text PRIMARY KEY,            -- usr_<hex> (x-conventions prefixed ids)
    email         text NOT NULL,
    password_hash text NOT NULL,               -- argon2id PHC string, never plaintext
    name          text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));

CREATE TABLE sessions (
    id           text PRIMARY KEY,             -- ses_<hex>
    user_id      text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   bytea NOT NULL UNIQUE,        -- sha256(cookie token); raw never stored
    device       text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);
