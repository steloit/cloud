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
	if delta < 0 {
		delta = 0 // a reduction never incurs overage — and never fakes being under
	}
	if allowance < 0 || used+delta <= allowance {
		return Decision{Allowed: true}
	}
	if kind == Hard {
		return Decision{HardBlocked: true}
	}
	// Only the NEWLY-crossed units are charged now (existing overage was already
	// consented to). newOver − existingOver, floored at 0.
	newOver := (used + delta) - allowance
	existingOver := used - allowance
	if existingOver < 0 {
		existingOver = 0
	}
	incremental := newOver - existingOver
	if incremental <= 0 {
		// no new overage (a no-op, or already over and not adding) — nothing to
		// confirm, so it proceeds; the existing overage bills on its own.
		return Decision{Allowed: true}
	}
	if confirm {
		return Decision{Allowed: true, OverageConfirmed: true}
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

// ClampOverageToCap is the soft-overage-halt-at-cap coupling (US-11.7 + US-11.2):
// a running service's soft overage STOPS ACCRUING once the org reaches its hard
// monthly spend cap. Given the org's spend so far this period, the org's hard
// cap (< 0 = uncapped), and a soft-overage amount about to accrue, it returns
// the portion that fits UNDER the cap — 0 once the cap is reached. The hard cap
// is a bound the soft meters cannot silently cross.
func ClampOverageToCap(spentCents, capCents, overageCents int64) int64 {
	if capCents < 0 || overageCents <= 0 {
		return maxInt64(overageCents, 0) // uncapped, or nothing to accrue
	}
	headroom := capCents - spentCents
	if headroom <= 0 {
		return 0 // at/over the cap: soft overage halts
	}
	if overageCents > headroom {
		return headroom // the last slice up to the cap accrues; the rest halts
	}
	return overageCents
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// QuotaWarning is the 80% notification payload (QA scenario 2): the meter, its
// usage vs allowance, the percent consumed, and the overage unit price so the
// math is shown up front ("keeps working and bills — warned at 80% with the
// math"), never discovered on the invoice.
type QuotaWarning struct {
	Meter          string
	Used           int64
	Allowance      int64
	PercentUsed    int   // 0..100+
	OverageRate    int64 // cents per unit beyond the allowance
	RemainingUnits int64 // allowance - used (0 if over)
}

// WarnMessage returns the 80% warning for a meter, and whether it should fire.
// It fires at >= 80% of a finite allowance (unlimited never warns). The payload
// carries the overage unit price — the "math" the banner/bell/email shows.
func WarnMessage(meter string, allowance, used, overageRateCents int64) (QuotaWarning, bool) {
	if !ShouldWarn(allowance, used) {
		return QuotaWarning{}, false
	}
	pct := 100
	if allowance > 0 {
		pct = int((used * 100) / allowance)
	}
	remaining := allowance - used
	if remaining < 0 {
		remaining = 0
	}
	return QuotaWarning{
		Meter: meter, Used: used, Allowance: allowance, PercentUsed: pct,
		OverageRate: overageRateCents, RemainingUnits: remaining,
	}, true
}
