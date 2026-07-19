package identity

// US-6.2: the per-org monthly usage report (B2) — the meter behind the
// (future) invoice, drawn from the metering rollup. No invoicing yet
// (billing attaches in E11); this is the meter table, honest and live from
// alpha day one. getUsage triggers a fresh rollup so the report reconciles
// with the raw meter events on read.

import (
	"context"
	"time"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/metering"
)

// usageRoller is the seam the composition root wires (identity doesn't own
// metering). Recompute makes the report reconcile with raw events on read.
type usageRoller interface {
	Rollup(ctx context.Context, orgID, period string, now time.Time) error
	Usage(ctx context.Context, orgID, period string) ([]store.QuotaUsage, error)
}

// spanRateToDollars converts a weighted span (Σ seconds × monthly-cents) into
// the month's prorated dollar figure: cents-seconds / seconds-per-month.
const secondsPerMonth = 30 * 24 * 3600

func (h *Handlers) GetUsage(ctx context.Context, req gen.GetUsageRequestObject) (gen.GetUsageResponseObject, error) {
	p, ok := principalOf(ctx)
	if !ok {
		return nil, ErrNoSession
	}
	if err := h.authz.Require(ctx, p, "billing.view", rbac.Scope{OrgID: req.OrgPathParam}); err != nil {
		return nil, err
	}
	month := metering.Period(time.Now())
	if req.Params.Month != nil && *req.Params.Month != "" {
		month = *req.Params.Month
	}
	if h.usage != nil {
		// recompute-on-read: the report reconciles with the raw spine
		if err := h.usage.Rollup(ctx, req.OrgPathParam, month, time.Now()); err != nil {
			return nil, err
		}
	}
	var rows []store.QuotaUsage
	if h.usage != nil {
		var err error
		if rows, err = h.usage.Usage(ctx, req.OrgPathParam, month); err != nil {
			return nil, err
		}
	}

	type meter = struct {
		Detail       *[]map[string]interface{} `json:"detail,omitempty"`
		Included     *float32                  `json:"included,omitempty"`
		Meter        *string                   `json:"meter,omitempty"`
		OverageCents *int                      `json:"overage_cents,omitempty"`
		OveragePrice *string                   `json:"overage_price,omitempty"`
		Used         *float32                  `json:"used,omitempty"`
	}
	meters := make([]meter, 0, len(rows))
	for _, r := range rows {
		name := r.Meter
		used := float32(r.Used)
		m := meter{Meter: &name, Used: &used}
		// compute-seconds carry a prorated dollar figure (no invoicing —
		// this is the meter, not a charge)
		if r.Meter == "service_span_seconds" && r.RateCents > 0 {
			cents := int(r.RateCents / secondsPerMonth)
			m.OverageCents = &cents
		}
		meters = append(meters, m)
	}
	updated := time.Now()
	return gen.GetUsage200JSONResponse(gen.UsageReport{
		Month: &month, Meters: &meters, UpdatedAt: &updated,
	}), nil
}

// principalOf is a small helper mirroring the session accessor.
func principalOf(ctx context.Context) (session.Principal, bool) {
	return session.PrincipalFrom(ctx)
}
