package identity

// T11.6 the hard spend cap (B1 budget, F9 flagship) + the billing overview that
// reports it. The bound is ENFORCED at the estimate-accept gate
// (provisioning) — this file owns the set/read surface. The MTD number here is
// the SAME arithmetic the invoice freezes and the cap enforces (one arithmetic
// everywhere): plan fee + Σ quota_usage.rate_cents.

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// currentPeriod is the billing period a meter accrues into (YYYY-MM, UTC).
func currentPeriod() string { return time.Now().UTC().Format("2006-01") }

// mtdSpend is the org's month-to-date spend: plan fee + every metered accrual.
// Returns (planFee, metered, total) so the overview can show the split. Uses the
// org's OWN plan (never a caller argument) — the same source the invoice trusts.
func (s *Service) mtdSpend(ctx context.Context, orgID string) (planFee, metered, total int64, err error) {
	org, err := s.q.GetOrg(ctx, orgID)
	if err != nil {
		return 0, 0, 0, err
	}
	if fee, ok := s.plans.PlanFeeCents(org.Plan); ok {
		planFee = int64(fee)
	}
	usage, err := s.q.GetQuotaUsage(ctx, store.GetQuotaUsageParams{OrgID: orgID, Period: currentPeriod()})
	if err != nil {
		return 0, 0, 0, err
	}
	for _, u := range usage {
		metered += u.RateCents
	}
	return planFee, metered, billing.SpendToDate(planFee, metered), nil
}

func (h *Handlers) SetBudget(ctx context.Context, req gen.SetBudgetRequestObject) (gen.SetBudgetResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "billing.manage_payment", true); err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "body", Detail: "required"}}}
	}
	// A null limit removes the cap. A negative bound is nonsense; a bound BELOW
	// current spend is allowed — it just pauses new provisioning immediately (it
	// never deletes a running service).
	var limit pgtype.Int8
	if req.Body.LimitCents != nil {
		if *req.Body.LimitCents < 0 {
			return nil, validationError{fields: []problem.FieldError{{Field: "limit_cents", Detail: "must be >= 0, or null to remove the cap"}}}
		}
		limit = pgtype.Int8{Int64: int64(*req.Body.LimitCents), Valid: true}
	}
	thresholds := []int32{80}
	if req.Body.AlertThresholds != nil {
		thresholds = thresholds[:0]
		for _, t := range *req.Body.AlertThresholds {
			thresholds = append(thresholds, int32(t))
		}
	}
	row, err := h.svc.q.UpsertBudget(ctx, store.UpsertBudgetParams{
		OrgID: req.OrgPathParam, LimitCents: limit, AlertThresholds: thresholds,
	})
	if err != nil {
		return nil, err
	}
	_, _, mtd, err := h.svc.mtdSpend(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	return gen.SetBudget200JSONResponse(budgetToAPI(row, mtd)), nil
}

func (h *Handlers) GetBillingOverview(ctx context.Context, req gen.GetBillingOverviewRequestObject) (gen.GetBillingOverviewResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "billing.view", false); err != nil {
		return nil, err
	}
	planFee, metered, mtd, err := h.svc.mtdSpend(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	out := gen.BillingOverview{}
	mtdI, feeI, resI := int(mtd), int(planFee), int(metered)
	out.MtdCents = &mtdI
	out.PlanFeeCents = &feeI
	out.ResourcesCents = &resI

	budget, err := h.svc.q.GetBudget(ctx, req.OrgPathParam)
	if err == nil {
		b := budgetToAPI(budget, mtd)
		out.Budget = &struct {
			AlertThresholds *[]int   `json:"alert_thresholds,omitempty"`
			LimitCents      *int     `json:"limit_cents,omitempty"`
			UsedPercent     *float32 `json:"used_percent,omitempty"`
		}{AlertThresholds: b.AlertThresholds, LimitCents: b.LimitCents, UsedPercent: b.UsedPercent}
	} else if err != pgx.ErrNoRows {
		return nil, err
	}
	return gen.GetBillingOverview200JSONResponse(out), nil
}

// budgetToAPI maps the stored budget + the current MTD spend into the wire
// shape, computing used_percent (0 when uncapped, so the UI never divides by a
// null cap).
func budgetToAPI(row store.Budget, mtd int64) gen.Budget {
	out := gen.Budget{}
	thresholds := make([]int, 0, len(row.AlertThresholds))
	for _, t := range row.AlertThresholds {
		thresholds = append(thresholds, int(t))
	}
	out.AlertThresholds = &thresholds
	pct := float32(0)
	if row.LimitCents.Valid {
		lc := int(row.LimitCents.Int64)
		out.LimitCents = &lc
		if row.LimitCents.Int64 > 0 {
			pct = float32(mtd) / float32(row.LimitCents.Int64) * 100
		}
	}
	out.UsedPercent = &pct
	return out
}
