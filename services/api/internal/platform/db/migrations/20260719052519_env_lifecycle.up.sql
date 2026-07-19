-- T4.7: environment lifecycle. `implicit` marks the BORN environment
-- (ADR-037: production is a stable IDENTITY — renameable, so the flag, not
-- the name, carries the identity). Deletion is scheduled, never a hard
-- DELETE: deployments reference envs ON DELETE RESTRICT and history is
-- immutable (DP1).
ALTER TABLE environments
    ADD COLUMN implicit boolean NOT NULL DEFAULT false,
    ADD COLUMN deletion_scheduled_at timestamptz;

-- Backfill: every 'production' env created so far was the born one.
UPDATE environments SET implicit = true WHERE name = 'production';
