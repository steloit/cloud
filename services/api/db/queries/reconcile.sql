-- US-1.3: the reconciler protocol (D9/A2.5, e1-substrate-design.md §2). The
-- control plane writes desired state; the cell-agent converges actual and
-- reports back. Nothing in this file provisions anything.

-- name: GetCell :one
-- Cell lookup for token scoping. An unknown cell is a 404 at the handler — a
-- reconciler token must never learn which other cells exist.
SELECT * FROM cells WHERE id = $1;

-- name: ListDesiredForCell :many
-- The agent's poll: every service in this cell whose desired state has moved
-- past what the agent last observed. Level-triggered — the full desired doc is
-- returned every time, so the agent never diffs by memory (§2 step 2).
SELECT id, env_id, name, product, intent, status, shape, scaling,
       desired, generation, observed_generation, cell_id, last_reconciled_at
FROM services
WHERE cell_id = $1 AND generation > $2
ORDER BY generation, id
LIMIT $3;

-- name: MarkObserved :one
-- Status writeback's bookkeeping half: advance observed_generation and stamp
-- last_reconciled_at. The guard `generation >= $2` rejects a STALE report — a
-- slow agent reporting on an older generation must not mark newer desired
-- state as converged. Returns no row when the report is stale.
UPDATE services
SET observed_generation = GREATEST(observed_generation, $2),
    last_reconciled_at  = now()
WHERE id = $1 AND generation >= $2
RETURNING *;

-- name: TouchCellHeartbeat :exec
-- The heartbeat rides the status call (§2 step 4) — there is no separate
-- endpoint to forget to call.
UPDATE cells SET agent_last_seen_at = now() WHERE id = $1;

-- name: BumpServiceGeneration :one
-- Every desired-state edit bumps the generation so the agent notices. Callers
-- are the desired-state writers (createService, shape/scaling edits), never a
-- reconciler endpoint.
UPDATE services
SET desired = $2, generation = generation + 1
WHERE id = $1
RETURNING *;
