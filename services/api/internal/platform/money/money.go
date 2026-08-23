// Package money is the platform's monetary type. ADR-025 says money is integer
// cents end-to-end; this package is what makes that true rather than intended.
//
// WHY A STRUCT AND NOT `type Cents int64`.
//
// A named integer type still admits `a + b` and `a * b`, so every arithmetic
// site remains an opportunity to overflow silently. A struct does not: Go will
// not compile `+` or `*` on it, so the only way to combine two amounts is
// through the checked methods below. That converts a class of defect from
// "caught in review, sometimes" to "does not compile".
//
// This is not a hypothetical. US-3.8 shipped an overflow fix into ONE arm of a
// three-arm pricing switch; the sibling arms kept wrapping, and
// `postgres {storage_gb: 1e18}` priced at -5340232221128652948 — which then
// poisoned the org's committed run-rate and disabled its hard spend cap for
// every later create. The aggregate in PriceAll wrapped separately, and so did
// the spend cap's own projection. Three independent sites, one class, found one
// at a time across three review rounds. Nothing about the fix was hard; finding
// each site was. The type removes the search.
//
// INVARIANT: a Cents value is ALWAYS in [0, MaxMonthly]. There is no
// constructor, operation, or decoder that can produce one outside that range —
// they return an error instead. Callers therefore never range-check a Cents;
// holding one IS the proof.
//
// Stdlib only (architecture is frozen: Go/stdlib-http/sqlc/River). Overflow
// detection uses math/bits, which is exact rather than heuristic.
package money

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"
)

// secondsInLongestMonth is what a monthly rate can be multiplied by downstream.
//
// UNEXPORTED, deliberately. An earlier revision exported it so a test could pin
// MaxMonthly's derivation — a false reason, since money_test.go is `package
// money` and reads it either way. What the export actually bought was one line
// in another package doing `price * money.SecondsInLongestMonth`: a raw int64
// multiply, in the codebase whose whole point is that those do not compile.
// Callers that need the question answered get SurvivesBillingMonth below.
const secondsInLongestMonth = int64(31 * 24 * 60 * 60)

// MaxMonthly is the largest monthly amount this platform can carry through its
// WHOLE money path without wrapping — not merely the largest one multiply
// survives.
//
// It is DERIVED, not chosen. `metering.Rollup` computes `weighted += secs *
// rate`, so an amount the estimate accepts is multiplied by a month of seconds
// on the billing side. An earlier bound guaranteed only that the estimate's own
// arithmetic fit in an int64; amounts just under it then wrapped in the rollup —
// the same defect one layer later, on the invoice instead of the quote.
//
// This is emphatically NOT a commercial ceiling. Choosing "at most N instances"
// or "at most $X/month" is a pricing decision, and the founder ruling of
// 2026-07-27 is that implementation code never makes those. This is the point
// past which the platform's integers stop being able to represent the answer,
// which is arithmetic. Anything below it that is merely unaffordable is the hard
// spend cap's business.
const MaxMonthly = int64(math.MaxInt64) / secondsInLongestMonth

// ErrOverflow is returned by any operation whose result would leave the
// representable range. It is deliberately one error for both directions: a
// caller's correct response to "too large" and to "wrapped negative" is
// identical, and distinguishing them invites handling only one.
var ErrOverflow = errors.New("money: amount is outside the representable range")

// ErrNegative is returned when a negative amount is constructed. Every amount in
// this system is a price, a rate or a limit; none of them is ever negative, and
// a negative one is always the symptom of a wrap somewhere upstream.
var ErrNegative = errors.New("money: amount is negative")

// Cents is a non-negative monetary amount in integer cents.
//
// The zero value is a valid, usable zero. The field is unexported so no value
// can be built that skips the range check — `money.Cents{v: -1}` does not
// compile outside this package.
type Cents struct{ v int64 }

// Zero is the additive identity, provided so callers never need a literal.
var Zero = Cents{}

// FromInt converts a raw integer, which is the ONLY way an untrusted number
// becomes a Cents. Every decoder, DB scan and API input funnels through here.
func FromInt(n int64) (Cents, error) {
	if n < 0 {
		return Zero, fmt.Errorf("%w: %d", ErrNegative, n)
	}
	if n > MaxMonthly {
		return Zero, fmt.Errorf("%w: %d exceeds the maximum representable monthly amount %d", ErrOverflow, n, MaxMonthly)
	}
	return Cents{v: n}, nil
}

// MustFromInt is FromInt for compile-time-known constants from the pricing
// table. It panics, deliberately: a pricing table that cannot be represented is
// a deployment that must not start, not a request that fails.
func MustFromInt(n int64) Cents {
	c, err := FromInt(n)
	if err != nil {
		panic("money: invalid constant amount: " + err.Error())
	}
	return c
}

// Int64 returns the raw cents, for the wire and the database. This is the ONLY
// exit from the type, so every place a raw number re-enters the world is
// greppable.
func (c Cents) Int64() int64 { return c.v }

// Add returns c+d, or ErrOverflow.
func (c Cents) Add(d Cents) (Cents, error) {
	sum, carry := bits.Add64(uint64(c.v), uint64(d.v), 0)
	if carry != 0 || sum > uint64(MaxMonthly) {
		return Zero, fmt.Errorf("%w: %d + %d", ErrOverflow, c.v, d.v)
	}
	return Cents{v: int64(sum)}, nil
}

// MulInt returns c*n for a non-negative count, or ErrOverflow.
//
// bits.Mul64 gives the full 128-bit product, so the check is exact rather than
// the usual "divide it back and compare", which is correct but easy to write
// subtly wrong — and being easy to write wrong is what this package exists to
// stop.
func (c Cents) MulInt(n int64) (Cents, error) {
	if n < 0 {
		return Zero, fmt.Errorf("%w: multiplier %d", ErrNegative, n)
	}
	hi, lo := bits.Mul64(uint64(c.v), uint64(n))
	if hi != 0 || lo > uint64(MaxMonthly) {
		return Zero, fmt.Errorf("%w: %d × %d", ErrOverflow, c.v, n)
	}
	return Cents{v: int64(lo)}, nil
}

// Sub returns c-d, or ErrNegative if d exceeds c.
//
// There is no negative Cents, and that is deliberate: every amount here is a
// price, a rate or a limit. A subtraction that would go negative is always
// either a bug or a case the caller must handle explicitly — so it is an error,
// not a value. Callers that want "the increase, if any" compare first.
func (c Cents) Sub(d Cents) (Cents, error) {
	if d.v > c.v {
		return Zero, fmt.Errorf("%w: %d - %d", ErrNegative, c.v, d.v)
	}
	return Cents{v: c.v - d.v}, nil
}

// AddMul returns c + (unit × n) in one checked step — the shape every priced
// dimension actually needs (a base plus a metered quantity), provided so no
// caller writes the two-step version and checks only one of them.
func (c Cents) AddMul(unit Cents, n int64) (Cents, error) {
	product, err := unit.MulInt(n)
	if err != nil {
		return Zero, err
	}
	return c.Add(product)
}

// Sum adds any number of amounts, failing on the first overflow.
//
// Aggregates are where the second wrap lived: bounding each line while leaving
// the total unchecked moved the defect up a level rather than removing it. Two
// individually-legal estimate lines produced a negative total.
func Sum(amounts ...Cents) (Cents, error) {
	total := Zero
	for i, a := range amounts {
		next, err := total.Add(a)
		if err != nil {
			return Zero, fmt.Errorf("summing amount %d of %d: %w", i+1, len(amounts), err)
		}
		total = next
	}
	return total, nil
}

// SurvivesBillingMonth reports whether this amount, treated as a monthly rate,
// can be multiplied by the longest billing month without leaving int64 — the
// question `metering.Rollup` implicitly asks of every rate it accumulates.
//
// It exists so no caller needs the raw constant to ask it. By the type's
// invariant this is true for every Cents, so a false here means the invariant
// is broken, not that the caller supplied something too big.
func (c Cents) SurvivesBillingMonth() bool {
	hi, lo := bits.Mul64(uint64(c.v), uint64(secondsInLongestMonth))
	return hi == 0 && lo <= uint64(math.MaxInt64)
}

// Cmp orders two amounts: -1, 0 or +1.
func (c Cents) Cmp(d Cents) int {
	switch {
	case c.v < d.v:
		return -1
	case c.v > d.v:
		return 1
	default:
		return 0
	}
}

// GreaterThan reports c > d, so callers read as prose at the cap check.
func (c Cents) GreaterThan(d Cents) bool { return c.v > d.v }

// IsZero reports whether the amount is nothing.
func (c Cents) IsZero() bool { return c.v == 0 }

// String renders $D.CC, the form the spend cap shows its arithmetic in.
func (c Cents) String() string {
	return fmt.Sprintf("$%d.%02d", c.v/100, c.v%100)
}

// MarshalJSON emits a bare integer: the wire contract is integer cents
// (ADR-025), and the type must not change what the API looks like.
func (c Cents) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(c.v, 10)), nil
}

// UnmarshalJSON is the decoding boundary, and it validates rather than
// truncating: a number outside the range is an error, never a silently clamped
// or wrapped value.
//
// json.Number, not int64, so a fractional or oversized literal is rejected here
// instead of being rounded by encoding/json into something that looks fine.
func (c *Cents) UnmarshalJSON(b []byte) error {
	// json.Number is a string type, so `"5800"` unmarshals into it happily. The
	// wire contract is a NUMBER; a quoted one is a different type that happens to
	// look right, and accepting it here would make the API quietly bivalent.
	if len(b) > 0 && b[0] == '"' {
		return fmt.Errorf("money: %s is a string, not a number — the wire form is a bare integer of cents", b)
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("money: not a number: %w", err)
	}
	i, err := n.Int64()
	if err != nil {
		return fmt.Errorf("money: %s is not a whole number of cents: %w", n.String(), err)
	}
	v, err := FromInt(i)
	if err != nil {
		return err
	}
	*c = v
	return nil
}

// Accrual is Σ(rate × seconds) — a metered accumulation, NOT an amount.
//
// WHY THIS IS NOT A Cents. A Cents is bounded by MaxMonthly, which is derived so
// that ONE monthly rate survives ONE month of seconds. An accrual is the product
// itself: a single service at the maximum rate for the longest month is ~9.2e18,
// which consumes the entire int64 budget on its own. Two such services are a
// legitimate business fact and an impossible int64. Bounding an accrual by
// MaxMonthly would therefore refuse arithmetic that is simply correct, and
// bounding it by MaxInt64 is the wrap this type exists to remove.
//
// So it carries 128 bits and CANNOT overflow in practice rather than being
// checked for overflow: to exceed 2^128 you would need ~3.7e19 service-months at
// the maximum representable rate. The bug class is unrepresentable instead of
// tested-against — which is the point ADR-0014 makes about invariants living in
// types.
//
// `metering.Rollup` accumulated this as a raw `weighted += secs * rate` across
// every span of every service in the org. MaxMonthly makes one service-month
// exactly fit; the SECOND service wraps `quota_usage.rate_cents`, the number
// billing derives charges from. Nothing detected it — a wrapped value is just a
// number.
type Accrual struct{ hi, lo uint64 }

// NoAccrual is the empty accumulation, provided so callers never need a literal.
var NoAccrual = Accrual{}

// AddMul accumulates rate × seconds. Seconds is a duration, not a count of
// money, so it is an ordinary int64 — but it must not be negative: a negative
// span is a clock or ordering bug upstream, and silently crediting it would
// REDUCE a bill.
func (a Accrual) AddMul(rate Cents, seconds int64) (Accrual, error) {
	if seconds < 0 {
		return NoAccrual, fmt.Errorf("%w: %d seconds", ErrNegative, seconds)
	}
	hi, lo := bits.Mul64(uint64(rate.v), uint64(seconds))
	sum, carry := bits.Add64(a.lo, lo, 0)
	high, carry := bits.Add64(a.hi, hi, carry)
	if carry != 0 {
		// Unreachable with any real fleet; returned rather than ignored because
		// "cannot happen" is how the int64 version was written too.
		return NoAccrual, fmt.Errorf("%w: accrual exceeds 128 bits", ErrOverflow)
	}
	return Accrual{hi: high, lo: sum}, nil
}

// Int64 narrows the accrual to an int64 for storage, or ErrOverflow.
//
// This is the ONLY place the 128-bit value can be lost, so it is the only place
// that has to fail — and it fails LOUDLY instead of wrapping. A caller that gets
// an error here has an org whose accrual genuinely cannot be represented in the
// column, which is a billing incident, not a rounding question.
func (a Accrual) Int64() (int64, error) {
	if a.hi != 0 || a.lo > uint64(math.MaxInt64) {
		return 0, fmt.Errorf("%w: accrual %s does not fit an int64", ErrOverflow, a)
	}
	return int64(a.lo), nil
}

// DivSeconds returns the accrual spread over n seconds, as an amount — the
// arithmetic that turns Σ(monthly-rate × seconds) back into money.
//
// Provided here rather than at the call site because the 128-by-64 division has
// a trap: bits.Div64 PANICS when the high word is at least the divisor. That is
// exactly the case where the quotient would not fit 64 bits, so the guard below
// is not defensive padding — without it, an org large enough to need this method
// crashes the process instead of returning an error.
func (a Accrual) DivSeconds(n int64) (Cents, error) {
	if n <= 0 {
		return Zero, fmt.Errorf("money: cannot spread an accrual over %d seconds", n)
	}
	d := uint64(n)
	if a.hi >= d {
		return Zero, fmt.Errorf("%w: %s spread over %d seconds exceeds 64 bits", ErrOverflow, a, n)
	}
	q, _ := bits.Div64(a.hi, a.lo, d)
	if q > uint64(MaxMonthly) {
		return Zero, fmt.Errorf("%w: %s spread over %d seconds is %d, above the maximum representable monthly amount %d",
			ErrOverflow, a, n, q, MaxMonthly)
	}
	return Cents{v: int64(q)}, nil
}

// IsZero reports whether nothing has accrued.
func (a Accrual) IsZero() bool { return a.hi == 0 && a.lo == 0 }

// String renders the full 128-bit value in decimal, so an overflow message names
// the actual number rather than a truncated one.
func (a Accrual) String() string {
	if a.hi == 0 {
		return strconv.FormatUint(a.lo, 10)
	}
	// Long division by 1e19 (the largest power of ten below 2^64) — two limbs is
	// at most three chunks, and this avoids math/big for a stdlib-only package.
	const chunk = uint64(1e19)
	hi, lo := a.hi, a.lo
	var parts []string
	for hi != 0 || lo != 0 {
		var rem uint64
		var qh uint64
		if hi >= chunk {
			qh = hi / chunk
			hi = hi % chunk
		}
		q, r := bits.Div64(hi, lo, chunk)
		rem = r
		hi, lo = qh, q
		if hi == 0 && lo == 0 {
			parts = append(parts, strconv.FormatUint(rem, 10))
		} else {
			parts = append(parts, fmt.Sprintf("%019d", rem))
		}
	}
	out := ""
	for i := len(parts) - 1; i >= 0; i-- {
		out += parts[i]
	}
	return out
}
