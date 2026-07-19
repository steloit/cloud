package identity

// T13.3 — the /assistant/threads surface. Every read/write is gated by the
// ai-assistant policy (AI Law 4): a `disabled` org's AI surface returns 404
// empty-equivalent (createThread) or is silently omitted (listThreads) — the
// rows persist untouched and reappear the instant the policy is re-enabled.

import (
	"context"
	"encoding/json"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// aiDisabled → 404 empty-equivalent (the AI layer is invisible when off, never
// a 403 that admits it could exist). Routed through the notFound seam.
func aiDisabled() error { return notFoundError{what: "assistant"} }

func (h *Handlers) CreateThread(ctx context.Context, req gen.CreateThreadRequestObject) (gen.CreateThreadResponseObject, error) {
	if req.Body == nil || req.Body.Context == nil || req.Body.Context.Org == nil || *req.Body.Context.Org == "" {
		return nil, validationError{fields: []problem.FieldError{{Field: "context.org", Detail: "required"}}}
	}
	orgID := *req.Body.Context.Org
	// AI-enablement gate FIRST: a disabled org's AI surface is INVISIBLE (404,
	// Law 4), never the 403 the ai.use narrowing would give — the surface must
	// not admit it could exist. Only once enabled do we authorize ai.use (which
	// is no longer policy-narrowed), so a member with the grant proceeds and a
	// member without it gets the honest 403.
	on, err := h.svc.AIAssistantEnabled(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if !on {
		return nil, aiDisabled()
	}
	userID, err := h.requireOrg(ctx, orgID, "ai.use", true)
	if err != nil {
		return nil, err
	}
	ctxJSON, _ := json.Marshal(req.Body.Context)
	attached := ""
	if req.Body.AttachedInsight != nil {
		attached = *req.Body.AttachedInsight
	}
	thr, err := h.svc.CreateThread(ctx, orgID, userID, "", ctxJSON, attached)
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
	// The user's threads across the orgs they belong to, EXCEPT orgs whose
	// ai-assistant policy is disabled — those are hidden, not deleted (AI Law
	// 4). No org path param exists in the contract, so the union is per-org.
	orgs, err := h.svc.ListOrgsForUser(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	var out []gen.Thread
	for _, org := range orgs {
		on, err := h.svc.AIAssistantEnabled(ctx, org.ID)
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
