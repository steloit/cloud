// Package quota is the M7 quota evaluator (T11.5): one decision function for
// every metered limit, so soft/hard behaviour can't diverge across call sites.
// Soft quotas keep working and BILL — an operation that would incur overage
// proceeds only with confirm=true and the price shown up front (never
// discovered on the invoice). Hard quotas fail loudly with a way forward
// (402/429 + remediation). Safety features are never gated (that check belongs
// to the caller via billing.IsNeverGated — this package never sees a safety
// capability). Money is integer cents (ADR-025).
package quota

// Kind distinguishes the two quota postures (billing pack).
type Kind int

const (
	Soft Kind = iota // egress, seats, builds, events, AI — keep working + bill
	Hard             // fail loudly (previews queue, the spend cap)
)

// Decision is the single outcome the caller maps to the problem catalog.
// Exactly one of the blocked flags is set when Allowed is false.
type Decision struct {
	Allowed           bool  // proceed
	OverageConfirmed  bool  // soft overage being billed (confirm=true)
	SoftBlocked       bool  // soft over + no confirm → 402 quota_exceeded (soft)
	HardBlocked       bool  // hard over → 402/429 quota_exceeded (hard)
	OveragePriceCents int64 // the INCREMENTAL overage this op incurs (soft 402)
}

// Evaluate decides one metered operation. `allowance` is the plan's included
// amount (< 0 = unlimited); `used` is the current period usage; `delta` is the
// units this operation adds; `overageRateCents` is the per-unit soft price.
//
//	within allowance          → Allowed
//	soft over + confirm=false  → SoftBlocked, price = incremental overage units × rate
//	soft over + confirm=true   → Allowed + OverageConfirmed (billed)
//	hard over                  → HardBlocked
func Evaluate(kind Kind, allowance, used, delta, overageRateCents int64, confirm bool) Decision {
	if allowance < 0 || used+delta <= allowance {
		return Decision{Allowed: true}
	}
	if kind == Hard {
		return Decision{HardBlocked: true}
	}
	if confirm {
		return Decision{Allowed: true, OverageConfirmed: true}
	}
	// Only the newly-crossed units are charged now (existing overage was already
	// consented to). newOver − existingOver, floored at 0.
	newOver := (used + delta) - allowance
	existingOver := used - allowance
	if existingOver < 0 {
		existingOver = 0
	}
	incremental := newOver - existingOver
	if incremental < 0 {
		incremental = 0
	}
	return Decision{SoftBlocked: true, OveragePriceCents: incremental * overageRateCents}
}

// WarnLevel is the fraction of the allowance consumed (0..1+; -1 for unlimited)
// — the 80% banner+bell+email fires at >= 0.8 (billing pack, QA scenario 2).
func WarnLevel(allowance, used int64) float64 {
	if allowance < 0 {
		return -1 // unlimited: never warns
	}
	if allowance == 0 {
		if used > 0 {
			return 1
		}
		return 0
	}
	return float64(used) / float64(allowance)
}

// ShouldWarn reports whether the 80% threshold has been crossed.
func ShouldWarn(allowance, used int64) bool {
	lvl := WarnLevel(allowance, used)
	return lvl >= 0.8
}
