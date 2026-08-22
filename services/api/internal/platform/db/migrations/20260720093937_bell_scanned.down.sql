-- Reverting drops the record of which events were already projected. Re-running
-- the up migration then re-seeds from the CURRENT spine, so nothing is
-- back-projected — but any events appended between the down and the next up
-- would be seeded as "already scanned" and never ring. Acceptable for a
-- development rollback; do not use it as an operational reset.
DROP INDEX IF EXISTS events_at_id_idx;
DROP TABLE IF EXISTS bell_scanned;
