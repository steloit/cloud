-- T3.3: services — desired state rows (D9: the reconciler converges them;
-- nothing here provisions). Status vocabulary is ADR-024: ready, never
-- running; metering starts at ready. cell_id per invariant 1.
CREATE TABLE services (
    id                     text PRIMARY KEY,   -- svc_<hex>
    env_id                 text NOT NULL REFERENCES environments (id) ON DELETE CASCADE,
    name                   text NOT NULL,
    product                text NOT NULL CHECK (product IN ('postgres', 'valkey', 'web', 'worker')),
    intent                 text CHECK (intent IN ('app', 'database', 'jobs', 'search', 'vector', 'cache', 'storage', 'ai')),
    status                 text NOT NULL DEFAULT 'provisioning'
        CHECK (status IN ('provisioning', 'ready', 'degraded', 'failed', 'suspended', 'deleting')),
    shape                  jsonb NOT NULL DEFAULT '{}',
    scaling                jsonb,              -- D22: mode + range; separate from shape (range changes never restart)
    override               jsonb,              -- manual pin {instances, reason, expires_at}; 24h auto-expiry
    provisioning_steps     jsonb NOT NULL DEFAULT '[]',  -- C4 timeline
    monthly_estimate_cents bigint NOT NULL DEFAULT 0,
    estimate_id            text REFERENCES estimates (id),
    cell_id                text NOT NULL DEFAULT 'cell-0',
    created_at             timestamptz NOT NULL DEFAULT now(),
    UNIQUE (env_id, name)
);

CREATE INDEX services_env_status_idx ON services (env_id, status);
