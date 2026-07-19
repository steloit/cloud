package metering

// T6.3: the rollup — quota_usage(org, meter, period) derived from the RAW
// span edges. Idempotent by construction: a pure recomputation upserted over
// the previous value; raw usage_events are never touched. The scheduler
// (River, per architecture §5) invokes Rollup periodically once internal
// jobs land; until then callers invoke it directly (billing reads, tests).

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/identity/store"
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
	var totalSeconds, weighted int64
	open := map[string]struct {
		at   time.Time
		rate int64
	}{}
	credit := func(from time.Time, rate int64) {
		a, b := from, cutoff
		if a.Before(start) {
			a = start
		}
		if b.After(cutoff) {
			b = cutoff
		}
		if b.After(a) {
			secs := int64(b.Sub(a) / time.Second)
			totalSeconds += secs
			weighted += secs * rate
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
					secs := int64(b.Sub(a) / time.Second)
					totalSeconds += secs
					weighted += secs * o.rate
				}
				delete(open, ed.ServiceID)
			}
		}
	}
	for _, o := range open { // still-running spans accrue to the cutoff
		credit(o.at, o.rate)
	}
	return e.q.UpsertQuotaUsage(ctx, store.UpsertQuotaUsageParams{
		OrgID: orgID, Meter: "service_span_seconds", Period: period,
		Used: totalSeconds, RateCents: weighted,
	})
}

// Usage returns the stored rollup rows for an org+period (billing/report reads).
func (e *Emitter) Usage(ctx context.Context, orgID, period string) ([]store.QuotaUsage, error) {
	return e.q.GetQuotaUsage(ctx, store.GetQuotaUsageParams{OrgID: orgID, Period: period})
}
