-- T10.4: email delivery ledger + durable outbox state machine. Every outbound
-- email is triggered by exactly one spine Event (AC: "nothing but Event kinds
-- sends mail") and recorded here, so delivery is auditable and idempotent.
--
-- Lifecycle: pending (claimed, not yet confirmed sent) → sent | failed;
-- skipped is a terminal "nothing to send" (a vanished invite). The ledger is
-- the outbox: a claim writes `pending` BEFORE the provider call, so a crash
-- mid-send leaves an honest `pending` row that the scan reclaims — never a
-- phantom `sent`. Retries are bounded by `attempts`.
CREATE TABLE email_deliveries (
    id               text PRIMARY KEY,            -- eml_<hex>
    event_id         text NOT NULL,               -- the spine event that triggered it
    org_id           text,                        -- NULL for account-level (e.g. password reset)
    recipient        text NOT NULL,
    template         text NOT NULL,
    template_version integer NOT NULL,
    status           text NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'sent', 'failed', 'skipped')),
    attempts         integer NOT NULL DEFAULT 0,
    provider         text NOT NULL,               -- resend | noop
    provider_id      text,                        -- the provider's message id (sent)
    error            text,                         -- provider error (failed)
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, recipient)
);
CREATE INDEX email_deliveries_org_idx ON email_deliveries (org_id, created_at DESC);
