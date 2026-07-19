package identity

// T7.2 password-reset handlers. Both are unauthenticated (security: []).
// request always returns 202 (no account disclosure); reset returns 204 and
// revokes every session.

import (
	"context"
	"errors"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

func (h *Handlers) RequestPasswordReset(ctx context.Context, req gen.RequestPasswordResetRequestObject) (gen.RequestPasswordResetResponseObject, error) {
	// Always 202 — a valid or unknown email is indistinguishable to the caller
	// (no account enumeration). A real error (infra) still surfaces as 500.
	if req.Body != nil && string(req.Body.Email) != "" {
		if err := h.svc.RequestPasswordReset(ctx, string(req.Body.Email)); err != nil {
			return nil, err
		}
	}
	return gen.RequestPasswordReset202Response{}, nil
}

func (h *Handlers) ResetPassword(ctx context.Context, req gen.ResetPasswordRequestObject) (gen.ResetPasswordResponseObject, error) {
	if req.Body == nil || req.Body.Token == "" || req.Body.Password == "" {
		return nil, validationError{fields: []problem.FieldError{{Field: "token", Detail: "token and password are required"}}}
	}
	if err := h.svc.ResetPassword(ctx, req.Body.Token, req.Body.Password); err != nil {
		if errors.Is(err, ErrInvalidResetToken) {
			// don't reveal which of unknown/used/expired — one message.
			return nil, validationError{fields: []problem.FieldError{{Field: "token",
				Detail: "this reset link is invalid or has expired — request a new one"}}}
		}
		return nil, err // WeakPasswordError etc. map through the seam
	}
	return gen.ResetPassword204Response{}, nil
}
