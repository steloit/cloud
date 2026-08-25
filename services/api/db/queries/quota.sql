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
-- APPEND-ONLY, one row per detection. No conflict clause: a remainder of zero is
-- never recorded, so repeated recomputes append nothing, and a second or nth late
-- arrival appends its own remainder instead of colliding with the first.
INSERT INTO usage_carry_forward (id, org_id, meter, origin_period, used, rate_cents, kind, rate_unit)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: CarriedTotalForOrigin :one
-- What has ALREADY been carried for this origin, applied or not. The remainder is
-- `recomputed - frozen - this`, so a second late arrival cannot re-carry the first.
SELECT coalesce(sum(used), 0)::bigint AS used, coalesce(sum(rate_cents), 0)::bigint AS rate_cents
FROM usage_carry_forward WHERE org_id = $1 AND meter = $2 AND origin_period = $3;

-- name: UnappliedCarryForward :many
-- CHARGES ONLY, and only from a period that ENDED before the one being closed: an
-- invoice dated June must never carry usage from July. Credits are excluded —
-- recording them is an engineering obligation, refunding them is commercial.
SELECT * FROM usage_carry_forward
WHERE org_id = $1 AND applied_period IS NULL AND kind = 'charge' AND origin_period < $2
ORDER BY origin_period;

-- name: MarkCarryForwardApplied :exec
-- BY ID. A blanket `WHERE applied_period IS NULL` stamped every row unapplied at
-- mark time — including ones created after the read, and ones skipped for a zero
-- amount — marking money billed that never reached a line.
UPDATE usage_carry_forward SET applied_period = $2
WHERE org_id = $1 AND id = ANY($3::text[]) AND applied_period IS NULL;

-- name: UnappliedCredits :many
-- Over-billing, surfaced. Not auto-applied.
SELECT * FROM usage_carry_forward
WHERE org_id = $1 AND applied_period IS NULL AND kind = 'credit' ORDER BY origin_period;
