package metering

// O19 · AC 2: the guard belongs on the side the constant does not control.
//
// money.MaxMonthly is derived as MaxInt64 / (31 days of seconds), and
// TestTheBillingMonthConstantCoversTheLongestRealPeriod (in package money)
// RE-IMPLEMENTS AddDate(0,1,0) to check it — because periodBounds is unexported
// and in this package. That catches the CONSTANT drifting and cannot catch
// metering's period WINDOW growing, which is the half that actually breaks the
// derivation. Nothing in this package referenced money at all; only prose tied
// them together.

import (
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/platform/money"
)

// TestEveryRealPeriodFitsWhatTheCeilingWasDerivedFrom drives the REAL
// periodBounds — not a re-implementation of it — across every month of several
// years, including leap Februaries and the DST-shifting months, and asserts that
// a rate at the ceiling multiplied by that period's seconds still fits.
func TestEveryRealPeriodFitsWhatTheCeilingWasDerivedFrom(t *testing.T) {
	ceiling := money.MustFromInt(money.MaxMonthly)

	checked := 0
	for year := 2024; year <= 2032; year++ { // 2024, 2028, 2032 are leap years
		for month := 1; month <= 12; month++ {
			period := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
			start, end, err := periodBounds(period)
			if err != nil {
				t.Fatalf("periodBounds(%q): %v", period, err)
			}
			secs := int64(end.Sub(start) / time.Second)
			if secs <= 0 {
				t.Fatalf("period %s has %d seconds", period, secs)
			}

			// The exact question Rollup asks of every rate it accumulates: one
			// service at the ceiling, running this whole period.
			if _, err := money.NoAccrual.AddMul(ceiling, secs); err != nil {
				t.Fatalf("period %s (%d seconds): a single service-month at the ceiling does not "+
					"accumulate: %v", period, secs, err)
			}
			acc, _ := money.NoAccrual.AddMul(ceiling, secs)
			if _, err := acc.Int64(); err != nil {
				t.Fatalf("period %s is %d seconds long, and MaxMonthly × that no longer fits an "+
					"int64: %v.\nmoney.MaxMonthly is derived from a 31-day month; if the period "+
					"window here grew, the derivation is stale and every rate the estimate accepts "+
					"can wrap on the billing side.", period, secs, err)
			}
			checked++
		}
	}
	if checked < 100 {
		t.Fatalf("only %d periods checked — this test would prove little", checked)
	}
}

// The window must not merely FIT the ceiling; the ceiling must be derived from
// the LONGEST window this function can actually produce. A 31-day month is that
// window, and this asserts periodBounds still produces exactly it — so shortening
// the derivation constant, or lengthening the period, is caught here rather than
// on an invoice.
func TestTheLongestPeriodThisFunctionProducesIsThirtyOneDays(t *testing.T) {
	var longest int64
	var at string
	for year := 2024; year <= 2032; year++ {
		for month := 1; month <= 12; month++ {
			period := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
			start, end, err := periodBounds(period)
			if err != nil {
				t.Fatal(err)
			}
			if secs := int64(end.Sub(start) / time.Second); secs > longest {
				longest, at = secs, period
			}
		}
	}
	const thirtyOneDays = int64(31 * 24 * 60 * 60)
	if longest != thirtyOneDays {
		t.Fatalf("the longest period periodBounds produces is %d seconds (%s), but money.MaxMonthly "+
			"is derived from %d. The two must move together.", longest, at, thirtyOneDays)
	}
	// And the ceiling is exactly the largest rate that survives it — one more
	// cent must not.
	if !money.MustFromInt(money.MaxMonthly).SurvivesBillingMonth() {
		t.Fatal("MaxMonthly does not survive a billing month — the derivation is broken")
	}
	if _, err := money.FromInt(money.MaxMonthly + 1); err == nil {
		t.Fatal("an amount above MaxMonthly was accepted, so the ceiling bounds nothing")
	}
}
