-- T11.6: the hard spend cap (B1 budget).

-- name: GetBudget :one
SELECT * FROM budgets WHERE org_id = $1;

-- name: UpsertBudget :one
INSERT INTO budgets (org_id, limit_cents, alert_thresholds)
VALUES ($1, $2, $3)
ON CONFLICT (org_id) DO UPDATE
    SET limit_cents = EXCLUDED.limit_cents,
        alert_thresholds = EXCLUDED.alert_thresholds,
        updated_at = now()
RETURNING *;
