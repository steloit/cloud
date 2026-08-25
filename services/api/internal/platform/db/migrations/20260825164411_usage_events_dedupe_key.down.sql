DROP INDEX IF EXISTS usage_events_dedupe_key_idx;
ALTER TABLE usage_events DROP COLUMN IF EXISTS dedupe_key;
ALTER TABLE services DROP COLUMN IF EXISTS status_changed_at;
