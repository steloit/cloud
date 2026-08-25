DROP TRIGGER IF EXISTS invoices_no_truncate_ever ON invoices;
DROP FUNCTION IF EXISTS invoices_no_truncate();
DROP TRIGGER IF EXISTS invoices_no_rewrite ON invoices;
DROP FUNCTION IF EXISTS invoices_are_append_only();
