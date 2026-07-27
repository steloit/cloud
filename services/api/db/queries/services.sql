-- T3.3: services — desired state. Status writes go through the guarded
-- machine in Go; SetServiceStatus enforces the FROM state in SQL too.

-- name: InsertService :one
-- desired is the reconciler's input (US-1.3a): populated at creation from the
-- shape so the agent has a real document to render from. generation defaults to
-- 1, observed_generation to 0, so a fresh service is immediately outstanding.
INSERT INTO services (id, env_id, name, product, intent, shape, scaling, provisioning_steps, monthly_estimate_cents, estimate_id, desired)
VALUES (
    sqlc.arg('id'), sqlc.arg('env_id'), sqlc.arg('name'), sqlc.arg('product'), sqlc.arg('intent'),
    sqlc.arg('shape'), sqlc.arg('scaling'), sqlc.arg('provisioning_steps'),
    sqlc.arg('monthly_estimate_cents'), sqlc.arg('estimate_id'), coalesce(sqlc.narg('desired'), '{}'::jsonb)
)
RETURNING *;

-- name: GetService :one
SELECT * FROM services WHERE id = $1;

-- name: ListServicesForEnv :many
SELECT * FROM services WHERE env_id = $1 ORDER BY created_at;

-- name: CountServicesForEnvs :one
SELECT count(*) FROM services s
JOIN environments e ON e.id = s.env_id
WHERE e.project_id = $1 AND s.status <> 'deleting';

-- name: SetServiceStatus :one
UPDATE services SET status = $3, provisioning_steps = coalesce(sqlc.narg('steps'), provisioning_steps)
WHERE id = $1 AND status = $2
RETURNING *;

-- name: UpdateServiceShape :one
-- FENCED ON GENERATION. The handler reads the service, prices from that read,
-- and writes back — so a concurrent edit that lands in between is silently
-- overwritten. That was a stale-desired race before US-3.8; now that the PRICE
-- column is written on every PATCH it is a billing race, and it can put three
-- facts in disagreement at once: the column holding one shape, the cell
-- rendering another from the stale desired doc, and the invoice charging a
-- third rate that repriceSpan cannot detect (both sides of its comparison come
-- from the stale read). A lost race must be a 409 the client re-reads and
-- retries, never a silent overwrite.
-- A desired-state edit (US-1.3a): rewrite desired and BUMP generation so the
-- service becomes outstanding (observed_generation < generation) and the cell
-- re-reconciles. Without the bump a converged service would never see the edit.
UPDATE services SET
    shape = coalesce(sqlc.narg('shape'), shape),
    scaling = coalesce(sqlc.narg('scaling'), scaling),
    override = sqlc.narg('override'),
    monthly_estimate_cents = coalesce(sqlc.narg('monthly_estimate_cents'), monthly_estimate_cents),
    desired = coalesce(sqlc.narg('desired'), desired),
    generation = generation + 1
WHERE id = $1 AND generation = sqlc.arg('generation') AND status <> 'deleting'
RETURNING *;

-- name: OrgForService :one
SELECT p.org_id FROM services s
JOIN environments e ON e.id = s.env_id
JOIN projects p ON p.id = e.project_id
WHERE s.id = $1;

-- name: ListExpiredOverrides :many
-- D22: pins past their 24h expiry. SELECT only — the caller computes the new
-- desired doc and base price, then clears everything in ONE statement, because
-- a clear that commits before the doc is rewritten leaves the row outstanding
-- with a stale pinned doc: the cell polls it, renders the pin, and MarkObserved
-- succeeds — after which nothing bumps generation again and the pin renders
-- forever.
--
-- A pin with NO expires_at is expired BY DEFINITION: "unset" must not mean
-- "forever", and such a row is otherwise unreachable by any sweep.
--
-- REQUIRES PostgreSQL 16+ (`pg_input_is_valid`), the repo's first use of it.
-- On 15 this fails every tick with "function does not exist" and the only
-- symptom is a log line, so the control DB's floor is now 16 — stated here
-- because nothing else records it.
--
-- The timestamp cast is GUARDED, and the guard is ONE ORDERED CASE rather than
-- a chain of OR/AND arms. One row with a malformed expires_at would otherwise
-- abort the whole statement, and the sweep would then fail on every tick,
-- silently, for every customer.
--
-- The previous form was a chain of OR arms ending in
-- `OR (pg_input_is_valid(x) AND x::timestamptz <= now())`, and its inner guards
-- turned out to be DEAD: deleting both of them changed nothing, because an
-- EARLIER arm was already `OR pg_input_is_valid(x) = false` — the same guard,
-- written twice, one arm up. OR short-circuits left to right, so the duplicate
-- shielded the cast and the inner copy could never fire. Delete the earlier arm
-- too and the statement aborts, which is how the mechanism was identified.
-- (An earlier version of this comment blamed query-plan reordering. It did not:
-- the shield was deterministic and the duplication was the whole story. Getting
-- that wrong matters — the fix for "a plan might reorder this" and the fix for
-- "I wrote this guard twice" are different fixes.)
--
-- That said, the old form WAS also leaning on ordering Postgres declines to
-- promise: WHERE clauses are "extensively reprocessed as part of developing an
-- execution plan", and the documented counter-example is exactly
-- `x <> 0 AND y/x > 1.5` still dividing by zero. So CASE remains the right
-- construct — it is the one whose evaluation order IS guaranteed — but it is
-- the forward-looking reason, not the diagnosis.
--
-- One CASE and not a CASE inside an OR. The intermediate fix kept the first two
-- arms in the OR and wrote the third as
-- `CASE WHEN pg_input_is_valid(x) THEN x::timestamptz <= now() ELSE true END`.
-- pg_input_is_valid is strict, so a NULL expiry returned NULL, missed the WHEN,
-- hit that ELSE true, and was swept anyway — which made the explicit
-- `IS NULL` arm deletable with no observable change. Note that this depended on
-- the ELSE being `true`; written the other way it would not have been absorbed.
-- Redundant arms cannot be tested, and an arm nobody can test is an arm nobody
-- can trust to still be doing its job. Flattened, each arm is reachable only by
-- its own input class and deleting any of them fails
-- TestTheSweepClearsEveryDeadPinShapeAndSurvivesAMalformedOne.
SELECT * FROM services
WHERE override IS NOT NULL
  AND status <> 'deleting'
  AND CASE
        -- No expiry at all: not a temporary pin, so it is dead on arrival.
        WHEN (override->>'expires_at') IS NULL THEN true
        -- Go's time.Parse(RFC3339) would reject it, so the API already refuses
        -- to honour the pin. If the sweep disagreed, the row would sit forever:
        -- unhonoured and unclearable. The two liveness implementations must
        -- agree, and this arm is what makes them.
        WHEN (override->>'expires_at') !~ '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})$' THEN true
        -- Shaped like a timestamp but not one ('2026-13-45T99:99:99Z').
        WHEN NOT pg_input_is_valid(override->>'expires_at', 'timestamptz') THEN true
        -- KNOWN BOUNDARY, deliberately uncovered: swapping <= for < survives
        -- mutation and no test can kill it — now() has moved on by the time any
        -- assertion runs. Go pins the half-open window (exactly-at-expiry is
        -- DEAD, TestOverrideLiveness) and <= is the arm that agrees with it; a
        -- bare < would strand a pin for one tick at the exact boundary.
        -- Recorded rather than claimed as covered.
        ELSE (override->>'expires_at')::timestamptz <= now()
      END;

-- name: ClearExpiredOverride :one
-- Clear the pin, restore the unpinned price, rewrite desired, and bump
-- generation — ATOMICALLY.
--
-- FENCED ON GENERATION, not merely on `override IS NOT NULL`. The desired doc
-- and price passed in are computed from the row as it was READ, and rows are
-- processed serially with several round trips each, so a customer edit can land
-- in between. Without the fence that edit is silently reverted — its shape
-- replaced by the pre-edit doc and its price by the pre-edit rate — and since
-- generation was bumped the cell converges on the stale doc and nothing bumps
-- again. A lost race must be a no-op the next tick re-lists, never a rollback.
--
-- `override IS NOT NULL` and `status <> 'deleting'` below are defence in depth
-- and NOT independently testable: every path that NULLs override or starts a
-- delete also bumps generation, so the generation fence always fires first.
-- Recorded rather than removed — but recorded, because this is the same
-- "arm nobody can test" shape as the ListExpiredOverrides lesson above, and an
-- untestable arm that goes unmentioned is how that lesson gets re-learned.
UPDATE services SET
    override = NULL,
    monthly_estimate_cents = $2,
    desired = $3,
    generation = generation + 1
WHERE id = $1 AND generation = $4 AND override IS NOT NULL AND status <> 'deleting'
RETURNING *;

