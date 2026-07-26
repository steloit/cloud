-- T12.6: dashboards (DB8) — SCOPE (org | project:prj_…) is orthogonal to
-- VISIBILITY (personal | org | restricted). Scope decides WHAT data a dashboard
-- reads; visibility decides WHO sees the dashboard object. Dashboards READ,
-- never provision. Project-scoped dashboards are "born filtered": creating or
-- reading one requires access to that project.
CREATE TABLE dashboards (
    id         text PRIMARY KEY,               -- dsh_<hex>
    org_id     text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    name       text NOT NULL,
    -- 'org' or 'project:prj_…'. Kept as the contract's opaque string; the
    -- project id is parsed at the boundary and access-checked.
    scope      text NOT NULL,
    visibility text NOT NULL CHECK (visibility IN ('personal', 'org', 'restricted')),
    owner_id   text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    prebuilt   boolean NOT NULL DEFAULT false,
    layout     jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX dashboards_org ON dashboards (org_id, created_at DESC);

CREATE TABLE dashboard_widgets (
    id           text PRIMARY KEY,             -- wdg_<hex>
    dashboard_id text NOT NULL REFERENCES dashboards (id) ON DELETE CASCADE,
    source       text NOT NULL CHECK (source IN ('metrics', 'logs', 'cost', 'deploys', 'ai', 'alerts')),
    query        text NOT NULL,
    viz          text NOT NULL CHECK (viz IN ('line', 'area', 'stat', 'table', 'list')),
    pos          jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX dashboard_widgets_dash ON dashboard_widgets (dashboard_id, created_at);
