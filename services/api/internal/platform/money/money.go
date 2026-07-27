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
