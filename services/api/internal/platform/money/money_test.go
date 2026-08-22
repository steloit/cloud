package money

import (
	"encoding/json"
	"errors"
	"math"
	"math/bits"
	"testing"
	"time"
)

// The type's whole promise: a Cents in hand is a Cents in range. Every route in
// is tested, because a single unchecked constructor would make every downstream
// range assumption false.
func TestNoRouteInCanProduceAnOutOfRangeAmount(t *testing.T) {
	for _, n := range []int64{-1, -5340232221128652948, math.MinInt64, MaxMonthly + 1, math.MaxInt64} {
		if c, err := FromInt(n); err == nil {
			t.Fatalf("FromInt(%d) produced %d; every later range assumption rests on this being impossible", n, c.Int64())
		}
	}
	for _, n := range []int64{0, 1, 1900, MaxMonthly} {
		c, err := FromInt(n)
		if err != nil {
			t.Fatalf("FromInt(%d) refused a legal amount: %v", n, err)
		}
		if c.Int64() != n {
			t.Fatalf("FromInt(%d).Int64() = %d", n, c.Int64())
		}
	}
}

// The ceiling is INCLUSIVE, and it is derived from what a full billing month can
// carry rather than from what one multiply survives. Both halves matter: an
// exclusive bound silently refuses the largest legal amount, and a bound derived
// from one multiply lets the invoice wrap where the estimate did not.
func TestTheCeilingIsInclusiveAndSurvivesAFullBillingMonth(t *testing.T) {
	if _, err := FromInt(MaxMonthly); err != nil {
		t.Fatalf("the maximum amount was refused: %v", err)
	}
	if _, err := FromInt(MaxMonthly + 1); err == nil {
		t.Fatal("one cent past the maximum was accepted")
	}
	// Runtime values, not constants: Go rejects a constant expression that
	// overflows at COMPILE time, so a constant version of this makes the
	// regression unbuildable rather than red — which reads as a broken test.
	secs, max := secondsInLongestMonth, MaxMonthly
	weighted := max * secs
	if weighted <= 0 || weighted/secs != max {
		t.Fatalf("the maximum amount wraps when weighted across a month: %d × %d = %d — metering.Rollup performs exactly this multiplication",
			max, secs, weighted)
	}
}

func TestAddIsChecked(t *testing.T) {
	half := MustFromInt(MaxMonthly / 2)
	if _, err := half.Add(half); err != nil {
		t.Fatalf("two halves must fit: %v", err)
	}
	max := MustFromInt(MaxMonthly)
	if _, err := max.Add(MustFromInt(1)); !errors.Is(err, ErrOverflow) {
		t.Fatalf("max+1 must overflow, got %v", err)
	}
	if _, err := max.Add(max); !errors.Is(err, ErrOverflow) {
		t.Fatalf("max+max must overflow, got %v", err)
	}
	sum, err := MustFromInt(1900).Add(MustFromInt(100))
	if err != nil || sum.Int64() != 2000 {
		t.Fatalf("1900+100 = %v, %v", sum.Int64(), err)
	}
}

func TestMulIntIsChecked(t *testing.T) {
	unit := MustFromInt(1200)
	// The exact boundary: the largest count whose product still fits.
	max := MaxMonthly / 1200
	if _, err := unit.MulInt(max); err != nil {
		t.Fatalf("the largest representable count (%d) was refused: %v", max, err)
	}
	if _, err := unit.MulInt(max + 1); !errors.Is(err, ErrOverflow) {
		t.Fatalf("one past the largest count must overflow, got %v", err)
	}
	// The value that started all of this.
	if _, err := unit.MulInt(math.MaxInt64); !errors.Is(err, ErrOverflow) {
		t.Fatalf("MaxInt64 instances must overflow, got %v", err)
	}
	// A 64×64 product whose LOW half alone looks harmless — the case a naive
	// `if lo > max` check without inspecting `hi` waves through.
	//
	// The multiplier is DERIVED so the product genuinely exceeds 2^64. An earlier
	// version used 2 × (MaxInt64/2 + 2) = 2^63 + 2, which is BELOW 2^64: hi is
	// zero there, so it died on the `lo` arm and proved nothing about the `hi`
	// arm its own comment named. Deleting `hi != 0` survived the entire suite,
	// and `postgres {storage_gb: 368934881474191033}` then priced at 1934 cents.
	// The example has to meet the standard of the rule it teaches.
	const unitCents = 3
	overflowing := int64(math.MaxUint64/unitCents) + 1 // unitCents × this >= 2^64
	hi, lo := bits.Mul64(unitCents, uint64(overflowing))
	if hi == 0 {
		t.Fatalf("this case no longer exercises the hi arm: %d × %d has hi=0, lo=%d", unitCents, overflowing, lo)
	}
	if _, err := MustFromInt(unitCents).MulInt(overflowing); !errors.Is(err, ErrOverflow) {
		t.Fatalf("a product that wraps into a small positive must overflow, got %v", err)
	}
	if _, err := unit.MulInt(-1); !errors.Is(err, ErrNegative) {
		t.Fatalf("a negative multiplier must be refused, got %v", err)
	}
	// Zero on either side is legal and is zero.
	if got, err := unit.MulInt(0); err != nil || !got.IsZero() {
		t.Fatalf("unit×0 = %v, %v", got.Int64(), err)
	}
	if got, err := Zero.MulInt(math.MaxInt64); err != nil || !got.IsZero() {
		t.Fatalf("0×MaxInt64 must be 0, got %v, %v", got.Int64(), err)
	}
}

// AddMul exists so no caller writes the two-step version and checks one half.
func TestAddMulMatchesTheTwoStepFormAndFailsWhenEitherHalfWould(t *testing.T) {
	base, unit := MustFromInt(4200), MustFromInt(1200)
	got, err := base.AddMul(unit, 3)
	if err != nil || got.Int64() != 4200+3*1200 {
		t.Fatalf("AddMul = %v, %v", got.Int64(), err)
	}
	// The multiply overflows.
	if _, err := base.AddMul(unit, math.MaxInt64); !errors.Is(err, ErrOverflow) {
		t.Fatalf("want overflow from the product, got %v", err)
	}
	// The multiply fits but the ADD does not — the half a one-sided check misses.
	nearMax := MustFromInt(MaxMonthly - 10)
	if _, err := nearMax.AddMul(MustFromInt(100), 1); !errors.Is(err, ErrOverflow) {
		t.Fatalf("want overflow from the addition, got %v", err)
	}
}

// Aggregates are where the second wrap lived: bounding each line but not the
// total moved the defect up a level instead of removing it.
func TestSumIsCheckedAndReportsWhichAmountBrokeIt(t *testing.T) {
	got, err := Sum(MustFromInt(100), MustFromInt(200), MustFromInt(300))
	if err != nil || got.Int64() != 600 {
		t.Fatalf("Sum = %v, %v", got.Int64(), err)
	}
	if got, err := Sum(); err != nil || !got.IsZero() {
		t.Fatalf("the empty sum must be zero, got %v, %v", got.Int64(), err)
	}
	max := MustFromInt(MaxMonthly)
	if _, err := Sum(max, max); !errors.Is(err, ErrOverflow) {
		t.Fatalf("two maximums must overflow, got %v", err)
	}
	// Individually legal, collectively not — the exact shape that produced a
	// negative estimate total from two valid lines.
	half := MustFromInt(MaxMonthly/2 + 1)
	if _, err := Sum(half, half); !errors.Is(err, ErrOverflow) {
		t.Fatalf("two legal halves that do not fit must overflow, got %v", err)
	}
}

// The wire contract is a bare integer (ADR-025) and the type must not change
// what the API looks like — otherwise adopting it is a breaking change.
func TestJSONIsABareIntegerInBothDirections(t *testing.T) {
	b, err := json.Marshal(MustFromInt(5800))
	if err != nil || string(b) != "5800" {
		t.Fatalf("marshalled to %q, %v — the wire form is a bare integer", b, err)
	}
	var c Cents
	if err := json.Unmarshal([]byte("5800"), &c); err != nil || c.Int64() != 5800 {
		t.Fatalf("unmarshal = %v, %v", c.Int64(), err)
	}
	// Decoding VALIDATES rather than truncating or wrapping.
	for _, bad := range []string{"-1", "58.5", "9223372036854775807", "1e30", `"5800"`} {
		var x Cents
		if err := json.Unmarshal([]byte(bad), &x); err == nil {
			t.Fatalf("unmarshal(%s) produced %d; the decoder is a construction site like any other", bad, x.Int64())
		}
	}
	// Round-trip through a struct field, which is how it actually travels.
	type line struct {
		MonthlyCents Cents `json:"monthly_cents"`
	}
	var l line
	if err := json.Unmarshal([]byte(`{"monthly_cents":1900}`), &l); err != nil || l.MonthlyCents.Int64() != 1900 {
		t.Fatalf("struct round-trip: %v, %v", l.MonthlyCents.Int64(), err)
	}
	out, _ := json.Marshal(l)
	if string(out) != `{"monthly_cents":1900}` {
		t.Fatalf("struct marshalled to %s", out)
	}
}

func TestStringRendersDollarsForTheCapsArithmetic(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{{0, "$0.00"}, {5, "$0.05"}, {1900, "$19.00"}, {123456, "$1234.56"}} {
		if got := MustFromInt(tc.in).String(); got != tc.want {
			t.Fatalf("%d rendered %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestComparisons(t *testing.T) {
	a, b := MustFromInt(100), MustFromInt(200)
	if a.Cmp(b) != -1 || b.Cmp(a) != 1 || a.Cmp(a) != 0 {
		t.Fatal("Cmp is not a total order")
	}
	if !b.GreaterThan(a) || a.GreaterThan(b) || a.GreaterThan(a) {
		t.Fatal("GreaterThan is not strict")
	}
	if !Zero.IsZero() || a.IsZero() {
		t.Fatal("IsZero")
	}
}

// There is no negative Cents, so subtraction that would go negative is an error
// rather than a value. The alternative — clamping to zero — would silently turn
// "we owe them" into "they owe nothing", which is the failure mode this whole
// type exists to prevent.
func TestSubRefusesToGoNegative(t *testing.T) {
	got, err := MustFromInt(5000).Sub(MustFromInt(1900))
	if err != nil || got.Int64() != 3100 {
		t.Fatalf("5000-1900 = %v, %v", got.Int64(), err)
	}
	if got, err := MustFromInt(1900).Sub(MustFromInt(1900)); err != nil || !got.IsZero() {
		t.Fatalf("equal amounts must subtract to zero: %v, %v", got.Int64(), err)
	}
	if _, err := MustFromInt(100).Sub(MustFromInt(101)); !errors.Is(err, ErrNegative) {
		t.Fatalf("want ErrNegative, got %v", err)
	}
}

// The billing-month constant is pinned against the arithmetic that actually
// multiplies it — not against itself.
//
// Ported from estimates (f970477) along with the constant it guards: it was
// written when `secondsInLongestMonth` lived in `engine.go`, and it belongs
// wherever that constant lives, because it is the ONLY check that the constant
// is not merely self-consistent.
//
// `secondsInLongestMonth` had one representation in the pricing engine and
// another in `metering.Rollup`, which multiplies a rate by the REAL elapsed
// seconds of a period. Changing the constant from 31 days to 30 survived every
// package, because the ceiling and the test that checked it moved together. The
// consequence was not academic: a 30-day constant admits a rate of
// 3,558,399,704,200 cents/month, and multiplying that by a real 31-day period
// gives 9.53e18, which wraps to -8,915,926,305,980,271,616 in
// `weighted += secs * rate` — persisted as `quota_usage.rate_cents`, the number
// billing derives charges from.
//
// So this derives the longest real period from the same `AddDate(0, 1, 0)`
// arithmetic `metering.periodBounds` uses, across a leap year and a non-leap
// year, and asserts the constant covers it. Two representations, one invariant,
// and the test depends on the one the constant does not control.
func TestTheBillingMonthConstantCoversTheLongestRealPeriod(t *testing.T) {
	var longest int64
	var when string
	for _, year := range []int{2024, 2026} { // leap and non-leap
		for month := 1; month <= 12; month++ {
			start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
			// The same expression periodBounds uses to find a period's end.
			secs := int64(start.AddDate(0, 1, 0).Sub(start).Seconds())
			if secs > longest {
				longest, when = secs, start.Format("2006-01")
			}
		}
	}
	if secondsInLongestMonth < longest {
		t.Fatalf("secondsInLongestMonth is %d but %s is %d seconds — the ceiling would admit a rate whose real-month product wraps in metering.Rollup's `weighted += secs * rate`",
			secondsInLongestMonth, when, longest)
	}
	// And the ceiling it produces genuinely survives that period.
	if got := MaxMonthly * longest; got <= 0 || got/longest != MaxMonthly {
		t.Fatalf("the maximum accepted rate wraps across %s: %d × %d = %d", when, MaxMonthly, longest, got)
	}
}

// SurvivesBillingMonth answers the question metering.Rollup implicitly asks of
// every rate it accumulates. It has NO production caller yet (O19 is the task
// that would add one); today only tests use it.
//
// That makes a negative case mandatory rather than nice-to-have. An earlier
// version of this test asserted only positives, so `return true` satisfied it —
// reproducing, in the test written to close the MulInt blocker, the exact defect
// that blocker WAS. The negative below uses `Cents{v: math.MaxInt64}`, a value no
// constructor can produce; it is reachable only because this test is
// `package money`, which is the point.
func TestSurvivesBillingMonthHoldsExactlyToTheCeiling(t *testing.T) {
	for _, n := range []int64{0, 1, 1900, MaxMonthly} {
		if !MustFromInt(n).SurvivesBillingMonth() {
			t.Fatalf("%d is representable but does not survive a billing month — the type invariant is broken", n)
		}
	}
	// THE discriminating case: an amount that cannot survive a billing month must
	// answer false. Without this, `func (c Cents) SurvivesBillingMonth() bool {
	// return true }` passes the whole suite.
	if (Cents{v: math.MaxInt64}).SurvivesBillingMonth() {
		t.Fatal("MaxInt64 cents claims to survive a billing month — the method is not actually checking")
	}
	if (Cents{v: MaxMonthly + 1}).SurvivesBillingMonth() {
		t.Fatal("one cent past the ceiling claims to survive a billing month — the method is not tight")
	}
	// MaxMonthly is the LAST value for which it holds. Checked against the raw
	// product rather than against the method, so this cannot pass by agreeing
	// with itself.
	if hi, lo := bits.Mul64(uint64(MaxMonthly), uint64(secondsInLongestMonth)); hi != 0 || lo > uint64(math.MaxInt64) {
		t.Fatalf("MaxMonthly × a month already leaves int64 (hi=%d lo=%d) — the ceiling is derived wrong", hi, lo)
	}
	if hi, lo := bits.Mul64(uint64(MaxMonthly+1), uint64(secondsInLongestMonth)); hi == 0 && lo <= uint64(math.MaxInt64) {
		t.Fatal("one cent past MaxMonthly still fits a billing month — the derivation is not tight")
	}
}
