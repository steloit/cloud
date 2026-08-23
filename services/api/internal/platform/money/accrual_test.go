package money

// O19: the accrual accumulator. `metering.Rollup` computed Σ(seconds × monthly
// rate) as a raw int64 across every span of every service in an org, and
// MaxMonthly only guarantees that ONE service-month fits.

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

// twoServicesAtTheCeiling is the exact scenario the old accumulator wrapped on.
func TestTwoServicesAtTheCeilingForAMonthDoNotWrap(t *testing.T) {
	rate := MustFromInt(MaxMonthly)
	const monthSecs = int64(31 * 24 * 60 * 60)

	a := NoAccrual
	for i := 0; i < 2; i++ {
		var err error
		if a, err = a.AddMul(rate, monthSecs); err != nil {
			t.Fatalf("service %d: %v", i+1, err)
		}
	}

	// The raw int64 the previous implementation used, for contrast.
	var wrapped int64
	wrapped += monthSecs * MaxMonthly
	wrapped += monthSecs * MaxMonthly
	if wrapped >= 0 {
		t.Fatalf("the int64 accumulator did not wrap (%d) — this test's premise is gone", wrapped)
	}

	want := new(big.Int).Mul(big.NewInt(MaxMonthly), big.NewInt(monthSecs))
	want.Mul(want, big.NewInt(2))
	if a.String() != want.String() {
		t.Fatalf("accrual = %s, want %s (the int64 version gave %d)", a, want, wrapped)
	}

	// And narrowing it REFUSES rather than storing the wrapped number.
	if _, err := a.Int64(); !errors.Is(err, ErrOverflow) {
		t.Fatalf("narrowing an unrepresentable accrual returned %v, want ErrOverflow — a wrapped "+
			"value in quota_usage.rate_cents is what billing derives charges from", err)
	}
}

// The type must not refuse arithmetic that is merely large but correct: ONE
// service-month at the ceiling is exactly what MaxMonthly was derived to allow.
func TestOneServiceMonthAtTheCeilingStillFits(t *testing.T) {
	const monthSecs = int64(31 * 24 * 60 * 60)
	a, err := NoAccrual.AddMul(MustFromInt(MaxMonthly), monthSecs)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Int64()
	if err != nil {
		t.Fatalf("one service-month at the ceiling was refused: %v — MaxMonthly is derived to make "+
			"exactly this fit, so refusing it means the derivation and the type disagree", err)
	}
	if got != MaxMonthly*monthSecs {
		t.Fatalf("accrual = %d, want %d", got, MaxMonthly*monthSecs)
	}
}

// Differential against math/big over a wide range, because the 128-bit
// arithmetic and the decimal rendering are both easy to write subtly wrong —
// which is the failure mode this package exists to remove, not reproduce.
func TestTheAccrualAgreesWithArbitraryPrecision(t *testing.T) {
	rates := []int64{0, 1, 2, 999, 5800, MaxMonthly / 3, MaxMonthly - 1, MaxMonthly}
	secs := []int64{0, 1, 59, 3600, 86400, 28 * 86400, 31 * 86400}

	for _, r := range rates {
		for _, s := range secs {
			for _, n := range []int{1, 2, 7, 100} {
				a := NoAccrual
				want := new(big.Int)
				for i := 0; i < n; i++ {
					var err error
					if a, err = a.AddMul(MustFromInt(r), s); err != nil {
						t.Fatalf("rate=%d secs=%d n=%d: %v", r, s, n, err)
					}
					want.Add(want, new(big.Int).Mul(big.NewInt(r), big.NewInt(s)))
				}
				if a.String() != want.String() {
					t.Fatalf("rate=%d secs=%d ×%d: accrual %s, want %s", r, s, n, a, want)
				}
				// Int64 must agree with big exactly when big fits.
				got, err := a.Int64()
				fits := want.IsInt64()
				if fits != (err == nil) {
					t.Fatalf("rate=%d secs=%d ×%d: fits=%v but Int64 err=%v", r, s, n, fits, err)
				}
				if fits && got != want.Int64() {
					t.Fatalf("rate=%d secs=%d ×%d: Int64 %d, want %d", r, s, n, got, want.Int64())
				}
			}
		}
	}
}

// DivSeconds is where bits.Div64 PANICS if the guard is missing — so the guard
// is asserted directly, not merely relied upon.
func TestDivSecondsRefusesInsteadOfPanicking(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DivSeconds panicked instead of returning an error: %v — bits.Div64 panics "+
				"when the high word is at least the divisor, which is exactly the large-org case", r)
		}
	}()
	// The guard is `a.hi >= divisor`, so the case has to be CONSTRUCTED, not
	// merely made large. Two ceiling service-months is 2^64 + 157184, i.e. hi=1 —
	// which is below a divisor of 3600, so bits.Div64 does not panic and the
	// quotient check downstream catches it instead. An earlier version of this
	// test used exactly that and proved nothing: deleting the guard was GREEN.
	//
	// hi >= 1 with a divisor of 1 is the smallest true instance. TWO ceiling
	// service-months is not enough — it lands 111,616 BELOW 2^64, so hi is still
	// zero — which is precisely why the earlier fixture proved nothing. Accumulate
	// until the high word is actually in use, and fail if it never is.
	a := NoAccrual
	for i := 0; i < 8 && a.hi == 0; i++ {
		var err error
		if a, err = a.AddMul(MustFromInt(MaxMonthly), 31*86400); err != nil {
			t.Fatalf("accumulation %d: %v", i+1, err)
		}
	}
	if a.hi == 0 {
		t.Fatal("could not construct an accrual with a non-zero high word — the panic case " +
			"this test exists for is not being exercised")
	}
	if _, err := a.DivSeconds(1); !errors.Is(err, ErrOverflow) {
		t.Fatalf("DivSeconds(1) with hi=%d returned %v, want ErrOverflow", a.hi, err)
	}
	// ...and the downstream quotient check still catches the merely-too-large.
	if _, err := a.DivSeconds(3600); !errors.Is(err, ErrOverflow) {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}
	for _, n := range []int64{0, -1, math.MinInt64} {
		if _, err := a.DivSeconds(n); err == nil {
			t.Errorf("DivSeconds(%d) was accepted", n)
		}
	}
}

// The arithmetic the invoice needs: a rate accrued for a whole period, spread
// back over that period, is the rate again — exactly, with no drift.
func TestAFullPeriodAtOneRateSpreadsBackToThatRate(t *testing.T) {
	for _, days := range []int64{28, 29, 30, 31} {
		secs := days * 86400
		for _, rate := range []int64{1, 2900, 9900, 123457} {
			a, err := NoAccrual.AddMul(MustFromInt(rate), secs)
			if err != nil {
				t.Fatal(err)
			}
			got, err := a.DivSeconds(secs)
			if err != nil {
				t.Fatal(err)
			}
			if got.Int64() != rate {
				t.Errorf("%d-day month at %d¢: spread back to %d¢", days, rate, got.Int64())
			}
		}
	}
}

func TestAccrualRejectsNegativeSeconds(t *testing.T) {
	if _, err := NoAccrual.AddMul(MustFromInt(100), -1); !errors.Is(err, ErrNegative) {
		t.Fatalf("err = %v, want ErrNegative — a negative span would REDUCE a bill", err)
	}
}
