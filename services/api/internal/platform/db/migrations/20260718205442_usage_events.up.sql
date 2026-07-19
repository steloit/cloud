-- T3.7: usage_events — the raw metering store (D10: every lifecycle edge
-- emits, from the FIRST resource; backfill is impossible, so this cannot
-- defer). Append-only like the spine: corrections are new events. T6.3's
-- rollup turns spans into quota_usage(org, meter, period).
CREATE TABLE usage_events (
    id         text PRIMARY KEY,               -- use_<hex>
    org_id     text NOT NULL,                  -- tags survive resource deletion:
    project_id text NOT NULL,                  -- no FKs by design (billing truth
    env_id     text NOT NULL,                  -- outlives the resource tree)
    service_id text NOT NULL,
    meter      text NOT NULL,                  -- service_span | compute_seconds | cu_hours | gb_months | egress_bytes | …
    edge       text NOT NULL CHECK (edge IN ('open', 'close')),
    product    text NOT NULL,
    rate_cents bigint NOT NULL,                -- monthly rate snapshot at the edge (integer cents, ADR-025)
    quantity   bigint NOT NULL DEFAULT 0,      -- meter-specific units (seconds, bytes, …); spans compute at rollup
    at         timestamptz NOT NULL DEFAULT now(),
    detail     jsonb NOT NULL DEFAULT '{}'
);

CREATE INDEX usage_events_org_at_idx ON usage_events (org_id, at DESC);
CREATE INDEX usage_events_service_idx ON usage_events (service_id, at);

CREATE FUNCTION usage_events_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'usage_events is append-only: % is forbidden (corrections are new events)', TG_OP
        USING ERRCODE = 'raise_exception';
END;
$$;

CREATE TRIGGER usage_events_no_update_delete
    BEFORE UPDATE OR DELETE ON usage_events
    FOR EACH ROW EXECUTE FUNCTION usage_events_append_only();
