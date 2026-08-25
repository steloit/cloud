-- O39 review round 1: four money defects in the first carry-forward design.
--
-- (1) A SECOND late shortfall was silently discarded. The unique key was
--     (org, meter, origin_period) with `ON CONFLICT … DO UPDATE … WHERE
--     applied_period IS NULL`; once the first carry was billed, the WHERE failed,
--     the :exec discarded the row count, and nothing errored. Measured: 14,400 s
--     / 34,560,000 c on no invoice, ever — precisely the Stripe failure mode the
--     design claimed to avoid.
-- (2) The delta was computed against the FROZEN row only, never against
--     `frozen + already-carried`, so even a working upsert would have double-billed
--     the first shortfall or overwritten it with a cumulative figure.
--
-- The fix is to stop upserting a running total and make the ledger APPEND-ONLY,
-- one row per detection, like `usage_events` itself:
--
--     remainder = recomputed − frozen − Σ(already carried for that origin)
--
-- A detection with remainder 0 appends nothing, so repeated recomputes are
-- idempotent BY CONSTRUCTION rather than by a conflict clause — and a second, a
-- third, an nth late arrival each append their own remainder. No unique key is
-- needed because no row is ever revised.
DROP TABLE IF EXISTS usage_carry_forward;

CREATE TABLE usage_carry_forward (
    id             text PRIMARY KEY,            -- cf_<hex>
    org_id         text NOT NULL,
    meter          text NOT NULL,
    origin_period  text NOT NULL,               -- the CLOSED period the usage belongs to
    applied_period text,                        -- the invoice period that billed it; NULL until then
    -- SIGNED. A negative remainder means we over-billed the closed period, and
    -- (3) the first design discarded that silently: `if deltaSeconds <= 0 && deltaCents <= 0`
    -- returned nil with no row and no log, so under-billing was carried forward
    -- with a WARN while over-billing — the customer's money — vanished. The
    -- asymmetry was the defect.
    used           bigint NOT NULL,
    rate_cents     bigint NOT NULL,
    -- `charge` is billed on the next close. `credit` is recorded and surfaced but
    -- NOT auto-applied: issuing a refund is a commercial decision, and the
    -- engineering obligation is that it cannot be invisible.
    kind           text NOT NULL CHECK (kind IN ('charge', 'credit')),
    -- The unit the amount is in. ADR-0018 moves rate_cents from a monthly-rate
    -- snapshot to a per-unit-hour price; a row written before that migration must
    -- not be re-read under the new arithmetic without anyone noticing.
    rate_unit      text NOT NULL DEFAULT 'monthly_rate_cent_seconds',
    detected_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX usage_carry_forward_unapplied_idx
    ON usage_carry_forward (org_id, origin_period) WHERE applied_period IS NULL;

-- (4) THE TRIGGER WAS BEFORE UPDATE OR DELETE, so INSERT into a closed period
--     SUCCEEDED — measured: a fresh `egress_bytes` row landed behind an already
--     frozen invoice with rate_cents=500000. TRUNCATE also succeeded, because a
--     FOR EACH ROW trigger never fires for it.
--
-- The `usage_events_append_only` idiom this copied is for a table where INSERT is
-- LEGITIMATE. That reasoning does not transfer to a freeze: a period with no row
-- for a meter at close time could still gain one afterwards, which is a rollup the
-- invoice never saw. Reachable today — `PeriodIsClosed` and `UpsertQuotaUsage` are
-- separate statements with no transaction between them.
DROP TRIGGER IF EXISTS quota_usage_no_write_after_close ON quota_usage;
DROP FUNCTION IF EXISTS quota_usage_closed_period_immutable();

CREATE FUNCTION quota_usage_closed_period_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    o text := COALESCE(NEW.org_id, OLD.org_id);
    p text := COALESCE(NEW.period, OLD.period);
BEGIN
    IF EXISTS (SELECT 1 FROM invoices i WHERE i.org_id = o AND i.period = p) THEN
        RAISE EXCEPTION
            'quota_usage %/% is frozen: invoice for that period is closed (corrections carry forward, they never rewrite a closed period)',
            o, p
            USING ERRCODE = 'raise_exception';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER quota_usage_no_write_after_close
    BEFORE INSERT OR UPDATE OR DELETE ON quota_usage
    FOR EACH ROW EXECUTE FUNCTION quota_usage_closed_period_immutable();

-- TRUNCATE needs its own statement-level arm: a row trigger never sees it, so the
-- whole table could be emptied out from under every closed invoice.
CREATE FUNCTION quota_usage_no_truncate() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'quota_usage cannot be TRUNCATEd: it is the input to closed invoices'
        USING ERRCODE = 'raise_exception';
END;
$$;

CREATE TRIGGER quota_usage_no_truncate_ever
    BEFORE TRUNCATE ON quota_usage
    FOR EACH STATEMENT EXECUTE FUNCTION quota_usage_no_truncate();
