-- US-1.3: the reconciler protocol (D9/A2.5, e1-substrate-design.md §2). The
-- control plane writes desired state; the cell-agent converges actual and
-- reports back. Nothing in this file provisions anything.

-- name: GetCell :one
-- Cell lookup for token scoping. An unknown cell is a 404 at the handler — a
-- reconciler token must never learn which other cells exist.
SELECT * FROM cells WHERE id = $1;

-- name: ListDesiredForCell :many
-- The agent's poll: every service in this cell with OUTSTANDING work — desired
-- has moved ahead of what the cell has converged (observed_generation <
-- generation). This is the level-triggered controller shape: the control plane
-- knows what needs reconciling, so the agent needs no client-side cursor (a
-- per-row generation cannot be a cell-wide cursor — that starves new services).
-- The full desired doc rides every row, so the agent never diffs by memory.
-- since_generation ($2) is an optional additional lower bound (0 = all work).
SELECT id, env_id, name, product, intent, status, shape, scaling,
       desired, generation, observed_generation, cell_id, last_reconciled_at
FROM services
WHERE cell_id = $1 AND observed_generation < generation AND generation > $2
ORDER BY generation, id
LIMIT $3;

-- name: MarkObserved :one
-- Status writeback's bookkeeping half: record that the cell has converged the
-- CURRENT generation. The guard `generation = $2` is exact — it rejects a
-- report for any generation other than the one desired holds right now:
--   * behind (agent converged an older desired that has since bumped): rejected,
--     so a stale converge never marks newer desired as done or drives status;
--   * ahead (impossible; a replay/bug reporting a generation desired never had):
--     rejected.
-- Returns no row on rejection — the agent re-polls (the row is still outstanding)
-- and converges the current generation. Idempotent: a duplicate report for the
-- current generation re-sets observed to the same value.
UPDATE services
SET observed_generation = $2,
    last_reconciled_at  = now()
WHERE id = $1 AND generation = $2
RETURNING *;

-- name: TouchCellHeartbeat :exec
-- The heartbeat rides the status call (§2 step 4) — there is no separate
-- endpoint to forget to call.
UPDATE cells SET agent_last_seen_at = now() WHERE id = $1;

-- name: BumpServiceGeneration :one
-- Every desired-state EDIT bumps the generation so the agent re-reconciles.
-- A freshly created service is already outstanding (observed 0 < generation 1),
-- so creation needs no bump. Wiring this into updateService/deleteService (so
-- shape edits and deletes reach the cell) is provisioning-side and OUT of
-- US-1.3's scope — tracked as a follow-up (US-1.3a). Reconciler endpoints never
-- call this; only desired-state writers do.
UPDATE services
SET desired = $2, generation = generation + 1
WHERE id = $1
RETURNING *;
