package identity

// T2.7: the strict handlers for org / member / invite governance + org API
// keys. Handlers translate; the service decides (§15).

import (
	"context"
	"errors"
	"strings"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// ---- orgs ------------------------------------------------------------------

func orgToAPI(o store.Org) gen.Org {
	return gen.Org{
		Id: o.ID, Slug: o.Slug, Name: o.Name, HomeRegion: o.HomeRegion,
		Plan: gen.Plan(o.Plan), CreatedAt: o.CreatedAt.Time,
	}
}

func (h *Handlers) CreateOrg(ctx context.Context, req gen.CreateOrgRequestObject) (gen.CreateOrgResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || strings.TrimSpace(req.Body.Name) == "" {
		return nil, validationError{fields: []problem.FieldError{{Field: "name", Detail: "required"}}}
	}
	region := "aws/ap-south-1"
	if req.Body.HomeRegion != nil && *req.Body.HomeRegion != "" {
		region = *req.Body.HomeRegion
	}
	org, err := h.svc.CreateOrgFull(ctx, req.Body.Name, region, p.UserID)
	if err != nil {
		return nil, err
	}
	return gen.CreateOrg201JSONResponse(orgToAPI(org)), nil
}

func (h *Handlers) ListMyOrgs(ctx context.Context, _ gen.ListMyOrgsRequestObject) (gen.ListMyOrgsResponseObject, error) {
	p, err := requireUser(ctx, false)
	if err != nil {
		return nil, err
	}
	rows, err := h.svc.ListOrgsForUser(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Org, 0, len(rows))
	for _, o := range rows {
		data = append(data, orgToAPI(o))
	}
	return gen.ListMyOrgs200JSONResponse(gen.OrgList{Data: &data}), nil
}

// requireOrg authorizes a permission in an org and returns the acting user.
// Unlike requireUser it accepts org-key principals (no UserID): they are
// AUTHENTICATED, so a missing grant is 403 through the evaluator
// (membership:none until the G8 scope model lands), never a 401.
func (h *Handlers) requireOrg(ctx context.Context, orgID string, perm rbac.Permission, mutating bool) (string, error) {
	p, ok := session.PrincipalFrom(ctx)
	if !ok {
		return "", ErrNoSession
	}
	if mutating && p.Kind == "token" && p.Scope != "full" {
		return "", ErrScopeDenied
	}
	if err := h.authz.Require(ctx, p, perm, rbac.Scope{OrgID: orgID}); err != nil {
		return "", err
	}
	return p.UserID, nil
}

func (h *Handlers) GetOrg(ctx context.Context, req gen.GetOrgRequestObject) (gen.GetOrgResponseObject, error) {
	// Any member may read their org: membership itself is the grant.
	p, err := requireUser(ctx, false)
	if err != nil {
		return nil, err
	}
	if _, err := h.svc.q.GetMemberRole(ctx, store.GetMemberRoleParams{OrgID: req.OrgPathParam, UserID: p.UserID}); err != nil {
		return nil, notFoundError{what: "organization"} // non-members can't probe org ids
	}
	org, err := h.svc.GetOrg(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	return gen.GetOrg200JSONResponse(orgToAPI(org)), nil
}

func (h *Handlers) UpdateOrg(ctx context.Context, req gen.UpdateOrgRequestObject) (gen.UpdateOrgResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, "org.manage", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "body", Detail: "required"}}}
	}
	org, err := h.svc.UpdateOrg(ctx, req.OrgPathParam, actor, req.Body.Name, req.Body.HomeRegion)
	if err != nil {
		return nil, err
	}
	return gen.UpdateOrg200JSONResponse(orgToAPI(org)), nil
}

func (h *Handlers) DeleteOrg(ctx context.Context, req gen.DeleteOrgRequestObject) (gen.DeleteOrgResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, "org.delete", true)
	if err != nil {
		return nil, err
	}
	if err := h.svc.ScheduleOrgDeletion(ctx, req.OrgPathParam, actor); err != nil {
		return nil, err
	}
	return gen.DeleteOrg202Response{}, nil
}

// ---- members ---------------------------------------------------------------

func memberToAPI(m store.ListMembersRow) gen.Member {
	out := gen.Member{Id: m.ID, UserId: m.UserID, Email: m.Email, Role: gen.Role(m.Role)}
	if m.Name != "" {
		n := m.Name
		out.Name = &n
	}
	mfa := false // MFA ships with T7.x; posture is honest until then
	out.MfaEnabled = &mfa
	joined := m.CreatedAt.Time
	out.JoinedAt = &joined
	return out
}

func (h *Handlers) ListMembers(ctx context.Context, req gen.ListMembersRequestObject) (gen.ListMembersResponseObject, error) {
	p, err := requireUser(ctx, false)
	if err != nil {
		return nil, err
	}
	if _, err := h.svc.q.GetMemberRole(ctx, store.GetMemberRoleParams{OrgID: req.OrgPathParam, UserID: p.UserID}); err != nil {
		return nil, notFoundError{what: "organization"}
	}
	org, err := h.svc.GetOrg(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	rows, err := h.svc.ListMembers(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	seats, err := h.svc.Seats(ctx, org)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Member, 0, len(rows))
	for _, m := range rows {
		data = append(data, memberToAPI(m))
	}
	out := gen.MemberList{Data: &data}
	out.Seats = &struct {
		Included          *int `json:"included,omitempty"`
		OveragePriceCents *int `json:"overage_price_cents,omitempty"`
		Used              *int `json:"used,omitempty"`
	}{Included: &seats.Included, OveragePriceCents: &seats.OveragePriceCents, Used: &seats.Used}
	return gen.ListMembers200JSONResponse(out), nil
}

func (h *Handlers) ChangeMemberRole(ctx context.Context, req gen.ChangeMemberRoleRequestObject) (gen.ChangeMemberRoleResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, "members.role_change", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "role", Detail: "required"}}}
	}
	m, err := h.svc.ChangeMemberRole(ctx, req.OrgPathParam, req.Member, string(req.Body.Role), actor)
	if err != nil {
		return nil, err
	}
	row, err := h.svc.q.GetMember(ctx, store.GetMemberParams{OrgID: req.OrgPathParam, ID: m.ID})
	if err != nil {
		return nil, err
	}
	return gen.ChangeMemberRole200JSONResponse(memberToAPI(store.ListMembersRow(row))), nil
}

func (h *Handlers) RemoveMember(ctx context.Context, req gen.RemoveMemberRequestObject) (gen.RemoveMemberResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, "members.role_change", true)
	if err != nil {
		return nil, err
	}
	flagged, err := h.svc.RemoveMember(ctx, req.OrgPathParam, req.Member, actor)
	if err != nil {
		return nil, err
	}
	out := gen.RemoveMember200JSONResponse{}
	items := make([]struct {
		Id   *string `json:"id,omitempty"`
		Kind *string `json:"kind,omitempty"`
		Name *string `json:"name,omitempty"`
	}, 0, len(flagged))
	for i := range flagged {
		f := flagged[i]
		items = append(items, struct {
			Id   *string `json:"id,omitempty"`
			Kind *string `json:"kind,omitempty"`
			Name *string `json:"name,omitempty"`
		}{Id: &f.ID, Kind: &f.Kind, Name: &f.Name})
	}
	out.FlaggedResources = &items
	return out, nil
}

// ---- invites ---------------------------------------------------------------

func inviteToAPI(inv store.Invite) gen.Invite {
	out := gen.Invite{
		Id: inv.ID, Email: inv.Email, Role: gen.Role(inv.Role),
		Status: gen.InviteStatus(InviteStatus(inv)), ExpiresAt: inv.ExpiresAt.Time,
	}
	inviter := inv.InviterID
	out.InviterId = &inviter
	return out
}

func (h *Handlers) CreateInvite(ctx context.Context, req gen.CreateInviteRequestObject) (gen.CreateInviteResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, "members.invite", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "body", Detail: "required"}}}
	}
	confirm := req.Params.Confirm != nil && *req.Params.Confirm
	inv, err := h.svc.CreateInvite(ctx, req.OrgPathParam, string(req.Body.Email), string(req.Body.Role), actor, confirm)
	if err != nil {
		return nil, err
	}
	return gen.CreateInvite201JSONResponse(inviteToAPI(inv)), nil
}

func (h *Handlers) ListInvites(ctx context.Context, req gen.ListInvitesRequestObject) (gen.ListInvitesResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "members.invite", false); err != nil {
		return nil, err
	}
	rows, err := h.svc.ListInvites(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Invite, 0, len(rows))
	for _, inv := range rows {
		data = append(data, inviteToAPI(inv))
	}
	return gen.ListInvites200JSONResponse(gen.InviteList{Data: &data}), nil
}

func (h *Handlers) RevokeInvite(ctx context.Context, req gen.RevokeInviteRequestObject) (gen.RevokeInviteResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, "members.invite", true)
	if err != nil {
		return nil, err
	}
	if err := h.svc.RevokeInvite(ctx, req.OrgPathParam, req.Invite, actor); err != nil {
		return nil, err
	}
	return gen.RevokeInvite204Response{}, nil
}

func (h *Handlers) GetInvitePublic(ctx context.Context, req gen.GetInvitePublicRequestObject) (gen.GetInvitePublicResponseObject, error) {
	view, err := h.svc.PublicInviteView(ctx, req.Invite)
	if err != nil {
		if errors.Is(err, ErrInviteGone) {
			return gen.GetInvitePublic410Response{}, nil
		}
		return nil, err
	}
	out := gen.InvitePublic{
		InviterName: &view.InviterName, OrgName: &view.OrgName,
		EmailHint: &view.EmailHint, Status: &view.Status,
	}
	role := gen.Role(view.Role)
	out.Role = &role
	rc := view.RoleConsequences
	out.RoleConsequences = &rc
	return gen.GetInvitePublic200JSONResponse(out), nil
}

func (h *Handlers) AcceptInvite(ctx context.Context, req gen.AcceptInviteRequestObject) (gen.AcceptInviteResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	err = h.svc.AcceptInvite(ctx, req.Invite, p.UserID)
	switch {
	case err == nil:
		return gen.AcceptInvite200Response{}, nil
	case errors.Is(err, ErrWrongAccount):
		return gen.AcceptInvite403Response{}, nil
	default:
		return nil, err
	}
}

func (h *Handlers) DeclineInvite(ctx context.Context, req gen.DeclineInviteRequestObject) (gen.DeclineInviteResponseObject, error) {
	if err := h.svc.DeclineInvite(ctx, req.Invite); err != nil {
		return nil, err
	}
	return gen.DeclineInvite204Response{}, nil
}

func (h *Handlers) RenewInvite(ctx context.Context, req gen.RenewInviteRequestObject) (gen.RenewInviteResponseObject, error) {
	if err := h.svc.RenewInvite(ctx, req.Invite); err != nil {
		return nil, err
	}
	return gen.RenewInvite202Response{}, nil
}

// ---- org API keys (G8) -----------------------------------------------------

func (h *Handlers) CreateApiKey(ctx context.Context, req gen.CreateApiKeyRequestObject) (gen.CreateApiKeyResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "api_keys.manage", true); err != nil {
		return nil, err
	}
	if req.Body == nil || strings.TrimSpace(req.Body.Name) == "" {
		return nil, validationError{fields: []problem.FieldError{{Field: "name", Detail: "required"}}}
	}
	scope := "read_only" // least-privilege default (G8)
	if req.Body.Scope != nil {
		scope = string(*req.Body.Scope)
	}
	days := 90
	if req.Body.ExpiresInDays != nil {
		days = int(*req.Body.ExpiresInDays)
	}
	minted, err := h.svc.MintOrgKey(ctx, req.OrgPathParam, req.Body.Name, scope, days)
	if err != nil {
		return nil, err
	}
	out := gen.TokenCreated{
		Id: minted.Row.ID, Token: minted.Secret, ShownOnce: true,
		Prefix: minted.Row.Prefix, HashStored: true,
	}
	if minted.Row.ExpiresAt.Valid {
		out.ExpiresAt = &minted.Row.ExpiresAt.Time
	}
	return gen.CreateApiKey201JSONResponse(out), nil
}

func (h *Handlers) ListApiKeys(ctx context.Context, req gen.ListApiKeysRequestObject) (gen.ListApiKeysResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "api_keys.manage", false); err != nil {
		return nil, err
	}
	rows, err := h.svc.ListOrgKeys(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Token, 0, len(rows))
	for _, r := range rows {
		t := gen.Token{Id: r.ID, Name: r.Name, Prefix: r.Prefix, Scope: gen.TokenScope(r.Scope)}
		if r.ExpiresAt.Valid {
			t.ExpiresAt = &r.ExpiresAt.Time
		}
		if r.LastUsedAt.Valid {
			t.LastUsedAt = &r.LastUsedAt.Time
		}
		data = append(data, t)
	}
	return gen.ListApiKeys200JSONResponse(gen.TokenList{Data: &data}), nil
}
