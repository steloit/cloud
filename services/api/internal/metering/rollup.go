package metering

// T6.3: the rollup — quota_usage(org, meter, period) derived from the RAW
// span edges. Idempotent by construction: a pure recomputation upserted over
// the previous value; raw usage_events are never touched. The scheduler
// (River, per architecture §5) invokes Rollup periodically once internal
// jobs land; until then callers invoke it directly (billing reads, tests).

import (
	"context"
	"fmt"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/money"
)

// tstz wraps a time for a timestamptz parameter.
func tstz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// Period is the billing month key, YYYY-MM (UTC).
func Period(t time.Time) string { return t.UTC().Format("2006-01") }

func periodBounds(period string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("metering: bad period %q (want YYYY-MM)", period)
	}
	return start, start.AddDate(0, 1, 0), nil
}

// Rollup recomputes one org's service_span meters for a period. Span
// seconds pair close edges with their open; spans open at the period end
// accrue up to `now` (or the period end, whichever is earlier). rate_cents
// aggregates Σ(seconds × monthly-rate) so billing can derive charges
// without re-reading raw events.
func (e *Emitter) Rollup(ctx context.Context, orgID, period string, now time.Time) error {
	start, end, err := periodBounds(period)
	if err != nil {
		return err
	}
	cutoff := end
	if now.Before(end) {
		cutoff = now
	}
	edges, err := e.q.SpanEdgesForOrg(ctx, store.SpanEdgesForOrgParams{OrgID: orgID, At: tstz(cutoff)})
	if err != nil {
		return err
	}
	// O19: `weighted` accumulates Σ(seconds × monthly rate) across EVERY span of
	// EVERY service in the org. money.MaxMonthly is derived so that ONE
	// service-month exactly fits an int64 — which makes the SECOND service the
	// wrap, silently, in the number billing derives charges from. money.Accrual
	// carries 128 bits, so the accumulation cannot overflow at all; the only
	// place the value can be lost is the narrowing at the write, which fails
	// loudly. See ADR-0014.
	var totalSeconds int64
	var weighted money.Accrual
	var accErr error
	open := map[string]struct {
		at   time.Time
		rate int64
	}{}
	// accrue is the ONE place seconds and a rate are combined, so there is no
	// second arm to fix later. An unrepresentable rate is recorded rather than
	// skipped: dropping it would under-bill silently, which is the failure the
	// money type exists to prevent.
	accrue := func(secs, rate int64) {
		totalSeconds += secs
		if accErr != nil {
			return
		}
		r, err := money.FromInt(rate)
		if err != nil {
			accErr = fmt.Errorf("metering: span rate %d is not a representable amount: %w", rate, err)
			return
		}
		weighted, accErr = weighted.AddMul(r, secs)
	}
	credit := func(from time.Time, rate int64) {
		a, b := from, cutoff
		if a.Before(start) {
			a = start
		}
		if b.After(cutoff) {
			b = cutoff
		}
		if b.After(a) {
			accrue(int64(b.Sub(a)/time.Second), rate)
		}
	}
	for _, ed := range edges {
		switch ed.Edge {
		case "open":
			open[ed.ServiceID] = struct {
				at   time.Time
				rate int64
			}{ed.At.Time, ed.RateCents}
		case "close":
			if o, ok := open[ed.ServiceID]; ok {
				a, b := o.at, ed.At.Time
				if a.Before(start) {
					a = start
				}
				if b.After(cutoff) {
					b = cutoff
				}
				if b.After(a) {
					accrue(int64(b.Sub(a)/time.Second), o.rate)
				}
				delete(open, ed.ServiceID)
			}
		}
	}
	for _, o := range open { // still-running spans accrue to the cutoff
		credit(o.at, o.rate)
	}
	if accErr != nil {
		// LOUD, and it does not write. A rollup that cannot be represented must
		// not upsert a partial or wrapped figure over the previous value: the
		// row stays as it was, the period is visibly not recomputed, and the
		// error propagates to the caller. THAT CALLER IS NOT A SCHEDULER — there
		// is none yet (see this file's header): the only production caller is
		// GetUsage's recompute-on-read, so an unrepresentable accrual makes
		// `GET /orgs/{org}/usage` a 500 for that org and period until the data
		// changes, and usage_events is append-only so there is no operator
		// correction path. That is still better than writing a wrapped figure
		// the invoice and the hard cap would both believe, but the blast radius
		// is real and is recorded in O30 rather than left implied.
		// Silently storing zero would read as "this org used nothing".
		slog.Error("ROLLUP OVERFLOW — quota_usage NOT written, billing figure would be wrong (D10)",
			"org", orgID, "period", period, "seconds", totalSeconds, "err", accErr)
		return fmt.Errorf("metering: rollup %s/%s: %w", orgID, period, accErr)
	}
	weightedCents, err := weighted.Int64()
	if err != nil {
		slog.Error("ROLLUP OVERFLOW — quota_usage NOT written, billing figure would be wrong (D10)",
			"org", orgID, "period", period, "seconds", totalSeconds, "accrual", weighted.String(), "err", err)
		return fmt.Errorf("metering: rollup %s/%s: %w", orgID, period, err)
	}
	// O39: A CLOSED PERIOD IS FROZEN. Recomputing one is not an error — GET
	// /orgs/{org}/usage takes a caller-supplied month and calls Rollup on the READ
	// path, so erroring here would make viewing any past month a permanent 500
	// with a remediation that cannot succeed. The application asks politely; the
	// database refuses regardless (quota_usage_no_write_after_close), so a writer
	// that forgets to ask cannot bypass it.
	closed, err := e.q.PeriodIsClosed(ctx, store.PeriodIsClosedParams{OrgID: orgID, Period: period})
	if err != nil {
		return err
	}
	if closed {
		return e.carryForward(ctx, orgID, period, totalSeconds, weightedCents)
	}
	return e.q.UpsertQuotaUsage(ctx, store.UpsertQuotaUsageParams{
		OrgID: orgID, Meter: "service_span_seconds", Period: period,
		Used: totalSeconds, RateCents: weightedCents,
	})
}

// carryForward records usage that belongs to a period whose invoice is already
// closed, as a DELTA against the frozen figure.
//
// The market offers three answers and they are not equivalent. Stripe drops past
// its grace window — and documents that under a mid-cycle price change the late
// usage lands on NEITHER the current nor a later invoice, i.e. it is silently
// lost. Metronome voids and regenerates. Lago carries forward for recurring
// meters.
//
// Carry-forward is the only one that neither loses money silently nor changes a
// number the customer has already been shown: dropping is invisible under-billing,
// and restating destroys what "closed" means. The delta is recomputed from the
// frozen row every time, so re-detecting the same shortfall is idempotent by
// construction rather than by a guard.
func (e *Emitter) carryForward(ctx context.Context, orgID, period string, totalSeconds, weightedCents int64) error {
	frozen, err := e.q.GetQuotaUsage(ctx, store.GetQuotaUsageParams{OrgID: orgID, Period: period})
	if err != nil {
		return err
	}
	var billedSeconds, billedCents int64
	for _, u := range frozen {
		if u.Meter == "service_span_seconds" {
			billedSeconds, billedCents = u.Used, u.RateCents
		}
	}
	deltaSeconds, deltaCents := totalSeconds-billedSeconds, weightedCents-billedCents
	if deltaSeconds <= 0 && deltaCents <= 0 {
		return nil // nothing late; the recompute agrees with the frozen figure
	}
	slog.WarnContext(ctx, "late usage for a CLOSED period — carried forward, not dropped and not restated (O39)",
		"org", orgID, "origin_period", period, "delta_seconds", deltaSeconds, "delta_cents", deltaCents)
	return e.q.RecordCarryForward(ctx, store.RecordCarryForwardParams{
		ID: ids.New("cf"), OrgID: orgID, Meter: "service_span_seconds",
		OriginPeriod: period, Used: deltaSeconds, RateCents: deltaCents,
	})
}

// Usage returns the stored rollup rows for an org+period (billing/report reads).
func (e *Emitter) Usage(ctx context.Context, orgID, period string) ([]store.QuotaUsage, error) {
	return e.q.GetQuotaUsage(ctx, store.GetQuotaUsageParams{OrgID: orgID, Period: period})
}
