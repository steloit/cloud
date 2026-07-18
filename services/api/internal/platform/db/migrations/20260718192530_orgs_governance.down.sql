DROP TRIGGER members_last_owner_guard ON members;
DROP FUNCTION members_keep_owner();
DROP TABLE invites;
DROP TABLE subscriptions;
ALTER TABLE orgs
    DROP CONSTRAINT orgs_slug_key,
    DROP COLUMN slug,
    DROP COLUMN home_region,
    DROP COLUMN plan,
    DROP COLUMN deletion_scheduled_at;
