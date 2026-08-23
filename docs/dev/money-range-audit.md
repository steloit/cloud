# Auditing stored monetary amounts (O20)

**Who this is for:** support, when a customer reports a permanent `409` on every
create or every PATCH, with a remediation saying "contact support".

## What happened

Before O16, the pricing engine could produce an amount outside the range
`money.Cents` admits, and that value was **persisted**. Measured on `main` at
`28fa30f`:

```
POST /v1/estimates  postgres {"storage_gb": 1000000000000000000}
  -> 200, monthly_total_cents = -5340232221128652948
POST /v1/envs/{env}/services  (with that estimate)
  -> 201, the negative value PERSISTED in monthly_estimate_cents
```

O16 closed the ingress. It did **not** clean up rows already written.

## Why the symptom is now a BLOCK, not a bypass

This is the part support needs to know, because the failure mode inverted.

- **Before O16** a poisoned row silently *disabled* the org's spend cap:
  `enforceBudget` projects against `SumOrgMonthlyEstimate`, so one row at
  ~-5.3e18 left the committed run-rate hugely negative and every later create
  looked far under the cap.
- **After O16** the same row makes the platform **fail closed**: the org gets a
  permanent `409` on every create (if it has a budget row) and the service gets a
  permanent `409` on every PATCH. Both are correct. Both are escapable only by
  fixing the data.

## The bound

`money.MaxMonthly` = **3,443,612,618,300** cents. It is derived
(`math.MaxInt64 / secondsInLongestMonth`), so if it ever changes, re-derive the
number below from `services/api/internal/platform/money/money.go` rather than
trusting this copy.

A stored amount is out of range when it is `< 0` or `> 3443612618300`.

## Detection — all five places money is stored

Run all of these. Checking one column would find one class of poisoned row and
report the database clean.

```sql
-- 1. services.monthly_estimate_cents — the one the overflow actually wrote.
--    `services` has no org_id: the chain is services -> environments ->
--    projects -> org_id. Two hops, not one.
SELECT 'services' AS src, s.id, p.org_id, s.monthly_estimate_cents AS cents
  FROM services s
  JOIN environments e ON e.id = s.env_id
  JOIN projects     p ON p.id = e.project_id
 WHERE s.monthly_estimate_cents < 0 OR s.monthly_estimate_cents > 3443612618300

UNION ALL
-- 2. estimates.total_cents
SELECT 'estimates.total', id, org_id, total_cents
  FROM estimates
 WHERE total_cents < 0 OR total_cents > 3443612618300

UNION ALL
-- 3. budgets.limit_cents — a stored value enforceBudget re-validates (O26)
SELECT 'budgets', org_id, org_id, limit_cents
  FROM budgets
 WHERE limit_cents < 0 OR limit_cents > 3443612618300

UNION ALL
-- 4. usage_events.rate_cents — what billing derives charges from (see O19)
SELECT 'usage_events', id, org_id, rate_cents
  FROM usage_events
 WHERE rate_cents < 0 OR rate_cents > 3443612618300;
```

```sql
-- 5. estimates.lines (jsonb) — PricedShapes refuses to decode these, so they
--    surface as a 409 telling the caller to re-price. The scalar total_cents
--    above can be in range while a LINE inside is not, so this is a separate
--    check, not a duplicate of (2).
SELECT e.id, e.org_id, l->>'name' AS line, (l->>'monthly_cents')::numeric AS cents
  FROM estimates e, LATERAL jsonb_array_elements(e.lines) l
 WHERE (l->>'monthly_cents')::numeric < 0
    OR (l->>'monthly_cents')::numeric > 3443612618300;
```

**Per row, not per org.** `enforceBudget` re-validates the *aggregate*
(`SumOrgMonthlyEstimate`), so two poisoned rows of near-opposite sign sum back
into range, pass `FromInt`, and the cap is then evaluated as if neither existed.
The queries above are deliberately per-row for that reason.

## If rows are found

**Do not** weaken the 409s to make a poisoned row usable — that reopens the
bypass. The choice between re-pricing from the current table and nulling the
column changes a number a customer may have been shown, so it is founder-visible.
Record the count and the decision in `tasks/eops/O20.md`.

## If zero rows are found

Say so in O20 and close it. The queries above are the deliverable; there is
nothing to build.
