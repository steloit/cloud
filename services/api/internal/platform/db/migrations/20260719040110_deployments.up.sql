-- T4.3: deployments — IMMUTABLE HISTORY (DP1). Rows are never deleted and
-- their identity never rewrites; only the lifecycle columns (state, gates,
-- canary_percent, annotation) may advance. number is the #142 in every
-- marker: per-env, monotonic, assigned in the insert tx.
CREATE TABLE deployments (
    id             text PRIMARY KEY,           -- dep_<hex>
    number         int NOT NULL,
    env_id         text NOT NULL REFERENCES environments (id) ON DELETE RESTRICT,
    service_id     text NOT NULL REFERENCES services (id) ON DELETE RESTRICT,
    git_sha        text NOT NULL DEFAULT '',
    state          text NOT NULL DEFAULT 'queued'
        CHECK (state IN ('queued', 'building', 'migrating', 'canary', 'verifying', 'live', 'failed', 'rolled_back', 'aborted')),
    actor          text NOT NULL,              -- human, token, or `user via assistant`
    canary_percent int,
    gates          jsonb NOT NULL DEFAULT '[]',
    migrations     jsonb NOT NULL DEFAULT '[]',
    annotation     text,
    promoted_from  text REFERENCES deployments (id),
    rollback_of    text REFERENCES deployments (id),
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (env_id, number)
);

CREATE INDEX deployments_env_idx ON deployments (env_id, number DESC);
CREATE INDEX deployments_service_idx ON deployments (service_id, number DESC);

-- Immutability: DELETE never; UPDATE may touch lifecycle columns only.
CREATE FUNCTION deployments_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'deployments are immutable history: DELETE is forbidden (DP1)'
            USING ERRCODE = 'raise_exception';
    END IF;
    IF NEW.id <> OLD.id OR NEW.number <> OLD.number OR NEW.env_id <> OLD.env_id
        OR NEW.service_id <> OLD.service_id OR NEW.git_sha <> OLD.git_sha
        OR NEW.actor <> OLD.actor OR NEW.created_at <> OLD.created_at
        OR NEW.promoted_from IS DISTINCT FROM OLD.promoted_from
        OR NEW.rollback_of IS DISTINCT FROM OLD.rollback_of THEN
        RAISE EXCEPTION 'deployments are immutable history: identity columns never rewrite (DP1)'
            USING ERRCODE = 'raise_exception';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER deployments_immutable_guard
    BEFORE UPDATE OR DELETE ON deployments
    FOR EACH ROW EXECUTE FUNCTION deployments_immutable();
