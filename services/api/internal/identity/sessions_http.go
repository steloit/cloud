package identity

// T7.3: the P-series security page — session list + revoke. Revocation is
// IMMEDIATE: the very next request with the revoked cookie fails (sessions
// resolve per request; nothing is cached).

import (
	"context"
	"time"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

func (h *Handlers) ListSessions(ctx context.Context, _ gen.ListSessionsRequestObject) (gen.ListSessionsResponseObject, error) {
	p, ok := session.PrincipalFrom(ctx)
	if !ok || p.Kind != "session" {
		// the security page is a signed-in-human surface; tokens don't have
		// a session list of their own to show
		if !ok {
			return nil, ErrNoSession
		}
	}
	if p.UserID == "" {
		return nil, ErrNoSession
	}
	rows, err := h.svc.q.ListActiveSessionsForUser(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	type item = struct {
		CreatedAt  *time.Time `json:"created_at,omitempty"`
		Current    *bool      `json:"current,omitempty"`
		Device     *string    `json:"device,omitempty"`
		Id         *string    `json:"id,omitempty"`
		LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	}
	data := make([]item, 0, len(rows))
	for _, s := range rows {
		it := item{}
		id := s.ID
		it.Id = &id
		cur := s.ID == p.SessionID
		it.Current = &cur
		if s.Device != "" {
			d := s.Device
			it.Device = &d
		}
		created := s.CreatedAt.Time
		it.CreatedAt = &created
		seen := s.LastSeenAt.Time
		it.LastSeenAt = &seen
		data = append(data, it)
	}
	return gen.ListSessions200JSONResponse(gen.SessionList{Data: &data}), nil
}

func (h *Handlers) RevokeSession(ctx context.Context, req gen.RevokeSessionRequestObject) (gen.RevokeSessionResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	n, err := h.svc.q.RevokeSessionOwned(ctx, store.RevokeSessionOwnedParams{ID: req.Ses, UserID: p.UserID})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, notFoundError{what: "session " + req.Ses}
	}
	return gen.RevokeSession204Response{}, nil
}
