package identity

import (
	"context"
	"strings"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// Personal-token operations (T2.2). The reveal-once/hash-stored contract
// (F10/U7): the plaintext appears in exactly one response, ever.

// requireUser returns the authenticated principal; mutating token operations
// additionally require a session or a full-scope token.
func requireUser(ctx context.Context, mutating bool) (session.Principal, error) {
	p, ok := session.PrincipalFrom(ctx)
	if !ok || p.UserID == "" {
		return session.Principal{}, ErrNoSession
	}
	if mutating && p.Kind == "token" && p.Scope != "full" {
		return session.Principal{}, ErrScopeDenied
	}
	return p, nil
}

type notFoundError struct{ what string }

func (e notFoundError) Error() string { return "identity: not found: " + e.what }

func (h *Handlers) CreatePersonalToken(ctx context.Context, req gen.CreatePersonalTokenRequestObject) (gen.CreatePersonalTokenResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || strings.TrimSpace(req.Body.Name) == "" {
		return nil, validationError{fields: []problem.FieldError{{Field: "name", Detail: "required"}}}
	}
	scope := "read_only"
	if req.Body.Scope != nil {
		scope = string(*req.Body.Scope)
	}
	days := 90
	if req.Body.ExpiresInDays != nil {
		days = int(*req.Body.ExpiresInDays)
	}
	minted, err := h.svc.MintPersonalToken(ctx, p.UserID, req.Body.Name, scope, days)
	if err != nil {
		return nil, err
	}
	out := gen.TokenCreated{
		Id:         minted.Row.ID,
		Token:      minted.Secret,
		ShownOnce:  true,
		Prefix:     minted.Row.Prefix,
		HashStored: true,
	}
	if minted.Row.ExpiresAt.Valid {
		out.ExpiresAt = &minted.Row.ExpiresAt.Time
	}
	return gen.CreatePersonalToken201JSONResponse(out), nil
}

func (h *Handlers) ListPersonalTokens(ctx context.Context, _ gen.ListPersonalTokensRequestObject) (gen.ListPersonalTokensResponseObject, error) {
	p, err := requireUser(ctx, false)
	if err != nil {
		return nil, err
	}
	rows, err := h.svc.ListPersonalTokens(ctx, p.UserID)
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
	return gen.ListPersonalTokens200JSONResponse(gen.TokenList{Data: &data}), nil
}

func (h *Handlers) RevokePersonalToken(ctx context.Context, req gen.RevokePersonalTokenRequestObject) (gen.RevokePersonalTokenResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	ok, err := h.svc.RevokePersonalToken(ctx, p.UserID, req.Tok)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFoundError{what: "token " + req.Tok}
	}
	return gen.RevokePersonalToken204Response{}, nil
}
