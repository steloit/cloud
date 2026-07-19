ALTER TABLE subscriptions DROP COLUMN IF EXISTS updated_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS plan_ends_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS next_retry_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS dunning_started_at;
ALTER TABLE subscriptions DROP CONSTRAINT subscriptions_status_check;
ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_status_check
    CHECK (status IN ('current', 'grace', 'provisioning_paused', 'suspended'));
