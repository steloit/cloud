-- T11.6: the hard spend cap (B1 budget).

-- name: GetBudget :one
SELECT * FROM budgets WHERE org_id = $1;

-- name: SumOrgMonthlyEstimate :one
-- The org's committed monthly run-rate: Σ active services' monthly estimates
-- across all its projects/envs. The hard cap projects against this (+ plan fee)
-- at the accept gate. `deleting` services are on their way out, so excluded;
-- everything else is counted (conservative — the cap must never under-count).
SELECT coalesce(sum(s.monthly_estimate_cents), 0)::bigint AS cents
FROM services s
JOIN environments e ON e.id = s.env_id
JOIN projects p ON p.id = e.project_id
WHERE p.org_id = $1 AND s.status <> 'deleting';

-- name: UpsertBudget :one
INSERT INTO budgets (org_id, limit_cents, alert_thresholds)
VALUES ($1, $2, $3)
ON CONFLICT (org_id) DO UPDATE
    SET limit_cents = EXCLUDED.limit_cents,
        alert_thresholds = EXCLUDED.alert_thresholds,
        updated_at = now()
RETURNING *;
