-- T2.3: orgs + members — the containment tree's root and the RBAC subject.
CREATE TABLE orgs (
    id         text PRIMARY KEY,                -- org_<hex>
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE members (
    id         text PRIMARY KEY,                -- mbr_<hex>
    org_id     text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN ('owner', 'admin', 'developer', 'billing')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, user_id)
);

CREATE INDEX members_user_id_idx ON members (user_id);

-- Deferred FK from T2.2's forward reference: org API keys now point at orgs.
ALTER TABLE tokens
    ADD CONSTRAINT tokens_org_id_fkey FOREIGN KEY (org_id) REFERENCES orgs (id) ON DELETE CASCADE;
