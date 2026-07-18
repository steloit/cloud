-- T3.1: estimates — the estimate-before-provision law's substrate. An
-- estimate is priced desired state: createService (T3.3) accepts only a live,
-- env-matching estimate id; the line grammar here IS the invoice grammar.
CREATE TABLE estimates (
    id          text PRIMARY KEY,              -- est_<hex>
    org_id      text REFERENCES orgs (id) ON DELETE CASCADE,
    env_id      text REFERENCES environments (id) ON DELETE CASCADE,
    services    jsonb NOT NULL,                -- the priced ServiceShapeInput[] (acceptance compares against this)
    lines       jsonb NOT NULL,                -- Estimate.lines verbatim
    total_cents bigint NOT NULL,
    expires_at  timestamptz NOT NULL,
    accepted_at timestamptz,                   -- set by createService (T3.3)
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX estimates_env_idx ON estimates (env_id);
