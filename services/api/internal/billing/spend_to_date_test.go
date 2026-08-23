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
	} {
		got := SpendToDate(tc.fee, tc.rows...)
		if got < 0 {
			t.Errorf("%s: SpendToDate = %d — a NEGATIVE month-to-date spend reads as far below "+
				"the cap, so the cap stops refusing anything", tc.name, got)
		}
		if got < tc.fee {
			t.Errorf("%s: SpendToDate = %d, below the plan fee %d alone", tc.name, got, tc.fee)
		}
	}
	// And it is exact for values that fit — saturation must not cost precision
	// on every ordinary invoice.
	if got := SpendToDate(9900, 20800, 162); got != 9900+20800+162 {
		t.Fatalf("ordinary arithmetic changed: %d", got)
	}
}
