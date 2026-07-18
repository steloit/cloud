-- T3.2: projects + environments — the containment tree below orgs.
-- Every resource row carries cell_id (INF-001 invariant 1) even while it is
-- always cell-0; project → exactly one cell (D7).
CREATE TABLE projects (
    id                    text PRIMARY KEY,    -- prj_<hex>
    org_id                text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    name                  text NOT NULL,
    cell_id               text NOT NULL DEFAULT 'cell-0',
    deletion_scheduled_at timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

-- ADR-037: production is born with the project (implicit-by-default; a
-- stable identity, not a deployment state). region_override NULL = inherit
-- the org home_region (models.md).
CREATE TABLE environments (
    id              text PRIMARY KEY,          -- env_<hex>
    project_id      text NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name            text NOT NULL,
    region_override text,
    kind            text NOT NULL DEFAULT 'standard' CHECK (kind IN ('standard', 'preview')),
    policy_flags    jsonb NOT NULL DEFAULT '[]',
    expires_at      timestamptz,               -- previews: expired by policy
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

CREATE INDEX environments_project_idx ON environments (project_id);

-- The policies.project_id FK deferred from T2.4 ("FK added with T3.2").
ALTER TABLE policies
    ADD CONSTRAINT policies_project_id_fkey
    FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;
