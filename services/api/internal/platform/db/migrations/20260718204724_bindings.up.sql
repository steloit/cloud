-- T3.6: bindings — wiring, $0 (models.md row). Internal target (target_id)
-- OR external provider (provider + secret_ref) per A5.5; credentials live in
-- Secrets (secret_ref), NEVER here. FK ON DELETE RESTRICT: deleting a bound
-- service goes through the 409-dependents path, never a silent cascade.
CREATE TABLE bindings (
    id              text PRIMARY KEY,          -- bnd_<hex>
    source_id       text NOT NULL REFERENCES services (id) ON DELETE RESTRICT,
    target_type     text NOT NULL DEFAULT 'service'
        CHECK (target_type IN ('service', 'storage', 'ai')),
    target_id       text REFERENCES services (id) ON DELETE RESTRICT,
    provider        text,
    provider_config jsonb,
    secret_ref      text,
    intent          text CHECK (intent IN ('app', 'database', 'jobs', 'search', 'vector', 'cache', 'storage', 'ai')),
    scope           text NOT NULL DEFAULT 'read_only'
        CHECK (scope IN ('read_only', 'read_write')),
    status          text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'active', 'revoked')),
    rotated_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (target_type = 'service' AND target_id IS NOT NULL AND provider IS NULL)
        OR (target_type IN ('storage', 'ai') AND target_id IS NULL AND provider IS NOT NULL)
    )
);

CREATE UNIQUE INDEX bindings_source_target_uniq
    ON bindings (source_id, target_id) WHERE target_type = 'service' AND status <> 'revoked';
CREATE INDEX bindings_source_idx ON bindings (source_id);
CREATE INDEX bindings_target_idx ON bindings (target_id);
