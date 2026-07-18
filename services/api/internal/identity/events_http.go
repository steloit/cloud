package identity

// T2.5: the JSON views over the events spine. The SSE variant of listEvents
// is served pre-strict by events.Streamer.Intercept (strict handlers buffer).

import (
	"context"
	"errors"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

func (h *Handlers) principalOrDeny(ctx context.Context) (session.Principal, error) {
	p, ok := session.PrincipalFrom(ctx)
	if !ok {
		return session.Principal{}, ErrNoSession
	}
	return p, nil
}

// ListAuditEvents — /orgs/{org}/audit (W12 compliance ledger; audit.read).
func (h *Handlers) ListAuditEvents(ctx context.Context, req gen.ListAuditEventsRequestObject) (gen.ListAuditEventsResponseObject, error) {
	p, err := h.principalOrDeny(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.authz.Require(ctx, p, "audit.read", rbac.Scope{OrgID: req.OrgPathParam}); err != nil {
		return nil, err
	}
	cursor := ""
	if req.Params.Cursor != nil {
		cursor = *req.Params.Cursor
	}
	page, err := h.reader.Page(ctx, req.OrgPathParam,
		events.Filters{Actor: req.Params.Actor, Action: req.Params.Action}, cursor)
	if err != nil {
		if errors.Is(err, events.ErrBadCursor) {
			return nil, validationError{fields: []problem.FieldError{{Field: "cursor", Detail: "malformed"}}}
		}
		return nil, err
	}
	return gen.ListAuditEvents200JSONResponse(page), nil
}

// ListEvents — /envs/{env}/events, JSON variant (observe.read). Env→org goes
// through the events.EnvResolver seam; real environments arrive with T3.2.
func (h *Handlers) ListEvents(ctx context.Context, req gen.ListEventsRequestObject) (gen.ListEventsResponseObject, error) {
	p, err := h.principalOrDeny(ctx)
	if err != nil {
		return nil, err
	}
	orgID, err := h.envs.OrgForEnv(ctx, req.EnvPathParam)
	if err != nil {
		if errors.Is(err, events.ErrEnvNotFound) {
			return nil, notFoundError{what: "environment"}
		}
		return nil, err
	}
	if err := h.authz.Require(ctx, p, "observe.read", rbac.Scope{OrgID: orgID, EnvID: req.EnvPathParam}); err != nil {
		return nil, err
	}
	cursor := ""
	if req.Params.Cursor != nil {
		cursor = *req.Params.Cursor
	}
	page, err := h.reader.Page(ctx, orgID, events.Filters{Kind: req.Params.Kind}, cursor)
	if err != nil {
		if errors.Is(err, events.ErrBadCursor) {
			return nil, validationError{fields: []problem.FieldError{{Field: "cursor", Detail: "malformed"}}}
		}
		return nil, err
	}
	return gen.ListEvents200JSONResponse(page), nil
}
