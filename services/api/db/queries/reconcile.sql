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

-- name: ListEnvironmentTeardownsForCell :many
-- The environment half of the agent's poll: environments in this cell whose
-- namespace still has to be removed.
--
-- THE `NOT EXISTS` IS A SAFETY GATE, NOT AN OPTIMISATION. Deleting a namespace
-- deletes everything in it, so advertising an environment before its services
-- are actually gone would tear down a still-terminating CNPG cluster out from
-- under US-3.5's final-backup contract. DeleteEnvironment only requires every
-- service to have REACHED `deleting`; that is not the same as torn down, so the
-- gate is here rather than there.
--
-- "Actually gone" is `status = 'deleting' AND observed_generation >= generation`
-- — the cell converged the teardown and reported it (US-3.3h: `deleting` + `gone`
-- converges). It is NOT row absence: nothing in the tree deletes a service row.
--
-- cell_id lives on PROJECTS, not on environments, hence the join.
SELECT e.id, e.project_id, p.org_id
FROM environments e
JOIN projects p ON p.id = e.project_id
WHERE p.cell_id = $1
  AND e.deletion_scheduled_at IS NOT NULL
  AND e.torn_down_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM services s
      WHERE s.env_id = e.id
        AND (s.status <> 'deleting' OR s.observed_generation < s.generation)
  )
ORDER BY e.deletion_scheduled_at, e.id
LIMIT $2;

-- name: MarkEnvironmentTornDown :execrows
-- The cell confirming the namespace is gone. Guarded on both halves so a replay
-- is a no-op and a report for an environment nobody scheduled cannot invent a
-- teardown: 0 rows means "not eligible", which the caller renders as a refusal
-- rather than a success.
UPDATE environments
SET torn_down_at = now()
WHERE id = $1 AND deletion_scheduled_at IS NOT NULL AND torn_down_at IS NULL;

-- name: GetEnvironmentForCell :one
-- Resolves an environment WITHIN a cell, so a reconciler token cannot confirm a
-- teardown for an environment on a cell it does not own. Existence is not leaked
-- (the caller renders 404 for both "no such environment" and "not your cell"),
-- the same rule the service path follows.
SELECT e.id, e.project_id, p.org_id, e.deletion_scheduled_at, e.torn_down_at
FROM environments e
JOIN projects p ON p.id = e.project_id
WHERE e.id = $1 AND p.cell_id = $2;
