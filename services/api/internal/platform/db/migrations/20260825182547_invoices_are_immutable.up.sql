-- O39, my own adversarial pass: THE FREEZE HAD A BACK DOOR.
--
-- `quota_usage` is frozen for any period that has an invoice. But nothing
-- protected the `invoices` table, so DELETing an invoice un-freezes its period —
-- the rollup becomes writable again, and the carry-forward rows that reference it
-- as `origin_period` are orphaned against a figure that can now move. Deleting
-- the evidence of the boundary removed the boundary.
--
-- An invoice is also the customer-facing record of what they owe. `status`
-- legitimately changes (open -> paid/void), and only that.
CREATE FUNCTION invoices_are_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION
            'invoice %/% cannot be deleted: it is the accounting boundary that freezes its period (void it instead)',
            OLD.org_id, OLD.period
            USING ERRCODE = 'raise_exception';
    END IF;
    -- Everything that decides the money is immutable; the lifecycle status is not.
    IF NEW.org_id <> OLD.org_id OR NEW.period <> OLD.period
       OR NEW.total_cents <> OLD.total_cents OR NEW.lines::text <> OLD.lines::text
       OR NEW.closed_at IS DISTINCT FROM OLD.closed_at THEN
        RAISE EXCEPTION
            'invoice %/% is frozen: only status may change after close (corrections carry forward)',
            OLD.org_id, OLD.period
            USING ERRCODE = 'raise_exception';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER invoices_no_rewrite
    BEFORE UPDATE OR DELETE ON invoices
    FOR EACH ROW EXECUTE FUNCTION invoices_are_append_only();

CREATE FUNCTION invoices_no_truncate() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'invoices cannot be TRUNCATEd: every closed period would become mutable'
        USING ERRCODE = 'raise_exception';
END;
$$;

CREATE TRIGGER invoices_no_truncate_ever
    BEFORE TRUNCATE ON invoices
    FOR EACH STATEMENT EXECUTE FUNCTION invoices_no_truncate();
