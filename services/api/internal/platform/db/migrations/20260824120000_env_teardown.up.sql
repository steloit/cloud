-- US-3.3b: an environment's NAMESPACE outlives the environment.
--
-- `deletion_scheduled_at` (T4.7) records that the customer asked for teardown.
-- Nothing has ever read it: the agent's teardown is per-SERVICE, so a deleted
-- environment left its namespace behind — a live tenant boundary for an
-- environment that no longer exists.
--
-- `torn_down_at` is the OTHER half: the cell confirming the namespace is gone.
-- Two columns rather than one status, because they are written by different
-- planes — the control plane schedules, the cell confirms — and a single column
-- could not distinguish "asked for" from "done".
--
-- The row itself is still never hard-DELETEd (T4.7: deployments reference envs
-- ON DELETE RESTRICT and history is immutable, DP1). This records the teardown;
-- it does not remove the record of the environment.
ALTER TABLE environments
    ADD COLUMN torn_down_at timestamptz;

-- The reconciler's poll asks "which environments in this cell still need their
-- namespace removed", which is a scan of scheduled-but-not-torn-down rows. The
-- partial index keeps that to the rows that can actually match: in steady state
-- almost every environment is neither scheduled nor torn down.
CREATE INDEX environments_teardown_outstanding_idx
    ON environments (deletion_scheduled_at)
    WHERE deletion_scheduled_at IS NOT NULL AND torn_down_at IS NULL;
