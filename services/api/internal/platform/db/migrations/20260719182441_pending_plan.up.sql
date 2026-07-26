-- Q9/US-11.3 slice: a CLEAN downgrade takes effect at the anchor, never
-- immediately (B4). pending_plan holds it and pending_plan_at is the anchor
-- instant the lifecycle sweep applies it. NULL = no pending change.
ALTER TABLE subscriptions ADD COLUMN pending_plan text
    CHECK (pending_plan IN ('free', 'pro', 'business', 'enterprise'));
ALTER TABLE subscriptions ADD COLUMN pending_plan_at timestamptz;
