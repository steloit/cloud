DROP INDEX IF EXISTS services_cell_outstanding_idx;
ALTER TABLE services DROP CONSTRAINT IF EXISTS services_cell_id_fkey;
ALTER TABLE services
    DROP COLUMN IF EXISTS last_reconciled_at,
    DROP COLUMN IF EXISTS observed_generation,
    DROP COLUMN IF EXISTS generation,
    DROP COLUMN IF EXISTS desired;
DROP TABLE IF EXISTS cells;
