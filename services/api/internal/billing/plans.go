// Package billing is the ONE pricing + quota table (T11.1), loaded from
// plans.json as data. The estimate engine, the quota evaluator (T11.5), and the
// invoice generator (T11.3) all read THIS — a second pricing constant anywhere
// is a bug (billing pack). Money is integer cents end-to-end (ADR-025); the
// shown estimate line IS the invoice line, so all three must price identically.
package billing

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed plans.json
var plansJSON []byte

// Plan is a subscription tier's fee + included allowances. A nil FeeCents means
// custom/negotiated (Enterprise) — never a hard-coded number. A -1 limit is
// unlimited.
type Plan struct {
	FeeCents      *int `json:"fee_cents"`
	ProjectLimit  int  `json:"project_limit"`
	IncludedSeats int  `json:"included_seats"`
}

// Overage is the soft-quota unit-price schedule (plan-independent).
type Overage struct {
	EgressCentsPerGB     int `json:"egress_cents_per_gb"`
	SeatCents            int `json:"seat_cents"`
	BuildCentsPerMin     int `json:"build_cents_per_min"`
	EventCentsPerMillion int `json:"event_cents_per_million"`
	AICentsPer1k         int `json:"ai_cents_per_1k"`
}

// Table is the parsed pricing/quota data.
type Table struct {
	Plans      map[string]Plan `json:"plans"`
	Overage    Overage         `json:"overage"`
	NeverGated struct {
		Capabilities []string `json:"capabilities"`
	} `json:"never_gated"`
}

// Load parses the embedded table and validates the invariants: known plans
// present, fees non-negative, allowances sane. Any deviation fails boot —
// pricing data must never be silently wrong. ("$note" annotation keys are
// ignored; the invariant checks and the canon test guard the real values.)
func Load() (*Table, error) { return parse(plansJSON) }

// parse is Load's testable core: decode + validate (any bad data fails, so a
// typo can never silently ship a $0 meter or an unlimited allowance).
func parse(data []byte) (*Table, error) {
	var t Table
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("billing: parse plans.json: %w", err)
	}
	for _, name := range []string{"free", "pro", "business", "enterprise"} {
		p, ok := t.Plans[name]
		if !ok {
			return nil, fmt.Errorf("billing: plan %q missing from the table", name)
		}
		if p.FeeCents != nil && *p.FeeCents < 0 {
			return nil, fmt.Errorf("billing: plan %q has a negative fee", name)
		}
		// -1 = unlimited; anything below is a typo that would silently unlimit.
		if p.ProjectLimit < -1 || p.IncludedSeats < -1 {
			return nil, fmt.Errorf("billing: plan %q has an invalid allowance (< -1)", name)
		}
	}
	// Every overage rate must be a positive price — a typo'd 0 would silently
	// bill an entire meter for free.
	for label, cents := range map[string]int{
		"egress_cents_per_gb":     t.Overage.EgressCentsPerGB,
		"seat_cents":              t.Overage.SeatCents,
		"build_cents_per_min":     t.Overage.BuildCentsPerMin,
		"event_cents_per_million": t.Overage.EventCentsPerMillion,
		"ai_cents_per_1k":         t.Overage.AICentsPer1k,
	} {
		if cents <= 0 {
			return nil, fmt.Errorf("billing: overage %q must be a positive price, got %d", label, cents)
		}
	}
	return &t, nil
}

// IncludedSeats is the seat allowance for a plan (0 for an unknown plan — fail
// closed). The seat gate reads this; there is no second seat constant.
func (t *Table) IncludedSeats(plan string) int {
	p, ok := t.Plans[plan]
	if !ok {
		return 0
	}
	return p.IncludedSeats
}

// Plan returns a tier by name (the second result is false for an unknown tier —
// deny-by-default: an unknown plan is a programming error, never a silent free).
func (t *Table) Plan(name string) (Plan, bool) {
	p, ok := t.Plans[name]
	return p, ok
}

// ProjectLimit is the B5 "Projects" allowance for a plan (-1 = unlimited, 0 for
// an unknown plan — fail closed). Replaces the hard-coded provisioning map.
func (t *Table) ProjectLimit(plan string) int {
	p, ok := t.Plans[plan]
	if !ok {
		return 0
	}
	return p.ProjectLimit
}

// PlanFeeCents returns the monthly fee for a plan; ok is false for a
// custom-priced tier (Enterprise) or an unknown plan.
func (t *Table) PlanFeeCents(plan string) (int, bool) {
	p, ok := t.Plans[plan]
	if !ok || p.FeeCents == nil {
		return 0, false
	}
	return *p.FeeCents, true
}

// SpendToDate is the org's month-to-date spend: the plan fee plus every metered
// accrual. It is the SAME arithmetic the invoice freezes (plan fee + Σ
// quota_usage.rate_cents), so the number the hard cap (T11.6) enforces against
// is exactly the number the overview and invoice show — one arithmetic
// everywhere (F9). meteredRateCents are the per-meter accrued cents for the
// current period.
func SpendToDate(planFeeCents int64, meteredRateCents ...int64) int64 {
	total := planFeeCents
	for _, c := range meteredRateCents {
		total += c
	}
	return total
}

// IsNeverGated reports whether a capability is on the never-plan-gated safety
// list (billing pack / F9 — TLS, backups, MFA, policies, alerts, dunning,
// deletion). Plans gate capabilities, never safety.
func (t *Table) IsNeverGated(capability string) bool {
	for _, c := range t.NeverGated.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// CapabilityGate is the decision a caller renders: gate a capability behind a
// plan, EXCEPT safety — a never-gated capability is always permitted (US-11.1
// enforcement path, US-11.2). The caller maps Gated → problem.PlanGated; a nil
// return means "permit, do not gate". `has` reports whether the current plan
// already includes the capability (callers with a real capability→plan matrix
// pass it; today no capability matrix exists, so safety is the only rule wired).
//
// The invariant this closes: a plan gate must consult IsNeverGated FIRST, so a
// safety capability (TLS, backups, MFA, policies, alerts, dunning, deletion) is
// never plan-gated at the call site — not merely absent from a list.
func (t *Table) GateCapability(plan, capability string, has bool) (requiredPlan string, gated bool) {
	if t.IsNeverGated(capability) {
		return "", false // safety is never gated, whatever the plan
	}
	if has {
		return "", false // the plan includes it
	}
	return plan, true // a real capability the plan lacks → gate
}
