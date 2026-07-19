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
