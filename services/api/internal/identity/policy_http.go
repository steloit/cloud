package identity

// T12.1 policy authoring handlers. Governed resource (ADR-0006): every op
// authorizes through the Authorizer under policy.manage. Get/update are keyed by
// policy id, so they load the row first and hide a foreign org's policy as a 404
// (never a 403 existence leak).

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// actorFrom derives the audit actor from the principal: an org key is a machine
// (system, identified by its token id), a user/personal-token acts as the user.
func actorFrom(ctx context.Context) Actor {
	p, _ := session.PrincipalFrom(ctx)
	if p.IsOrgKey() {
		return Actor{ID: p.TokenID, Via: "system"}
	}
	id := p.UserID
	if id == "" {
		id = p.TokenID
	}
	return Actor{ID: id, Via: "user"}
}

func (h *Handlers) CreatePolicy(ctx context.Context, req gen.CreatePolicyRequestObject) (gen.CreatePolicyResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, "policy.manage", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.Key == "" {
		return nil, validationError{fields: []problem.FieldError{{Field: "key", Detail: "required"}}}
	}
	if req.Body.Rule == nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "rule", Detail: "required — a structured rule the platform can evaluate (G9)"}}}
	}
	enforcement := ""
	if req.Body.Enforcement != nil {
		enforcement = string(*req.Body.Enforcement)
	}
	enforcement, verr := ValidatePolicyEnforcement(req.Body.Key, enforcement)
	if verr != nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "enforcement", Detail: verr.Error()}}}
	}
	ruleJSON, _ := json.Marshal(req.Body.Rule)
	draft := PolicyDraft{
		OrgID: req.OrgPathParam, Key: req.Body.Key, ProjectID: policyProjectID(req.Body.Scope),
		Description: strPtr(req.Body.Description), Enforcement: enforcement,
		Rule: ruleJSON, EnforceFrom: req.Body.EnforceFrom,
	}

	// dry-run: impact preview, nothing persisted (G11 — preview before enforce).
	if req.Params.DryRun != nil && *req.Params.DryRun {
		impact, err := h.svc.PreviewPolicy(ctx, draft)
		if err != nil {
			return nil, err
		}
		return gen.CreatePolicy200JSONResponse(impactToAPI(impact)), nil
	}

	_ = actor // authorization only; the audit actor is derived from the principal
	pol, err := h.svc.CreatePolicy(ctx, draft, actorFrom(ctx))
	if err != nil {
		return nil, err // policyConflictError → 409 via the carrier seam
	}
	return gen.CreatePolicy201JSONResponse(h.policyToAPI(ctx, pol)), nil
}

func (h *Handlers) ListPolicies(ctx context.Context, req gen.ListPoliciesRequestObject) (gen.ListPoliciesResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "policy.manage", false); err != nil {
		return nil, err
	}
	pols, err := h.svc.ListPolicies(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	out := make([]gen.Policy, 0, len(pols))
	for _, p := range pols {
		out = append(out, h.policyToAPI(ctx, p))
	}
	return gen.ListPolicies200JSONResponse{Data: &out}, nil
}

func (h *Handlers) GetPolicy(ctx context.Context, req gen.GetPolicyRequestObject) (gen.GetPolicyResponseObject, error) {
	pol, err := h.policyScoped(ctx, req.Policy, "policy.manage", false)
	if err != nil {
		return nil, err
	}
	return gen.GetPolicy200JSONResponse(h.policyToAPI(ctx, pol)), nil
}

func (h *Handlers) UpdatePolicy(ctx context.Context, req gen.UpdatePolicyRequestObject) (gen.UpdatePolicyResponseObject, error) {
	pol, err := h.policyScoped(ctx, req.Policy, "policy.manage", true)
	if err != nil {
		return nil, err
	}
	patch := PolicyPatch{}
	if req.Body != nil {
		if req.Body.Enforcement != nil {
			e := string(*req.Body.Enforcement)
			patch.Enforcement = &e
		}
		if req.Body.Config != nil {
			patch.Config, _ = json.Marshal(*req.Body.Config)
		}
		patch.EnforceFrom = req.Body.EnforceFrom
	}
	updated, err := h.svc.UpdatePolicy(ctx, pol.ID, patch, actorFrom(ctx))
	if err != nil {
		return nil, err
	}
	return gen.UpdatePolicy200JSONResponse(h.policyToAPI(ctx, updated)), nil
}

// policyScoped loads a policy and authorizes the caller against its org. A
// policy in an org the caller can't act in is hidden as a 404, not a 403.
func (h *Handlers) policyScoped(ctx context.Context, policyID string, perm rbac.Permission, mutating bool) (store.Policy, error) {
	pol, err := h.svc.GetPolicy(ctx, policyID)
	if err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			return store.Policy{}, notFoundError{what: "policy"}
		}
		return store.Policy{}, err
	}
	if _, err := h.requireOrg(ctx, pol.OrgID, perm, mutating); err != nil {
		var ad AccessDeniedError
		// A principal with no standing in the org — a non-member (membership:none)
		// or an org key scoped to a different org (key:…) — must not learn the
		// policy exists: 404, not a 403 oracle. A member who merely lacks the
		// permission (role:…) gets the honest 403.
		if errors.As(err, &ad) && ad.AccessDeniedNoStanding() {
			return store.Policy{}, notFoundError{what: "policy"}
		}
		return store.Policy{}, err
	}
	return pol, nil
}

func (h *Handlers) policyToAPI(ctx context.Context, p store.Policy) gen.Policy {
	var rule map[string]interface{}
	_ = json.Unmarshal(p.Rule, &rule)
	if rule == nil {
		rule = map[string]interface{}{}
	}
	enf := gen.Enforcement(p.Enforcement)
	out := gen.Policy{
		Id: p.ID, Key: p.Key, Rule: rule, Version: int(p.Version), Enforcement: &enf,
	}
	if p.Description.Valid {
		out.Description = &p.Description.String
	}
	if p.LastChangeEvent.Valid {
		out.LastChangeEvent = &p.LastChangeEvent.String
	}
	if p.EnforceFrom.Valid {
		t := p.EnforceFrom.Time
		out.EnforceFrom = &t
	}
	if p.ProjectID.Valid {
		pid := p.ProjectID.String
		out.Scope = &struct {
			ProjectId *string `json:"project_id,omitempty"`
		}{ProjectId: &pid}
	}
	// warn-mode telemetry that makes promote-to-enforce a data-backed click.
	if v, err := h.svc.PolicyViolations30d(ctx, p.OrgID, p.Key); err == nil {
		out.ViolationCount30d = &v
	}
	return out
}

func impactToAPI(i PolicyImpact) gen.PolicyImpact {
	out := gen.PolicyImpact{MembersAffected: &i.MembersAffected}
	if len(i.Conflicts) > 0 {
		cs := make([]struct {
			Detail      *string   `json:"detail,omitempty"`
			Resolutions *[]string `json:"resolutions,omitempty"`
			WithPolicy  *string   `json:"with_policy,omitempty"`
		}, 0, len(i.Conflicts))
		for _, c := range i.Conflicts {
			c := c
			cs = append(cs, struct {
				Detail      *string   `json:"detail,omitempty"`
				Resolutions *[]string `json:"resolutions,omitempty"`
				WithPolicy  *string   `json:"with_policy,omitempty"`
			}{Detail: &c.Detail, Resolutions: &c.Resolutions, WithPolicy: &c.WithPolicy})
		}
		out.Conflicts = &cs
	}
	if len(i.Affected) > 0 {
		as := make([]struct {
			Effect *string `json:"effect,omitempty"`
			Id     *string `json:"id,omitempty"`
			Name   *string `json:"name,omitempty"`
		}, 0, len(i.Affected))
		for _, a := range i.Affected {
			a := a
			as = append(as, struct {
				Effect *string `json:"effect,omitempty"`
				Id     *string `json:"id,omitempty"`
				Name   *string `json:"name,omitempty"`
			}{Effect: &a.Effect, Id: &a.ID, Name: &a.Name})
		}
		out.Affected = &as
	}
	return out
}

func policyProjectID(scope *struct {
	ProjectId *string `json:"project_id,omitempty"`
}) string {
	if scope == nil || scope.ProjectId == nil {
		return ""
	}
	return *scope.ProjectId
}

func strPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
