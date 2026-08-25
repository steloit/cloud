DROP TABLE IF EXISTS usage_carry_forward;
DROP TRIGGER IF EXISTS quota_usage_no_write_after_close ON quota_usage;
DROP FUNCTION IF EXISTS quota_usage_closed_period_immutable();
