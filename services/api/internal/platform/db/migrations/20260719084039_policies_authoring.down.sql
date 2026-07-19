-- T12.1 rollback.
DROP TABLE IF EXISTS policy_versions;

ALTER TABLE policies DROP CONSTRAINT policies_enforcement_check;
ALTER TABLE policies ADD CONSTRAINT policies_enforcement_check
    CHECK (enforcement IN ('enabled', 'opt_in', 'disabled'));

ALTER TABLE policies DROP COLUMN IF EXISTS updated_at;
ALTER TABLE policies DROP COLUMN IF EXISTS last_change_event;
ALTER TABLE policies DROP COLUMN IF EXISTS enforce_from;
ALTER TABLE policies DROP COLUMN IF EXISTS version;
ALTER TABLE policies DROP COLUMN IF EXISTS description;
ALTER TABLE policies DROP COLUMN IF EXISTS rule;
