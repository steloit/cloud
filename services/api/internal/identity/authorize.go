package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// AccessDeniedError carries the E3 explanation (missing role OR denying
// policy) that the 403 body must name.
type AccessDeniedError struct{ DeniedBy string }

func (e AccessDeniedError) Error() string { return "identity: access denied: " + e.DeniedBy }

// DeniedReason exposes the denial cause across a package boundary, so callers
// that cannot import identity — `events.Streamer`, which identity itself
// imports — can still tell a no-standing denial from a lacks-permission one.
// Without it the SSE half of an endpoint answers 403 where the JSON half
// answers 404, and one request header reopens the existence oracle the 404 was
// added to close (api-conventions.md; US-3.8 architecture review).
func (e AccessDeniedError) DeniedReason() string { return e.DeniedBy }

// Authorizer binds the pure evaluator to membership. Every mutating handler
// in every module routes through this — the governed-resource contract's
// clause (3); no module ships its own authZ.
type Authorizer struct {
	q         *store.Queries
	evaluator *rbac.Evaluator
	rec       *events.Recorder
}

func NewAuthorizer(q *store.Queries, e *rbac.Evaluator, rec *events.Recorder) *Authorizer {
	return &Authorizer{q: q, evaluator: e, rec: rec}
}

// ValidGrantable reports whether a permission may be granted to an org key
// (ADR-0007): a registered, non-delegated matrix permission. Delegated
// permissions (AI Law 1) are never grantable directly.
func (a *Authorizer) ValidGrantable(perm rbac.Permission) bool {
	return a.evaluator.Known(perm) && !a.evaluator.Delegated(perm)
}

// Holds reports whether the principal currently has a permission in scope,
// WITHOUT auditing — used for mint-time CEILING checks: a delegation can never
// exceed the delegator's own authority (ADR-0007 — the granted subset is a
// ceiling, and you cannot grant what you do not hold). Mirrors Require's
// per-principal branching but is side-effect-free.
func (a *Authorizer) Holds(ctx context.Context, p session.Principal, perm rbac.Permission, scope rbac.Scope) bool {
	if p.IsOrgKey() {
		if scope.OrgID == "" || scope.OrgID != p.OrgID {
			return false
		}
		if a.evaluator.Delegated(perm) || !a.evaluator.Known(perm) {
			return false
		}
		granted := false
		for _, g := range p.Permissions {
			if rbac.Permission(g) == perm {
				granted = true
				break
			}
		}
		return granted && a.evaluator.CheckPolicies(ctx, perm, scope).Allowed
	}
	role, err := a.q.GetMemberRole(ctx, store.GetMemberRoleParams{OrgID: scope.OrgID, UserID: p.UserID})
	if err != nil {
		return false
	}
	return a.evaluator.Check(ctx, rbac.Role(role), perm, scope).Allowed
}

// Require resolves the principal's role in the scope's org and runs the
// two-layer check. read_only bearer tokens are ceiling-limited to
// non-mutating checks by the caller (requireUser); role logic lives here.
// DENIALS ARE AUDITED (US-2.2 / spine contract: "grants, denials — yes,
// denials"): every deny lands on the spine naming what denied it.
func (a *Authorizer) Require(ctx context.Context, p session.Principal, perm rbac.Permission, scope rbac.Scope) error {
	// ADR-0007: an org-key principal authorizes against its explicit granted
	// permission subset (one canonical matrix vocabulary, no role, no
	// membership) — then policies may still NARROW, never widen.
	if p.IsOrgKey() {
		return a.requireOrgKey(ctx, p, perm, scope)
	}
	role, err := a.q.GetMemberRole(ctx, store.GetMemberRoleParams{OrgID: scope.OrgID, UserID: p.UserID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return a.deny(ctx, p, perm, scope, "membership:none — you are not a member of this organization")
		}
		return fmt.Errorf("identity: role lookup: %w", err)
	}
	d := a.evaluator.Check(ctx, rbac.Role(role), perm, scope)
	if !d.Allowed {
		return a.deny(ctx, p, perm, scope, d.DeniedBy)
	}
	return nil
}

// requireOrgKey authorizes a service-principal against its granted subset.
// The key's org must match the scope; the permission must be in the key's
// list AND be a real, non-delegated matrix permission (deny-by-default for
// unknown or delegated perms, exactly as for roles); then policies narrow.
func (a *Authorizer) requireOrgKey(ctx context.Context, p session.Principal, perm rbac.Permission, scope rbac.Scope) error {
	if scope.OrgID == "" || scope.OrgID != p.OrgID {
		// audit into the KEY's own org (the requested target may not exist)
		return a.deny(ctx, p, perm, rbac.Scope{OrgID: p.OrgID}, "key:scoped to a different organization")
	}
	if a.evaluator.Delegated(perm) {
		return a.deny(ctx, p, perm, scope,
			fmt.Sprintf("permission:%s is evaluated per underlying action — check that permission instead (AI Law 1)", perm))
	}
	if !a.evaluator.Known(perm) {
		return a.deny(ctx, p, perm, scope, fmt.Sprintf("permission:%s is not registered in the matrix", perm))
	}
	granted := false
	for _, g := range p.Permissions {
		if rbac.Permission(g) == perm {
			granted = true
			break
		}
	}
	if !granted {
		return a.deny(ctx, p, perm, scope, fmt.Sprintf("key:not granted %s (least-privilege — add it to the key's scope)", perm))
	}
	// policies still apply (tighten-only) to service principals
	if d := a.evaluator.CheckPolicies(ctx, perm, scope); !d.Allowed {
		return a.deny(ctx, p, perm, scope, d.DeniedBy)
	}
	return nil
}

// deny records the denial on the spine and returns the explained 403.
// Policy denials land as policy_trigger, everything else as membership.
func (a *Authorizer) deny(ctx context.Context, p session.Principal, perm rbac.Permission, scope rbac.Scope, deniedBy string) error {
	if a.rec != nil && scope.OrgID != "" {
		kind := "membership"
		if strings.HasPrefix(deniedBy, "policy:") {
			kind = "policy_trigger"
		}
		actor := p.UserID
		if actor == "" {
			actor = p.TokenID
		}
		if _, err := a.rec.Append(ctx, events.Input{
			OrgID: scope.OrgID, Kind: kind, Via: "user", Actor: actor,
			Action: "authz.denied", Subject: string(perm),
			Detail: []byte(`{"denied_by":` + strconv.Quote(deniedBy) + `}`),
		}); err != nil {
			slog.Error("events: denial audit failed", "perm", perm, "org", scope.OrgID, "err", err)
		}
	}
	return AccessDeniedError{DeniedBy: deniedBy}
}

// AddMember is the raw membership insert (role validity is DB-checked).
// actorID is who performed the change — never inferred from the added user.
func (s *Service) AddMember(ctx context.Context, orgID, userID, role, actorID string) error {
	_, err := s.q.AddMember(ctx, store.AddMemberParams{
		ID: ids.New("mbr"), OrgID: orgID, UserID: userID, Role: role,
	})
	if err != nil {
		return err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "membership", Via: "user", Actor: actorID,
		Action: "member.added", Subject: userID,
		Detail: []byte(`{"role":` + strconv.Quote(role) + `}`),
	})
	return nil
}

// record appends to the spine; ledger failures after a committed state change
// are logged loudly, never swallowed silently and never able to roll back the
// change they describe. (A tx-outbox lands with the provisioning epics.)
func (s *Service) record(ctx context.Context, in events.Input) {
	if s.rec == nil {
		return // recorder is optional only in unit-test worlds
	}
	if _, err := s.rec.Append(ctx, in); err != nil {
		slog.Error("events: append failed after state change", "action", in.Action, "org", in.OrgID, "err", err)
	}
}
