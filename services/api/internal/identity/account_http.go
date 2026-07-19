package identity

// T7.6: leave-org + account self-deletion. The last-owner rule holds
// (F1/US-2.4); self-deletion is NEVER plan-gated (a person can always leave);
// scheduling honors grace windows (org billing data = 90-day rule, T2.7;
// account = grace before purge).

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// DeleteAccount schedules the caller's account for deletion. Blocked (409) if
// they are the SOLE owner of any org — each named — so ownership is never
// orphaned (the last-owner rule at account scope). Never plan-gated.
func (h *Handlers) DeleteAccount(ctx context.Context, _ gen.DeleteAccountRequestObject) (gen.DeleteAccountResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	orgs, blockers, err := h.svc.ScheduleAccountDeletion(ctx, p.UserID)
	if err != nil {
		if errors.Is(err, ErrOrgDeleting) {
			return nil, accountBlockedError{reasons: []string{"account deletion already scheduled"},
				remediation: "Your account is already scheduled for deletion within the grace window."}
		}
		return nil, err
	}
	if len(blockers) > 0 {
		reasons := make([]string, 0, len(blockers))
		for _, o := range blockers {
			reasons = append(reasons, "you are the sole owner of "+o.Name+" ("+o.ID+")")
		}
		return nil, accountBlockedError{reasons: reasons,
			remediation: "Transfer ownership or delete these organizations first — deletion never orphans an org."}
	}
	// spine fact in every org the user still belongs to: their account is
	// scheduled to vanish (visible to /audit, the bell, webhooks).
	for _, orgID := range orgs {
		h.svc.record(ctx, events.Input{
			OrgID: orgID, Kind: "membership", Via: "user", Actor: p.UserID,
			Action: "member.account_deletion_scheduled", Subject: p.UserID,
		})
	}
	if c := session.CarrierFrom(ctx); c != nil {
		c.Add(h.mgr.ClearCookie())
	}
	return gen.DeleteAccount202Response{}, nil
}

// LeaveOrg removes the caller's own membership. The last owner cannot leave
// (F1, DB trigger → 409). The account and other memberships are untouched —
// unlike an admin removal, leaving one org does NOT sign you out everywhere.
func (h *Handlers) LeaveOrg(ctx context.Context, req gen.LeaveOrgRequestObject) (gen.LeaveOrgResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	m, err := h.svc.q.RemoveOwnMembership(ctx, store.RemoveOwnMembershipParams{OrgID: req.OrgPathParam, UserID: p.UserID})
	if err != nil {
		if isLastOwner(err) {
			return nil, ErrLastOwner
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notFoundError{what: "membership"} // not a member of this org
		}
		return nil, err // infra error → 500, never a misleading 404
	}
	h.svc.record(ctx, events.Input{
		OrgID: req.OrgPathParam, Kind: "membership", Via: "user", Actor: p.UserID,
		Action: "member.left", Subject: p.UserID,
		Detail: []byte(`{"role":` + strconv.Quote(m.Role) + `}`),
	})
	return gen.LeaveOrg204Response{}, nil
}

// accountBlockedError is a 409 with reasons that maps through the problem seam.
type accountBlockedError struct {
	reasons     []string
	remediation string
}

func (e accountBlockedError) Error() string { return "identity: account action blocked" }
func (e accountBlockedError) Problem() problem.Problem {
	return problem.Conflict(e.reasons, e.remediation)
}
