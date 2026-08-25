-- O39: a CLOSED accounting period is immutable, enforced by the database.
--
-- THE DEFECT, measured before this migration: `invoice.Close` freezes an invoice
-- from `quota_usage`, and the invoice row itself is immutable (ON CONFLICT DO
-- NOTHING). But `quota_usage` is not. `Rollup` upserts with
-- `DO UPDATE SET used = EXCLUDED.used`, and `GET /orgs/{org}/usage?month=…` calls
-- Rollup on the read path with a CALLER-SUPPLIED month. So a customer viewing
-- last month's usage rewrites the rollup their invoice was computed from:
--
--   closed: invoice total=8640000, rollup used=3600s
--   after a late edge + one GET: rollup used=14400s, invoice total=8640000
--
-- 10800 seconds billed by nobody, and two customer-facing surfaces permanently
-- contradicting each other. Not a race — the normal read path.
--
-- WHY A TRIGGER AND NOT A CHECK IN THE QUERY. A WHERE clause is caller
-- discipline: the next query that touches this table, or a support script, or a
-- backfill, silently bypasses it. This table is the input to an invoice, so its
-- immutability must not depend on every future writer remembering. Same idiom as
-- `usage_events_append_only`, which is already the proven pattern here.
CREATE FUNCTION quota_usage_closed_period_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM invoices i
               WHERE i.org_id = OLD.org_id AND i.period = OLD.period) THEN
        RAISE EXCEPTION
            'quota_usage %/% is frozen: invoice for that period is closed (corrections carry forward, they never rewrite a closed period)',
            OLD.org_id, OLD.period
            USING ERRCODE = 'raise_exception';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER quota_usage_no_write_after_close
    BEFORE UPDATE OR DELETE ON quota_usage
    FOR EACH ROW EXECUTE FUNCTION quota_usage_closed_period_immutable();

-- Late usage is NEITHER dropped NOR allowed to restate a closed invoice: it is
-- carried forward, and this table is the record of that having happened.
--
-- Deciding between the market's three answers (Stripe drops past the grace
-- window — and documents that under a mid-cycle price change late usage appears
-- on NEITHER the current nor a later invoice; Metronome voids and regenerates;
-- Lago carries forward for recurring meters): carry-forward is the only one that
-- neither loses money silently nor changes a number a customer has already seen.
-- Dropping is invisible under-billing. Restating destroys the meaning of "closed".
CREATE TABLE usage_carry_forward (
    id             text PRIMARY KEY,            -- cf_<hex>
    org_id         text NOT NULL,
    meter          text NOT NULL,
    origin_period  text NOT NULL,               -- the CLOSED period the usage belongs to
    applied_period text,                        -- the open period it was billed in; NULL until then
    used           bigint NOT NULL,             -- meter units the closed rollup did not include
    rate_cents     bigint NOT NULL,
    detected_at    timestamptz NOT NULL DEFAULT now(),
    -- One carry-forward per (org, meter, origin period). A second detection of
    -- the same shortfall must not add a second charge — the amount is a DELTA
    -- against a frozen number, so recomputing it is idempotent by construction.
    UNIQUE (org_id, meter, origin_period)
);

CREATE INDEX usage_carry_forward_unapplied_idx
    ON usage_carry_forward (org_id) WHERE applied_period IS NULL;
