DROP INDEX IF EXISTS environments_teardown_outstanding_idx;
ALTER TABLE environments DROP COLUMN IF EXISTS torn_down_at;
