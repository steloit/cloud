// Package invoice is the M7 invoice generator (T11.3): it freezes the meter on
// the billing anchor into an invoice whose lines each carry a usage_ref back to
// the B2 rows (an invoice is DATA, B3). One invoice per (org, period), so a
// monthly close is idempotent — the meter is frozen once, never double-billed.
// Every line and total is integer cents (ADR-025); the invoice reads the SAME
// billing table (plan fee) and quota_usage (metered) the estimate does, so the
// shown estimate line IS the invoice line by construction.
package invoice

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// Line is one invoice line — a description, the cents, and the usage_ref that
// expands to the meter rows behind it (matches the openapi Invoice.lines shape).
type Line struct {
	Description string  `json:"description"`
	ProjectID   *string `json:"project_id"`
	Cents       int64   `json:"cents"`
	UsageRef    string  `json:"usage_ref"`
}

// Store is the persistence the generator needs (sqlc queries satisfy it).
type Store interface {
	GetOrg(ctx context.Context, id string) (store.Org, error)
	GetQuotaUsage(ctx context.Context, arg store.GetQuotaUsageParams) ([]store.QuotaUsage, error)
	UpsertInvoiceForPeriod(ctx context.Context, arg store.UpsertInvoiceForPeriodParams) (store.Invoice, error)
	GetInvoiceForPeriod(ctx context.Context, arg store.GetInvoiceForPeriodParams) (store.Invoice, error)
}

type Service struct {
	q     Store
	plans *billing.Table
	now   func() time.Time
}

func NewService(q Store, plans *billing.Table) *Service {
	return &Service{q: q, plans: plans, now: time.Now}
}

func (s *Service) WithClock(now func() time.Time) *Service { s.now = now; return s }

// Close freezes the org's meter for a period into an OPEN invoice: the plan fee
// (from the one billing table, priced by the org's OWN plan — never a
// caller-supplied string) plus a line per metered accrual, each with a
// usage_ref. Idempotent — a re-close returns the existing invoice unchanged
// (the ON CONFLICT DO NOTHING gate), so a monthly close can be retried safely.
func (s *Service) Close(ctx context.Context, orgID, period string) (store.Invoice, error) {
	// The plan fee is sourced from the org's own plan row — a wrong fee would be
	// frozen permanently, so it is never trusted to a caller argument.
	org, err := s.q.GetOrg(ctx, orgID)
	if err != nil {
		return store.Invoice{}, err
	}
	plan := org.Plan
	usage, err := s.q.GetQuotaUsage(ctx, store.GetQuotaUsageParams{OrgID: orgID, Period: period})
	if err != nil {
		return store.Invoice{}, err
	}

	var lines []Line
	var total int64
	// the subscription/plan fee (from the ONE pricing table; enterprise is
	// custom-priced so PlanFeeCents returns !ok and no fee line is generated).
	if fee, ok := s.plans.PlanFeeCents(plan); ok && fee > 0 {
		lines = append(lines, Line{Description: plan + " plan", Cents: int64(fee), UsageRef: "plan:" + period})
		total += int64(fee)
	}
	// one line per metered accrual (the rollup already priced it into rate_cents).
	for _, u := range usage {
		if u.RateCents == 0 {
			continue
		}
		lines = append(lines, Line{
			Description: u.Meter, Cents: u.RateCents,
			UsageRef: "meter:" + u.Meter + ":" + period,
		})
		total += u.RateCents
	}

	linesJSON, err := json.Marshal(lines)
	if err != nil {
		return store.Invoice{}, err
	}
	inv, err := s.q.UpsertInvoiceForPeriod(ctx, store.UpsertInvoiceForPeriodParams{
		ID: ids.New("inv"), OrgID: orgID, Period: period, Status: "open",
		TotalCents: total, Lines: linesJSON, Tax: nil,
		ClosedAt: pgtype.Timestamptz{Time: s.now(), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// already closed for this period — return the frozen invoice unchanged.
		return s.q.GetInvoiceForPeriod(ctx, store.GetInvoiceForPeriodParams{OrgID: orgID, Period: period})
	}
	return inv, err
}
