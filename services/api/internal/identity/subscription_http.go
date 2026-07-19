package identity

// Q9/US-11.3 core: the subscription surface. changePlan — upgrades immediate,
// CLEAN downgrades at the anchor, blocked downgrades 409 with EVERY reason
// verbatim (B4: "the button says why"); cancel = plan ≠ resources (B12).
// The dunning{} object is DERIVED from status + timestamps, never stored.

import (
	"context"
	"strconv"
	"time"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
	"github.com/steloit/cloud/services/api/internal/subscription"
)

func (h *Handlers) GetSubscription(ctx context.Context, req gen.GetSubscriptionRequestObject) (gen.GetSubscriptionResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "billing.view", false); err != nil {
		return nil, err
	}
	sub, err := h.subs.Get(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	return gen.GetSubscription200JSONResponse(subscriptionToAPI(sub, h.svc.now())), nil
}

func (h *Handlers) ChangePlan(ctx context.Context, req gen.ChangePlanRequestObject) (gen.ChangePlanResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "subscription.change", true); err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "plan", Detail: "required"}}}
	}
	target := string(req.Body.Plan)
	if _, ok := h.svc.plans.Plan(target); !ok {
		return nil, validationError{fields: []problem.FieldError{{Field: "plan", Detail: "unknown plan"}}}
	}
	sub, err := h.subs.Get(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	// Downgrade gate (B4): every blocking reason, verbatim, with remediation —
	// never the first one found. Evaluable dimensions today: members vs the
	// target's included seats, projects vs the target's project limit. (Cells
	// are not yet data — the canon's "2 cells" reason lands with E1 runtime;
	// finding recorded.)
	if rankOf(target) < rankOf(sub.Plan) {
		var reasons []problem.Reason
		members, err := h.svc.q.CountMembers(ctx, req.OrgPathParam)
		if err != nil {
			return nil, err
		}
		if seats := h.svc.plans.IncludedSeats(target); int(members) > seats {
			reasons = append(reasons, problem.Reason{
				Code:        "members_over_target_seats",
				Message:     strconv.Itoa(int(members)) + " members > " + planTitle(target) + "'s " + strconv.Itoa(seats) + " included seats",
				Remediation: "Remove members or keep a plan with enough seats.",
			})
		}
		projects, err := h.svc.q.CountProjects(ctx, req.OrgPathParam)
		if err != nil {
			return nil, err
		}
		if limit := h.svc.plans.ProjectLimit(target); limit >= 0 && int(projects) > limit {
			reasons = append(reasons, problem.Reason{
				Code:        "projects_over_target_limit",
				Message:     strconv.Itoa(int(projects)) + " projects > " + planTitle(target) + "'s " + strconv.Itoa(limit) + " project limit",
				Remediation: "Delete projects or keep a plan with a higher limit.",
			})
		}
		if len(reasons) > 0 {
			return nil, downgradeBlockedError{reasons: reasons}
		}
	}
	out, _, err := h.subs.ChangePlan(ctx, req.OrgPathParam, target)
	if err != nil {
		return nil, err
	}
	return gen.ChangePlan200JSONResponse(subscriptionToAPI(out, h.svc.now())), nil
}

func (h *Handlers) CancelSubscription(ctx context.Context, req gen.CancelSubscriptionRequestObject) (gen.CancelSubscriptionResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "subscription.change", true); err != nil {
		return nil, err
	}
	reason := ""
	if req.Body != nil && req.Body.ReasonCode != nil {
		reason = *req.Body.ReasonCode
	}
	out, err := h.subs.Cancel(ctx, req.OrgPathParam, reason)
	if err != nil {
		return nil, err
	}
	return gen.CancelSubscription200JSONResponse(subscriptionToAPI(out, h.svc.now())), nil
}

// subscriptionToAPI derives the wire shape — dunning{} and wind_down{} are
// projections of status + timestamps (never a second source of truth).
func subscriptionToAPI(sub store.Subscription, now time.Time) gen.Subscription {
	out := gen.Subscription{
		Plan: gen.Plan(sub.Plan), Status: gen.SubscriptionStatus(sub.Status),
		AnchorDay: int(sub.AnchorDay),
	}
	if sub.TrialEndsAt.Valid {
		t := sub.TrialEndsAt.Time
		out.TrialEndsAt = &t
	}
	if sub.DunningStartedAt.Valid {
		day := int(now.Sub(sub.DunningStartedAt.Time).Hours()) / 24
		d := struct {
			Day         *int                          `json:"day,omitempty"`
			NextRetryAt *time.Time                    `json:"next_retry_at,omitempty"`
			State       *gen.SubscriptionDunningState `json:"state,omitempty"`
		}{Day: &day}
		// state carries only the dunning vocabulary — a cancelled-while-dunning
		// row keeps day/retry but no out-of-vocab state on the wire.
		switch sub.Status {
		case "current", "grace", "provisioning_paused", "suspended":
			st := gen.SubscriptionDunningState(sub.Status)
			d.State = &st
		}
		if sub.NextRetryAt.Valid {
			r := sub.NextRetryAt.Time
			d.NextRetryAt = &r
		}
		out.Dunning = &d
	}
	if sub.Status == "cancelled_at_anchor" && sub.PlanEndsAt.Valid {
		ends := sub.PlanEndsAt.Time
		unaffected := gen.SubscriptionWindDownResourcesUnaffected(true)
		out.WindDown = &struct {
			PlanEndsAt          *time.Time                                   `json:"plan_ends_at,omitempty"`
			ResourcesUnaffected *gen.SubscriptionWindDownResourcesUnaffected `json:"resources_unaffected,omitempty"`
		}{PlanEndsAt: &ends, ResourcesUnaffected: &unaffected}
	}
	return out
}

func rankOf(plan string) int { return subscription.PlanRank(plan) }

func planTitle(p string) string {
	switch p {
	case "free":
		return "Free"
	case "pro":
		return "Pro"
	case "business":
		return "Business"
	case "enterprise":
		return "Enterprise"
	}
	return p
}

// downgradeBlockedError is the B4 409: EVERY blocking reason, verbatim, each
// with remediation — maps through the problem.Carrier seam.
type downgradeBlockedError struct{ reasons []problem.Reason }

func (e downgradeBlockedError) Error() string { return "identity: downgrade blocked" }
func (e downgradeBlockedError) Problem() problem.Problem {
	return problem.ConflictReasons(e.reasons,
		"Resolve every listed reason, then retry the downgrade — it takes effect at your billing anchor.")
}
