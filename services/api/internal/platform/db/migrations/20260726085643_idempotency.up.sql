-- US-3.6 / S7: idempotency for mutating POSTs. Dedupe on (principal, endpoint,
-- key) for 24h; a replay returns the ORIGINAL response, so a client that timed
-- out mid-create cannot burn a second estimate or create a second service.
--
-- The body fingerprint makes "same key, different body" detectable — that is a
-- client bug (a reused key for a different request) and must be a 409, never a
-- silent replay of the wrong response.
CREATE TABLE idempotency_keys (
    -- principal: a PREFIXED identity (`user:<id>` / `orgkey:<token id>` /
    -- `anon:<ip>`) so the namespaces cannot collide — a user id must never be
    -- readable as an org-key id.
    principal   text NOT NULL,
    -- endpoint: `METHOD /concrete/path`, NOT the operationId or route pattern.
    -- With a pattern, one key replayed against a different {env} would return
    -- the first env's resource.
    endpoint    text NOT NULL,
    key         text NOT NULL CHECK (length(key) <= 255),
    body_sha256 text NOT NULL,            -- fingerprint of the request body
    -- claim_token fences writes to the CURRENT owner. Without it, an owner
    -- whose request outran the abandonment window could return and release (or
    -- overwrite) the claim of the successor that legitimately took over —
    -- letting a third request run concurrently with the second.
    claim_token text NOT NULL,
    status_code int,                      -- the original response, replayed verbatim
    -- bytea, NOT jsonb: S7 promises the ORIGINAL response. jsonb re-serializes
    -- on read — it reorders object keys and re-spaces the text — so a replay
    -- would be semantically equal but not the same bytes, and a client that
    -- hashes or diffs the response would see two different answers to one
    -- request. Nothing here queries into the body, so jsonb buys nothing.
    response    bytea,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (principal, endpoint, key)
);

-- Expiry sweep support: entries older than 24h are reusable.
CREATE INDEX idempotency_keys_created_idx ON idempotency_keys (created_at);
