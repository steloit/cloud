-- US-1.3: the reconciler protocol's storage (D9/A2.5, e1-substrate-design.md §2).
-- Control-plane Postgres holds DESIRED state; cells hold ACTUAL. Nothing here
-- provisions — the cell-agent converges and reports back. DR = restore desired,
-- reconcile actual.

-- Cells are the data-plane units (D6/D7). cell-0 is seeded to match the existing
-- services.cell_id default, so the FK below adopts every current row unchanged.
CREATE TABLE cells (
    id                 text PRIMARY KEY,   -- 'cell-0'
    region             text NOT NULL,
    status             text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'draining', 'offline')),
    agent_last_seen_at timestamptz,        -- heartbeat rides the status call (§2 step 4)
    created_at         timestamptz NOT NULL DEFAULT now()
);

INSERT INTO cells (id, region, status) VALUES ('cell-0', 'us-central1', 'active');

-- Desired-state columns. `desired` is what the agent renders from; `generation`
-- bumps on every desired-state edit; `observed_generation` is how far the agent
-- has converged. observed < generation means work is outstanding.
ALTER TABLE services
    ADD COLUMN desired             jsonb       NOT NULL DEFAULT '{}',
    ADD COLUMN generation          bigint      NOT NULL DEFAULT 1,
    ADD COLUMN observed_generation bigint      NOT NULL DEFAULT 0,
    ADD COLUMN last_reconciled_at  timestamptz;

ALTER TABLE services
    ADD CONSTRAINT services_cell_id_fkey FOREIGN KEY (cell_id) REFERENCES cells (id);

-- The desired-poll query shape: WHERE cell_id = $1 AND observed_generation < generation.
-- Partial index — only OUTSTANDING rows are indexed, so the hot poll path never
-- scans converged services.
CREATE INDEX services_cell_outstanding_idx ON services (cell_id, generation)
    WHERE observed_generation < generation;
