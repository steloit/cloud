-- T4.1: GitHub integration substrate (G3: ONE org-level app, a per-service
-- repo map). Links key on the REPO NAME + org — never the installation id —
-- so they survive app reinstalls (the AC); installations are a separate,
-- replaceable row.
CREATE TABLE github_installations (
    id              text PRIMARY KEY,          -- ghi_<hex>
    org_id          text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    installation_id bigint NOT NULL,
    account_login   text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,               -- uninstall keeps the row (reinstall = new row)
    UNIQUE (installation_id)
);

CREATE TABLE repo_links (
    id         text PRIMARY KEY,               -- rln_<hex>
    org_id     text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    service_id text NOT NULL REFERENCES services (id) ON DELETE CASCADE,
    repo       text NOT NULL,                  -- owner/name
    branch     text NOT NULL DEFAULT 'main',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (service_id)                        -- one repo per service; monorepos link many services to one repo
);

CREATE INDEX repo_links_repo_idx ON repo_links (org_id, repo);

-- Deliveries are stored idempotently by GitHub's delivery id — a redelivered
-- webhook never double-processes (the AC's "received and stored").
CREATE TABLE github_deliveries (
    id          text PRIMARY KEY,              -- ghd_<hex>
    delivery_id text NOT NULL UNIQUE,          -- X-GitHub-Delivery
    event       text NOT NULL,                 -- push | pull_request | installation | …
    action      text NOT NULL DEFAULT '',      -- opened | closed | created | deleted | …
    repo        text NOT NULL DEFAULT '',
    payload     jsonb NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX github_deliveries_repo_idx ON github_deliveries (repo, received_at DESC);
