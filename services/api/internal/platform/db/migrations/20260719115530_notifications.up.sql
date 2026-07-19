-- T10.3: the notification routing matrix. An event on the spine (T2.5) is the
-- single source; each route (bell | email | webhook) is a projection of it.
--   notifications      — the in-app bell (U8): a per-user projection of events.
--   notification_prefs  — per-user routing (P6): channels + quiet hours. ROUTING
--                         ONLY — a pref off never suppresses a recording (glossary law).
--   webhooks            — org outbound (G-plane): reveal-once signing secret
--                         (hash at rest, like a personal token), event filter.
--   webhook_deliveries  — the durable outbox for webhook fan-out, same shape as
--                         email_deliveries (claim-before-send, bounded retry).

CREATE TABLE notifications (
    id         text PRIMARY KEY,             -- ntf_<hex>
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    event_id   text REFERENCES events (id) ON DELETE SET NULL, -- the spine source (null for synthetic)
    kind       text NOT NULL,                -- event kind (lifecycle | security | …)
    title      text NOT NULL,
    body       text NOT NULL DEFAULT '',
    link       text NOT NULL DEFAULT '',
    read       boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
-- the bell reads newest-first per user; keyset pages on (created_at, id).
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC, id DESC);
-- one bell row per (user, spine event): makes InsertNotification's ON CONFLICT
-- DO NOTHING actually dedupe, so an at-least-once router never double-rings.
CREATE UNIQUE INDEX notifications_user_event_uk ON notifications (user_id, event_id) WHERE event_id IS NOT NULL;

CREATE TABLE notification_prefs (
    user_id     text PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    channels    jsonb NOT NULL DEFAULT '{"email":true,"inapp":true}',
    quiet_hours jsonb,                         -- {start "HH:MM", end, timezone} | null
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE webhooks (
    id          text PRIMARY KEY,             -- wbh_<hex>
    org_id      text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    url         text NOT NULL,
    events      text[] NOT NULL DEFAULT '{}', -- event-spine kinds filter ({} = all)
    status      text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    -- The signing secret is reveal-once BUT must be recoverable to sign every
    -- outbound POST — so it is envelope-encrypted (secrets.Seal, AAD-bound to
    -- the webhook id), NOT hashed like a token (which is only ever verified).
    ciphertext  bytea NOT NULL,
    nonce       bytea NOT NULL,
    wrapped_dek bytea NOT NULL,
    dek_nonce   bytea NOT NULL,
    kek_id      text  NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webhooks_org_idx ON webhooks (org_id, created_at DESC);

CREATE TABLE webhook_deliveries (
    id          text PRIMARY KEY,             -- whd_<hex>
    webhook_id  text NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
    event_id    text NOT NULL,                -- spine event id (or a synthetic test id)
    status      text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'failed', 'skipped')),
    status_code integer,                       -- last HTTP status observed
    attempts    integer NOT NULL DEFAULT 0,
    error       text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    -- one delivery per (webhook, event): the idempotency gate for the outbox.
    UNIQUE (webhook_id, event_id)
);
CREATE INDEX webhook_deliveries_pending_idx ON webhook_deliveries (status) WHERE status = 'pending';
