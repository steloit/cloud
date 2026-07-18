package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// AccessDeniedError carries the E3 explanation (missing role OR denying
// policy) that the 403 body must name.
type AccessDeniedError struct{ DeniedBy string }

func (e AccessDeniedError) Error() string { return "identity: access denied: " + e.DeniedBy }

// Authorizer binds the pure evaluator to membership. Every mutating handler
// in every module routes through this — the governed-resource contract's
// clause (3); no module ships its own authZ.
type Authorizer struct {
	q         *store.Queries
	evaluator *rbac.Evaluator
}

func NewAuthorizer(q *store.Queries, e *rbac.Evaluator) *Authorizer {
	return &Authorizer{q: q, evaluator: e}
}

// Require resolves the principal's role in the scope's org and runs the
// two-layer check. read_only bearer tokens are ceiling-limited to
// non-mutating checks by the caller (requireUser); role logic lives here.
func (a *Authorizer) Require(ctx context.Context, p session.Principal, perm rbac.Permission, scope rbac.Scope) error {
	role, err := a.q.GetMemberRole(ctx, store.GetMemberRoleParams{OrgID: scope.OrgID, UserID: p.UserID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AccessDeniedError{DeniedBy: "membership:none — you are not a member of this organization"}
		}
		return fmt.Errorf("identity: role lookup: %w", err)
	}
	d := a.evaluator.Check(ctx, rbac.Role(role), perm, scope)
	if !d.Allowed {
		return AccessDeniedError{DeniedBy: d.DeniedBy}
	}
	return nil
}

// --- org bootstrap (consumed by T2.7's endpoints; used by tests now) --------

// CreateOrgWithOwner creates the org and its owner membership atomically-ish
// (two inserts; the org endpoints task wraps them in a tx when it lands).
func (s *Service) CreateOrgWithOwner(ctx context.Context, name, ownerUserID string) (store.Org, error) {
	org, err := s.q.CreateOrg(ctx, store.CreateOrgParams{ID: ids.New("org"), Name: name})
	if err != nil {
		return store.Org{}, fmt.Errorf("identity: create org: %w", err)
	}
	if _, err := s.q.AddMember(ctx, store.AddMemberParams{
		ID: ids.New("mbr"), OrgID: org.ID, UserID: ownerUserID, Role: "owner",
	}); err != nil {
		return store.Org{}, fmt.Errorf("identity: owner membership: %w", err)
	}
	return org, nil
}

// AddMember is the raw membership insert (role validity is DB-checked).
func (s *Service) AddMember(ctx context.Context, orgID, userID, role string) error {
	_, err := s.q.AddMember(ctx, store.AddMemberParams{
		ID: ids.New("mbr"), OrgID: orgID, UserID: userID, Role: role,
	})
	return err
}
