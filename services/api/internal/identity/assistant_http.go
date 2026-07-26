package identity

// T13.3 — the /assistant/threads surface. Every read/write is gated by the
// ai-assistant policy (AI Law 4): a `disabled` org's AI surface returns 404
// empty-equivalent (createThread) or is silently omitted (listThreads) — the
// rows persist untouched and reappear the instant the policy is re-enabled.

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

// aiDisabled → 404 empty-equivalent (the AI layer is invisible when off, never
// a 403 that admits it could exist). Routed through the notFound seam.
func aiDisabled() error { return notFoundError{what: "assistant"} }

// ErrAssistantUserScoped: the conversational assistant is a HUMAN surface
// (drawer/workspace, AI2/AI4); an org-key principal has no thread of its own.
var ErrAssistantUserScoped = errors.New("identity: assistant threads are user-scoped")

func (h *Handlers) CreateThread(ctx context.Context, req gen.CreateThreadRequestObject) (gen.CreateThreadResponseObject, error) {
	if req.Body == nil || req.Body.Context == nil || req.Body.Context.Org == nil || *req.Body.Context.Org == "" {
		return nil, validationError{fields: []problem.FieldError{{Field: "context.org", Detail: "required"}}}
	}
	orgID := *req.Body.Context.Org
	projectID := ""
	if req.Body.Context.Project != nil {
		projectID = *req.Body.Context.Project
	}

	p, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, ErrNoSession
	}
	// Threads are user-owned (the human drawer/workspace, AI2/AI4). An org-key
	// principal (automation, no UserID) has no thread surface — a clean 403,
	// never a NOT NULL user_id FK 500 (review M3).
	if p.UserID == "" {
		return nil, ErrAssistantUserScoped
	}
	// Membership FIRST, collapsing non-members to 404 — the AI surface must not
	// admit it could exist, and a non-member must not learn the org's AI on/off
	// bit from a 403/404 split or pollute its audit spine (review M1/M2, and
	// GetOrg's own 404-for-non-members convention).
	if _, err := h.svc.q.GetMemberRole(ctx, store.GetMemberRoleParams{OrgID: orgID, UserID: p.UserID}); err != nil {
		return nil, aiDisabled()
	}
	// Enablement gate (closest-wins at the project scope, review H1): disabled
	// → 404 invisible.
	on, err := h.svc.AIAssistantEnabled(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	if !on {
		return nil, aiDisabled()
	}
	// Now authorize ai.use at the EXACT scope the thread targets (review m1: a
	// project-scoped ai.use denial must bind) — no longer policy-narrowed since
	// AI is enabled, so a member with the grant proceeds, without it gets 403.
	if p.Kind == "token" && p.Scope != "full" {
		return nil, ErrScopeDenied
	}
	if err := h.authz.Require(ctx, p, "ai.use", rbac.Scope{OrgID: orgID, ProjectID: projectID}); err != nil {
		return nil, err
	}
	ctxJSON, _ := json.Marshal(req.Body.Context)
	attached := ""
	if req.Body.AttachedInsight != nil {
		attached = *req.Body.AttachedInsight
	}
	thr, err := h.svc.CreateThread(ctx, orgID, p.UserID, "", ctxJSON, attached)
	if err != nil {
		return nil, err
	}
	return gen.CreateThread201JSONResponse(threadToAPI(thr)), nil
}

func (h *Handlers) ListThreads(ctx context.Context, req gen.ListThreadsRequestObject) (gen.ListThreadsResponseObject, error) {
	p, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, ErrNoSession
	}
	// Threads are user-owned; an org-key principal has no thread surface.
	if p.UserID == "" {
		return nil, ErrAssistantUserScoped
	}
	// The user's threads across the orgs they belong to, EXCEPT orgs whose
	// ai-assistant policy is disabled — those are hidden, not deleted (AI Law
	// 4). No org path param exists in the contract, so the union is per-org.
	orgs, err := h.svc.ListOrgsForUser(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	out := []gen.Thread{}
	for _, org := range orgs {
		on, err := h.svc.AIAssistantEnabled(ctx, org.ID, "")
		if err != nil {
			return nil, err
		}
		if !on {
			continue // hidden while disabled — the rows stay in the store
		}
		threads, err := h.svc.ListThreads(ctx, p.UserID, org.ID, 50)
		if err != nil {
			return nil, err
		}
		for _, t := range threads {
			out = append(out, threadToAPI(t))
		}
	}
	return gen.ListThreads200JSONResponse{Data: &out}, nil
}

func threadToAPI(t store.AssistantThread) gen.Thread {
	out := gen.Thread{Id: t.ID, CreatedAt: t.CreatedAt.Time}
	if t.Title != "" {
		out.Title = &t.Title
	}
	if len(t.Context) > 0 {
		var m map[string]any
		if json.Unmarshal(t.Context, &m) == nil {
			out.Context = &m
		}
	}
	if t.AttachedInsight.Valid {
		v := t.AttachedInsight.String
		out.AttachedInsight = &v
	}
	return out
}
