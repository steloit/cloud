-- T7.6: account self-deletion grace window. deletion_scheduled_at marks
-- the account for purge after the grace window (a later job); until then the
-- user exists but every session is revoked at schedule time.
ALTER TABLE users ADD COLUMN deletion_scheduled_at timestamptz;
