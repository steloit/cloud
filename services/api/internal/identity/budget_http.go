package identity

// T11.6 the hard spend cap (B1 budget, F9 flagship) + the billing overview that
// reports it. The bound is ENFORCED at the estimate-accept gate
// (provisioning) — this file owns the set/read surface. The MTD number here is
// the SAME arithmetic the invoice freezes and the cap enforces (one arithmetic
// everywhere): plan fee + Σ quota_usage.rate_cents.

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/events"
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

// monthlyRunRate is the org's committed monthly forecast: plan fee + Σ active
// services' monthly estimates — the SAME number the hard cap enforces against
// (provisioning.enforceBudget), so used_percent reflects how close the org is to
// the wall, not just what it has accrued so far.
func (s *Service) monthlyRunRate(ctx context.Context, orgID string) (int64, error) {
	org, err := s.q.GetOrg(ctx, orgID)
	if err != nil {
		return 0, err
	}
	var planFee int64
	if fee, ok := s.plans.PlanFeeCents(org.Plan); ok {
		planFee = int64(fee)
	}
	committed, err := s.q.SumOrgMonthlyEstimate(ctx, orgID)
	if err != nil {
		return 0, err
	}
	return planFee + committed, nil
}

func (h *Handlers) SetBudget(ctx context.Context, req gen.SetBudgetRequestObject) (gen.SetBudgetResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, "billing.manage_payment", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "body", Detail: "required"}}}
	}
	// the prior bound, for the audit trail (a silent cap-lift/removal is a
	// spend-control bypass — every change lands on the spine).
	prior := "none"
	if old, err := h.svc.q.GetBudget(ctx, req.OrgPathParam); err == nil && old.LimitCents.Valid {
		prior = fmt.Sprintf("%d", old.LimitCents.Int64)
	} else if err != nil && err != pgx.ErrNoRows {
		return nil, err
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
	next := "none"
	if row.LimitCents.Valid {
		next = fmt.Sprintf("%d", row.LimitCents.Int64)
	}
	h.svc.record(ctx, events.Input{
		OrgID: req.OrgPathParam, Kind: "billing", Via: "user", Actor: actor,
		Action: "billing.budget_changed", Subject: req.OrgPathParam,
		Detail: []byte(`{"from_limit_cents":"` + prior + `","to_limit_cents":"` + next + `"}`),
	})
	runRate, err := h.svc.monthlyRunRate(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	return gen.SetBudget200JSONResponse(budgetToAPI(row, runRate)), nil
}

func (h *Handlers) GetBillingOverview(ctx context.Context, req gen.GetBillingOverviewRequestObject) (gen.GetBillingOverviewResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "billing.view", false); err != nil {
		return nil, err
	}
	planFee, metered, mtd, err := h.svc.mtdSpend(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	runRate, err := h.svc.monthlyRunRate(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	out := gen.BillingOverview{}
	mtdI, feeI, resI, fcI := int(mtd), int(planFee), int(metered), int(runRate)
	out.MtdCents = &mtdI // actual accrued so far this month
	out.PlanFeeCents = &feeI
	out.ResourcesCents = &resI
	out.ForecastCents = &fcI // committed monthly run-rate (the enforced number)

	budget, err := h.svc.q.GetBudget(ctx, req.OrgPathParam)
	if err == nil {
		// used_percent is run-rate ÷ cap — the ratio the cap actually enforces,
		// so the developer sees how close to the wall they are (not just accrued).
		b := budgetToAPI(budget, runRate)
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

// budgetToAPI maps the stored budget + the spend figure used_percent is measured
// against (the monthly run-rate — the enforced number) into the wire shape.
// used_percent is 0 when uncapped, so the UI never divides by a null cap.
func budgetToAPI(row store.Budget, spendCents int64) gen.Budget {
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
			pct = float32(spendCents) / float32(row.LimitCents.Int64) * 100
		}
	}
	out.UsedPercent = &pct
	return out
}
