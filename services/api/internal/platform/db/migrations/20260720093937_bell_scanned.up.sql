-- US-10.3: the bell projection's terminal ledger.
--
-- bell_scanned records EVERY event the projection has looked at — worthy or
-- not. That distinction is the entire design.
--
-- The webhook outbox gets away with "have I written a row for this?" because
-- its deliverability filter lives in SQL (e.kind = ANY(w.events)), so
-- undeliverable events never enter its pending set, and webhook_deliveries
-- records a terminal row for the rest. The bell decides worthiness in Go
-- (notify.Classify), so an unworthy event would leave no notifications row,
-- permanently satisfy `WHERE ... IS NULL`, and occupy a slot in every later
-- LIMIT window. Silence is the COMMON case, so the scan would starve after one
-- batch and the bell would stop ringing forever.
--
-- A timestamp watermark cannot substitute: events.at defaults to now(), which
-- in Postgres is TRANSACTION START time, so a write tx that opens before a scan
-- tick and commits after it lands below any watermark and is skipped
-- permanently and silently. Timestamps order events; they never prove handling.

CREATE TABLE bell_scanned (
    event_id   text PRIMARY KEY REFERENCES events (id),
    scanned_at timestamptz NOT NULL DEFAULT now()
);

-- The anti-join scans events ordered by (at, id) across ALL orgs. The only
-- existing index is events_org_at_idx, which is org-leading and therefore
-- unusable here — without this index every tick is a seq scan + sort of the
-- whole spine.
CREATE INDEX events_at_id_idx ON events (at, id);

-- Seed the ledger with the entire existing spine so the first tick does not fan
-- the org's whole history onto every member's bell. No-backfill then holds by
-- construction rather than by a time predicate.
--
-- This is one-shot and irreversible in effect: once seeded, those events can
-- never be projected. That is exactly why TestPreExistingSpineIsNotBackfilled
-- exercises it against a NON-empty spine — CI migrates an empty database, so a
-- wrong predicate here would otherwise ship silently.
INSERT INTO bell_scanned (event_id)
SELECT id FROM events
ON CONFLICT DO NOTHING;

-- Cost note: golang-migrate runs each file in ONE transaction, so CREATE INDEX
-- CONCURRENTLY is unavailable. The index build takes a SHARE lock on events,
-- blocking spine writes for its duration, and the seed is an unbounded
-- INSERT ... SELECT over the same table. Both are acceptable at current scale;
-- the measured pause and the events row count are reported in the PR.
