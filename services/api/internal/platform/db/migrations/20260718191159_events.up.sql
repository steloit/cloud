-- T2.5: the events spine (GOV-002 primitive 9) — ONE append-only pipeline;
-- /orgs/{org}/audit and /envs/{env}/events are views over this table.
-- Corrections are new events; via provenance is forever.
CREATE TABLE events (
    id      text PRIMARY KEY,                   -- evt_<hex>
    org_id  text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    kind    text NOT NULL,                      -- deploy | scale | alert_state | policy_trigger | lifecycle | membership | billing | …
    via     text NOT NULL CHECK (via IN ('user', 'assistant', 'system')),
    actor   text NOT NULL,                      -- usr_/tok_ id or 'system'
    action  text NOT NULL,                      -- e.g. org.created, member.added
    subject text NOT NULL DEFAULT '',           -- the xxx_ id acted on
    at      timestamptz NOT NULL DEFAULT now(),
    detail  jsonb NOT NULL DEFAULT '{}'
);

CREATE INDEX events_org_at_idx ON events (org_id, at DESC, id DESC);

-- Append-only AT THE DB LEVEL: the app role owns this table, so REVOKE would
-- not bind — a trigger is the only owner-proof enforcement.
CREATE FUNCTION events_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'events is append-only: % is forbidden (corrections are new events)', TG_OP
        USING ERRCODE = 'raise_exception';
END;
$$;

CREATE TRIGGER events_no_update_delete
    BEFORE UPDATE OR DELETE ON events
    FOR EACH ROW EXECUTE FUNCTION events_append_only();
