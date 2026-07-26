package identity

// T7.1 MFA handlers. All require a live session (the user acting on their own
// account); MFA is never plan-gated.

import (
	"context"
	"errors"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

func (h *Handlers) TotpEnroll(ctx context.Context, _ gen.TotpEnrollRequestObject) (gen.TotpEnrollResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	u, err := h.svc.UserByID(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	secret, uri, err := h.svc.EnrollTOTP(ctx, p.UserID, u.Email)
	if err != nil {
		return nil, err
	}
	return gen.TotpEnroll200JSONResponse{Secret: &secret, OtpauthUri: &uri}, nil
}

func (h *Handlers) TotpVerify(ctx context.Context, req gen.TotpVerifyRequestObject) (gen.TotpVerifyResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.Code == "" {
		return nil, validationError{fields: []problem.FieldError{{Field: "code", Detail: "required"}}}
	}
	if _, err := h.svc.VerifyTOTPEnrollment(ctx, p.UserID, req.Body.Code); err != nil {
		return nil, err
	}
	// 204 per contract; the recovery codes are surfaced via the regenerate op
	// (a client that wants them at enroll time calls regenerate next — kept
	// separate so the 204 stays a clean confirmation). Finding recorded.
	return gen.TotpVerify204Response{}, nil
}

func (h *Handlers) RegenerateRecoveryCodes(ctx context.Context, _ gen.RegenerateRecoveryCodesRequestObject) (gen.RegenerateRecoveryCodesResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	codes, err := h.svc.RegenerateRecovery(ctx, p.UserID)
	if err != nil {
		if errors.Is(err, ErrMFANotEnrolled) {
			return nil, validationError{fields: []problem.FieldError{{Field: "mfa", Detail: "enroll TOTP before generating recovery codes"}}}
		}
		return nil, err
	}
	return gen.RegenerateRecoveryCodes200JSONResponse{Codes: &codes}, nil
}

func (h *Handlers) DisableMfa(ctx context.Context, _ gen.DisableMfaRequestObject) (gen.DisableMfaResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DisableMFA(ctx, p.UserID); err != nil {
		return nil, err
	}
	return gen.DisableMfa204Response{}, nil
}
