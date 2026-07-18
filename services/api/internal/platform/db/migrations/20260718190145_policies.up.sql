-- T2.4: policies — evaluation substrate (models.md; authoring API/UI = E12).
-- Attachment: org-level (project_id NULL) or project-level; env-sensitivity
-- lives in config (closest-wins resolution is code, not schema).
CREATE TABLE policies (
    id          text PRIMARY KEY,               -- pol_<hex>
    org_id      text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    project_id  text,                           -- NULL = org-wide; FK added with the projects migration (T3.2)
    key         text NOT NULL,                  -- typed kind: ai_assistant | region_allowlist | …
    enforcement text NOT NULL CHECK (enforcement IN ('enabled', 'opt_in', 'disabled')),
    config      jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    -- NULLS NOT DISTINCT (PG15+): without it, NULL project_ids are distinct
    -- and duplicate ORG-WIDE rows of the same key would slip through.
    UNIQUE NULLS NOT DISTINCT (org_id, project_id, key)
);

CREATE INDEX policies_org_idx ON policies (org_id);
