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
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/steloit/cloud/services/api/internal/metering"
	"math"
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
	UnappliedCarryForward(ctx context.Context, arg store.UnappliedCarryForwardParams) ([]store.UsageCarryForward, error)
	ClaimCarryForward(ctx context.Context, arg store.ClaimCarryForwardParams) ([]store.UsageCarryForward, error)
	GetInvoiceForPeriod(ctx context.Context, arg store.GetInvoiceForPeriodParams) (store.Invoice, error)
}

type Service struct {
	q     Store
	db    *pgxpool.Pool // when set, Close runs as ONE transaction (see Close)
	plans *billing.Table
	now   func() time.Time
}

func NewService(db *pgxpool.Pool, plans *billing.Table) *Service {
	return &Service{q: store.New(db), db: db, plans: plans, now: time.Now}
}

func (s *Service) WithClock(now func() time.Time) *Service { s.now = now; return s }

// Close freezes the org's meter for a period into an OPEN invoice: the plan fee
// (from the one billing table, priced by the org's OWN plan — never a
// caller-supplied string) plus a line per metered accrual, each with a
// usage_ref. Idempotent — a re-close returns the existing invoice unchanged
// (the ON CONFLICT DO NOTHING gate), so a monthly close can be retried safely.
func (s *Service) Close(ctx context.Context, orgID, period string) (store.Invoice, error) {
	// A PERIOD THAT HAS NOT ENDED CANNOT BE CLOSED. Without this, closing early
	// silently converts the REST OF THE MONTH into "late usage": measured,
	// closing 2026-07 on the 2nd froze the first day and turned 86,400s of
	// ordinary in-period usage into a carry-forward billed on a later invoice.
	// Closing is an accounting boundary, so it must be bounded by the calendar
	// rather than by whoever calls it.
	_, end, err := metering.PeriodBounds(period)
	if err != nil {
		return store.Invoice{}, err
	}
	if s.now().Before(end) {
		return store.Invoice{}, fmt.Errorf(
			"invoice: period %s has not ended (ends %s) — closing it early would turn the rest of the month into late usage",
			period, end.Format("2006-01-02"))
	}
	// ONE TRANSACTION. Closing a period reads the meter, claims carried usage and
	// writes the invoice; without a transaction two concurrent closes both read
	// the same unapplied carries and both bill them, and the loser's claim is a
	// silent no-op. Money must not depend on the two statements happening to
	// interleave favourably.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return store.Invoice{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit
	q := store.New(tx)

	// The plan fee is sourced from the org's own plan row — a wrong fee would be
	// frozen permanently, so it is never trusted to a caller argument.
	org, err := q.GetOrg(ctx, orgID)
	if err != nil {
		return store.Invoice{}, err
	}
	plan := org.Plan
	usage, err := q.GetQuotaUsage(ctx, store.GetQuotaUsageParams{OrgID: orgID, Period: period})
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
	// The metered rows are summed with a CHECKED addition below, not `+=` and NOT
	// billing.SpendToDate.
	//
	// SpendToDate saturates to MaxInt64, which is right for a figure that must be
	// RENDERED (the billing overview) and catastrophic for one that is FROZEN. An
	// invoice total is a charge, and there is no safe direction in which a charge
	// can be MaxInt64. Measured: one `quota_usage` row at -500 (the column is
	// `bigint NOT NULL` with no CHECK) froze TotalCents at 9223372036854775807
	// against Σlines of 30200 — and UpsertInvoiceForPeriod is ON CONFLICT DO
	// NOTHING, so that $92-quadrillion invoice is permanent. An earlier revision
	// of this branch introduced exactly that while trying to fix a wrap.
	//
	// So: refuse to close. The invariant an invoice owes is Σ(lines) == total, and
	// an invoice that cannot honour it must not exist.
	// one line per metered accrual (the rollup already priced it into rate_cents).
	for _, u := range usage {
		if u.RateCents == 0 {
			continue
		}
		lines = append(lines, Line{
			Description: u.Meter, Cents: u.RateCents,
			UsageRef: "meter:" + u.Meter + ":" + period,
		})
		if u.RateCents < 0 || total > math.MaxInt64-u.RateCents {
			return store.Invoice{}, fmt.Errorf(
				"invoice: %s/%s cannot be totalled: meter %q contributes %d to a running total of %d",
				orgID, period, u.Meter, u.RateCents, total)
		}
		total += u.RateCents
	}

	// O39: LATE USAGE FROM A CLOSED PERIOD IS BILLED HERE, or carrying it forward
	// would have been a more elaborate way of losing it. Each carried delta is its
	// own line naming the period it belongs to, so the customer sees "late usage
	// from 2026-07" on their August invoice rather than an unexplained increase.
	// ORIGIN < PERIOD. Without it, closing an EARLIER period bills a later
	// period's late usage: measured, `Close(org, "2026-06")` after July's carry
	// produced a June invoice line reading "late usage from 2026-07". An invoice
	// dated June carrying July's usage is not defensible to a customer or an
	// auditor. The query also excludes credits — recording an over-bill is an
	// engineering obligation, refunding it is a commercial decision.
	carried, err := q.UnappliedCarryForward(ctx, store.UnappliedCarryForwardParams{
		OrgID: orgID, OriginPeriod: period,
	})
	if err != nil {
		return store.Invoice{}, err
	}
	// CLAIM BEFORE BILLING. Only the ids that actually produce a line are claimed,
	// and only the rows the claim RETURNS are billed — so a row another close took
	// first never reaches a line here. Inside the transaction, so a failure below
	// releases the claim rather than stranding the money as billed-but-invoiceless.
	var candidateIDs []string
	for _, cf := range carried {
		if cf.RateCents != 0 {
			candidateIDs = append(candidateIDs, cf.ID)
		}
	}
	var claimed []store.UsageCarryForward
	if len(candidateIDs) > 0 {
		claimed, err = q.ClaimCarryForward(ctx, store.ClaimCarryForwardParams{
			OrgID: orgID, AppliedPeriod: pgtype.Text{String: period, Valid: true}, Column3: candidateIDs,
		})
		if err != nil {
			return store.Invoice{}, err
		}
	}
	for _, cf := range claimed {
		lines = append(lines, Line{
			Description: "late usage from " + cf.OriginPeriod,
			Cents:       cf.RateCents,
			UsageRef:    "carry:" + cf.Meter + ":" + cf.OriginPeriod,
		})
		// Same rule as the meter loop above: refuse to close rather than write a
		// total that does not equal Σ(lines).
		if cf.RateCents < 0 || total > math.MaxInt64-cf.RateCents {
			return store.Invoice{}, fmt.Errorf(
				"invoice: %s/%s cannot be totalled: carried usage from %s contributes %d to a running total of %d",
				orgID, period, cf.OriginPeriod, cf.RateCents, total)
		}
		total += cf.RateCents
	}

	linesJSON, err := json.Marshal(lines)
	if err != nil {
		return store.Invoice{}, err
	}
	inv, err := q.UpsertInvoiceForPeriod(ctx, store.UpsertInvoiceForPeriodParams{
		ID: ids.New("inv"), OrgID: orgID, Period: period, Status: "open",
		TotalCents: total, Lines: linesJSON, Tax: nil,
		ClosedAt: pgtype.Timestamptz{Time: s.now(), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Already closed. The transaction is ROLLED BACK by the deferred call, so
		// any rows this attempt claimed are released — they were not billed, and
		// claiming them would lose them. Read the frozen invoice outside the tx.
		return s.q.GetInvoiceForPeriod(ctx, store.GetInvoiceForPeriodParams{OrgID: orgID, Period: period})
	}
	if err != nil {
		return inv, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Invoice{}, fmt.Errorf("invoice: close %s/%s did not commit: %w", orgID, period, err)
	}
	// Marked applied only after the invoice that carries them actually exists, so
	// a failure anywhere above leaves them unapplied and they land on the next
	// close instead of vanishing.
	return inv, nil
}
