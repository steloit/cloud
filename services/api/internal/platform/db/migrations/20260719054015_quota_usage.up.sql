-- T6.3: quota_usage — the rollup over the raw metering store (models.md).
-- Raw usage_events are RETAINED forever (append-only, T3.7); this table is a
-- derived, idempotently-recomputable view: (org, meter, period) → used.
CREATE TABLE quota_usage (
    org_id      text NOT NULL,
    meter       text NOT NULL,                 -- service_span_seconds | egress_bytes | build_minutes | events | ai_requests | …
    period      text NOT NULL,                 -- YYYY-MM (billing month)
    used        bigint NOT NULL DEFAULT 0,
    rate_cents  bigint NOT NULL DEFAULT 0,     -- weighted snapshot for span meters (Σ seconds × monthly rate)
    computed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, meter, period)
);

CREATE INDEX quota_usage_org_period_idx ON quota_usage (org_id, period);
