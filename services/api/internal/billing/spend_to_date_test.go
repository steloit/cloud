package billing

import (
	"math"
	"testing"
)

// THE HARD CAP'S OWN ARITHMETIC MUST NOT WRAP (O19).
//
// SpendToDate is the number the cap enforces against, and it summed
// quota_usage.rate_cents with a raw `+=`. A wrapped total is NEGATIVE, which
// reads as "far below the cap" and disables the cap at exactly the moment it
// matters. Measured before the fix: SpendToDate(2900, MaxInt64/2, MaxInt64/2)
// = -9223372036854772910.
func TestSpendToDateSaturatesInsteadOfWrapping(t *testing.T) {
	half := int64(math.MaxInt64 / 2)
	for _, tc := range []struct {
		name string
		fee  int64
		rows []int64
	}{
		{"two half-max meters", 2900, []int64{half, half}},
		{"fee plus max", 2900, []int64{math.MaxInt64}},
		{"three large meters", 0, []int64{half, half, half}},
		{"a negative row (a wrap upstream)", 2900, []int64{-1}},
		// A NEGATIVE PLAN FEE. Not reachable from either caller today — both read
		// the fee from the pricing table — but the arm exists, and the loop's
		// `c < 0` check is only redundant BECAUSE this one guarantees total >= 0
		// going in. Leaving it untested made the redundancy load-bearing and
		// invisible: deleting this arm was the one mutation that survived.
		{"a negative plan fee", -1, []int64{100}},
		{"a negative plan fee alone", -1, nil},
	} {
		got := SpendToDate(tc.fee, tc.rows...)
		// == MaxInt64, not merely >= 0. A PARTIAL SUM satisfies "non-negative and
		// at least the fee" while UNDER-reporting spend, and that mutation
		// survived the weaker assertion — measured. Saturation has to be
		// asserted as saturation.
		if got != math.MaxInt64 {
			t.Errorf("%s: SpendToDate = %d, want MaxInt64. Anything else here is a partial sum, "+
				"which under-reports the month-to-date figure instead of pegging it.", tc.name, got)
		}
	}

	// Exact at the boundary, in both directions — so "saturating" cannot be
	// implemented as "peg anything large".
	for _, tc := range []struct {
		name string
		fee  int64
		rows []int64
		want int64
	}{
		{"exactly MaxInt64", 1, []int64{math.MaxInt64 - 1}, math.MaxInt64},
		{"one below", 1, []int64{math.MaxInt64 - 2}, math.MaxInt64 - 1},
		{"an ordinary invoice", 9900, []int64{20800, 162}, 9900 + 20800 + 162},
		{"no rows at all", 2900, nil, 2900},
	} {
		if got := SpendToDate(tc.fee, tc.rows...); got != tc.want {
			t.Errorf("%s: SpendToDate = %d, want %d", tc.name, got, tc.want)
		}
	}

	// MONOTONIC: appending a non-negative row can never lower the answer. This is
	// the property a partial-sum implementation breaks.
	base := SpendToDate(2900, 1000, 2000)
	for _, extra := range []int64{0, 1, 5000, math.MaxInt64} {
		if got := SpendToDate(2900, 1000, 2000, extra); got < base {
			t.Errorf("adding a row of %d LOWERED the total from %d to %d", extra, base, got)
		}
	}
}
