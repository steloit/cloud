-- T6.3: the rollup. RollupServiceSpans recomputes one org+period from the
-- RAW spans — pure derivation, idempotent by construction (upsert of a
-- deterministic aggregate; raw events are never touched).
--
-- Span seconds: close edges pair with the latest earlier open edge per
-- service; spans still open at rollup time accrue to the period bound.

-- name: UpsertQuotaUsage :exec
INSERT INTO quota_usage (org_id, meter, period, used, rate_cents, computed_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (org_id, meter, period)
DO UPDATE SET used = EXCLUDED.used, rate_cents = EXCLUDED.rate_cents, computed_at = now();

-- name: SpanEdgesForOrg :many
SELECT service_id, edge, product, rate_cents, at FROM usage_events
WHERE org_id = $1 AND meter = 'service_span' AND at < $2
ORDER BY service_id, at;

-- name: GetQuotaUsage :many
SELECT * FROM quota_usage WHERE org_id = $1 AND period = $2 ORDER BY meter;

-- O39: is this period's accounting closed? The rollup consults this so the READ
-- path (GET /usage of a past month) keeps working instead of hitting the trigger
-- and 500ing. The trigger is still the enforcement — this is the polite door.
-- name: PeriodIsClosed :one
SELECT EXISTS (SELECT 1 FROM invoices WHERE org_id = $1 AND period = $2);

-- name: RecordCarryForward :exec
-- Idempotent on (org, meter, origin_period): the amount is a DELTA against a
-- frozen number, so re-detecting the same shortfall must not charge twice.
INSERT INTO usage_carry_forward (id, org_id, meter, origin_period, used, rate_cents)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (org_id, meter, origin_period)
DO UPDATE SET used = EXCLUDED.used, rate_cents = EXCLUDED.rate_cents, detected_at = now()
WHERE usage_carry_forward.applied_period IS NULL;

-- name: UnappliedCarryForward :many
SELECT * FROM usage_carry_forward WHERE org_id = $1 AND applied_period IS NULL ORDER BY origin_period;

-- name: MarkCarryForwardApplied :exec
UPDATE usage_carry_forward SET applied_period = $2
WHERE org_id = $1 AND applied_period IS NULL;
