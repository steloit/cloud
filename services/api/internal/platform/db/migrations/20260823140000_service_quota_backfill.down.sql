-- Reverse of the backfill: return every desired doc to having no envelope, which
-- is the shape the pre-US-3.3e agent expects. This necessarily also strips
-- envelopes written after the deploy — that is correct for a down, since the
-- agent being rolled back to does not read the key at all, and leaving it would
-- put a field in the doc that nothing on either side of the wire owns.
--
-- generation is bumped for the same reason as the up: the agent must re-poll.
UPDATE services
SET desired    = desired - 'quota',
    generation = generation + 1
WHERE desired ? 'quota';
