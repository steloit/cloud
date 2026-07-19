-- T12.4: templates — FROZEN COPIES (ADR-021): capture strips all secret
-- material (the CHECK below is the DB-level guard QA #5 demands, defense in
-- depth behind the structural whitelist in capture); editing/deleting a
-- template never touches instantiations (copies never link — used_by_count is
-- informational, no FK from instantiations back to templates exists ANYWHERE).
-- source_* are provenance STRINGS (never FKs) so a template survives its
-- source's deletion.
CREATE TABLE templates (
    id                     text PRIMARY KEY,             -- tpl_<hex>
    org_id                 text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    name                   text NOT NULL,
    visibility             text NOT NULL DEFAULT 'org'
        CHECK (visibility IN ('org', 'restricted')),
    version                int NOT NULL DEFAULT 1,
    source_project         text NOT NULL,                -- provenance: "ecommerce"
    source_env             text NOT NULL,                -- provenance: "production"
    contents               jsonb NOT NULL,
    required_inputs        jsonb NOT NULL DEFAULT '[]',  -- excluded-service bindings, surfaced
    monthly_estimate_cents bigint NOT NULL DEFAULT 0,    -- instantiation price from birth (integer cents, ADR-025)
    used_by_count          int NOT NULL DEFAULT 0,
    updated_by             text NOT NULL,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name),
    -- QA #5's DB-level guard: contents may NEVER carry secret ids, secret refs,
    -- or ciphertext fields. Conservative by design — a capture tripping this is
    -- rejected loudly (problem+json), never silently stripped-after-the-fact.
    CONSTRAINT templates_no_secret_material CHECK (
        position('sec_' in contents::text) = 0
        AND position('secret_ref' in contents::text) = 0
        AND position('ciphertext' in contents::text) = 0
    )
);
CREATE INDEX templates_org_idx ON templates (org_id, updated_at DESC);
