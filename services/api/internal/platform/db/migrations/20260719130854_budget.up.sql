-- T11.6: the hard spend cap (B1 budget, F9 flagship). One bound per org; the
-- platform ENFORCES it at the estimate-accept gate — crossing it is impossible
-- by construction, never alerts-only. A NULL limit_cents (or no row) is
-- uncapped. alert_thresholds are the % points that warn (80 default); storing
-- them here, the warn DELIVERY rides T10.3 routing (follow-up).
CREATE TABLE budgets (
    org_id           text PRIMARY KEY REFERENCES orgs (id) ON DELETE CASCADE,
    limit_cents      bigint,                       -- NULL = no cap
    alert_thresholds integer[] NOT NULL DEFAULT '{80}',
    updated_at       timestamptz NOT NULL DEFAULT now()
);
