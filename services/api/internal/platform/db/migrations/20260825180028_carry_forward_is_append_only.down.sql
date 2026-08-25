DROP TRIGGER IF EXISTS quota_usage_no_truncate_ever ON quota_usage;
DROP FUNCTION IF EXISTS quota_usage_no_truncate();
DROP TABLE IF EXISTS usage_carry_forward;
