-- T11.2: the subscription lifecycle state machine. Extends the T2.7 inert row
-- with the full status vocabulary (adds trial + cancelled_at_anchor) and the
-- dunning/cancel timestamps the state machine computes days + retries from.
-- Status is the single master state; the API's dunning{} object and wind_down{}
-- are DERIVED from status + these timestamps (never a second source of truth).
ALTER TABLE subscriptions DROP CONSTRAINT subscriptions_status_check;
ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_status_check
    CHECK (status IN ('trial', 'current', 'grace', 'provisioning_paused', 'suspended', 'cancelled_at_anchor'));

ALTER TABLE subscriptions ADD COLUMN dunning_started_at timestamptz; -- null unless in a billing-failure track
ALTER TABLE subscriptions ADD COLUMN next_retry_at       timestamptz; -- scheduled payment retry (dunning)
ALTER TABLE subscriptions ADD COLUMN plan_ends_at        timestamptz; -- cancel wind-down: the plan stops here (resources keep running)
ALTER TABLE subscriptions ADD COLUMN updated_at          timestamptz NOT NULL DEFAULT now();
