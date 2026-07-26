package identity

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/notify"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
	"github.com/steloit/cloud/services/api/internal/subscription"
)

// Handlers implements the generated strict interface for the operations this
// module owns (oapi-server.cfg.yaml include-operation-ids — the list and this
// type grow together; nothing generated goes unimplemented).
type Handlers struct {
	svc    *Service
	mgr    *session.Manager
	authz  *Authorizer
	reader *events.Reader
	envs   events.EnvResolver
	usage  usageRoller
	notify *notify.Router
	subs   *subscription.Service
}

func NewHandlers(svc *Service, mgr *session.Manager, authz *Authorizer, reader *events.Reader, envs events.EnvResolver, usage usageRoller, router *notify.Router, subs *subscription.Service) *Handlers {
	return &Handlers{svc: svc, mgr: mgr, authz: authz, reader: reader, envs: envs, usage: usage, notify: router, subs: subs}
}

// Mount wires the strict server onto mux under /v1 with the module's
// middleware chain and the problem-catalog error mapping. ssi is the COMPOSED
// server (identity + other modules' handler sets embedded in one struct by
// the composition root); pass h itself when identity is the whole surface.
func (h *Handlers) Mount(mux *http.ServeMux, ssi gen.StrictServerInterface) {
	strict := gen.NewStrictHandlerWithOptions(ssi,
		[]gen.StrictMiddlewareFunc{h.contextMiddleware},
		gen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  h.requestError,
			ResponseErrorHandlerFunc: h.responseError,
		})
	// testWebhook is registered pre-strict: its contract path glues a `:verb`
	// custom-method onto a wildcard, which the stdlib mux can't parse.
	h.MountWebhookTest(mux)
	// CSV billing exports stream a raw body — pre-strict, not typed JSON (T11.6).
	h.MountBillingExports(mux)
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseURL:    "/v1",
		BaseRouter: mux,
	})
}

// contextMiddleware injects the cookie carrier + request meta, resolves the
// session principal when the cookie is present, and applies handler-set
// cookies to the ResponseWriter before the response is visited.
func (h *Handlers) contextMiddleware(f gen.StrictHandlerFunc, _ string) gen.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		ctx, carrier := session.WithCarrier(ctx)
		ctx = session.WithMeta(ctx, session.Meta{IP: clientIP(r), Device: deviceOf(r)})
		if p, ok := h.svc.PrincipalFromRequest(ctx, r); ok {
			ctx = session.WithPrincipal(ctx, p)
		}
		resp, err := f(ctx, w, r, request)
		carrier.Apply(w)
		return resp, err
	}
}

// PrincipalFromRequest resolves the request's credentials — session cookie or
// bearer token (both kinds share one hash lookup). Shared by the strict
// middleware and the pre-strict SSE path (events.Streamer).
func (s *Service) PrincipalFromRequest(ctx context.Context, r *http.Request) (session.Principal, bool) {
	if ck, err := r.Cookie(session.CookieName); err == nil && ck.Value != "" {
		if sess, u, err := s.Resolve(ctx, ck.Value); err == nil {
			return session.Principal{
				Kind: "session", UserID: u.ID, SessionID: sess.ID, Device: sess.Device,
				CreatedAt: sess.CreatedAt.Time, LastSeenAt: sess.LastSeenAt.Time,
			}, true
		}
		return session.Principal{}, false
	}
	if ah := r.Header.Get("Authorization"); strings.HasPrefix(ah, "Bearer ") {
		if p, err := s.ResolveBearer(ctx, strings.TrimPrefix(ah, "Bearer ")); err == nil {
			return p, true
		}
	}
	return session.Principal{}, false
}

// requestError: malformed request bodies → 422 (catalog validation_failed).
func (h *Handlers) requestError(w http.ResponseWriter, r *http.Request, err error) {
	problem.Write(w, r, problem.ValidationFailed([]problem.FieldError{{Field: "body", Detail: err.Error()}}))
}

// responseError maps typed domain errors onto the closed problem catalog
// (auth_failed 401 ratified via S-process 2026-07-19, closing the T2.1 finding).
// Other modules' errors arrive here too (one strict server, one error seam):
// any error implementing problem.Carrier writes its own catalog problem.
func (h *Handlers) responseError(w http.ResponseWriter, r *http.Request, err error) {
	var carrier problem.Carrier
	if errors.As(err, &carrier) {
		problem.Write(w, r, carrier.Problem())
		return
	}
	var weak WeakPasswordError
	var limited RateLimitedError
	switch {
	case errors.As(err, &weak):
		problem.Write(w, r, problem.ValidationFailed([]problem.FieldError{{Field: "password", Detail: weak.Detail}}))
	case errors.As(err, &limited):
		problem.Write(w, r, problem.RateLimited(limited.RetryAfterS))
	case errors.Is(err, ErrEmailTaken):
		problem.Write(w, r, problem.Conflict([]string{"email already registered"},
			"Sign in with this email instead, or use password reset when it ships (T7.2)."))
	case errors.Is(err, ErrSlugTaken):
		problem.Write(w, r, problem.Conflict([]string{"organization slug already taken"},
			"Pick a different name — the slug is derived from it and is immutable."))
	case errors.Is(err, ErrLastOwner):
		problem.Write(w, r, problem.Conflict([]string{"last owner cannot be demoted or removed"},
			"Promote another member to owner first (F1: an organization keeps at least one owner)."))
	case errors.Is(err, ErrOrgDeleting):
		problem.Write(w, r, problem.Conflict([]string{"deletion already scheduled"},
			"The organization is already scheduled for deletion; state is kept per the 90-day rule."))
	case errors.Is(err, ErrAlreadyMember):
		problem.Write(w, r, problem.Conflict([]string{"already a member"},
			"No invite needed — this person already has access."))
	case errors.Is(err, ErrAlreadyInvited):
		problem.Write(w, r, problem.Conflict([]string{"already invited"},
			"A pending invitation exists for this email; revoke it first to change the role."))
	case errors.Is(err, ErrInviteGone):
		problem.Write(w, r, problem.Conflict([]string{"invite expired, used or revoked"},
			"If you accepted earlier — even on another device — you're a member; just sign in."))
	case errors.Is(err, ErrInviteNotExpired):
		problem.Write(w, r, problem.Conflict([]string{"invite is not expired"},
			"The original link still works — renewal is only for expired invitations."))
	case errors.As(err, new(SeatOverageError)):
		var so SeatOverageError
		errors.As(err, &so)
		problem.Write(w, r, problem.QuotaSoft(so.PriceCents))
	case errors.Is(err, ErrInvalidCredentials):
		problem.Write(w, r, problem.AuthFailed("invalid credentials",
			"Check the email and password and try again."))
	case errors.Is(err, ErrMFACodeInvalid) || errors.Is(err, ErrMFANotEnrolled):
		problem.Write(w, r, problem.AuthFailed("MFA code invalid",
			"Enter the current code from your authenticator, or a recovery code."))
	case errors.Is(err, ErrNoSession):
		problem.Write(w, r, problem.AuthFailed("no active session", "Sign in first."))
	case errors.Is(err, ErrAccountDeleting):
		problem.Write(w, r, problem.AuthFailed("account scheduled for deletion",
			"This account is being deleted and can no longer sign in."))
	case errors.Is(err, ErrScopeDenied):
		problem.Write(w, r, problem.PermissionDenied("read_only token",
			"Use a full-scope token or a browser session for this operation."))
	case errors.Is(err, ErrAssistantUserScoped):
		problem.Write(w, r, problem.PermissionDenied("org-key principal",
			"The assistant is a user surface — sign in as a user (browser session or personal token)."))
	case errors.As(err, new(AccessDeniedError)):
		var ad AccessDeniedError
		errors.As(err, &ad)
		problem.Write(w, r, problem.PermissionDenied(ad.DeniedBy,
			"Ask an org owner or admin to grant the missing role, or adjust the denying policy."))
	case errors.As(err, new(notFoundError)):
		var nf notFoundError
		errors.As(err, &nf)
		problem.Write(w, r, problem.NotFound(nf.what))
	case errors.Is(err, subscription.ErrNoSubscription):
		problem.Write(w, r, problem.NotFound("subscription"))
	case errors.Is(err, subscription.ErrBadTransition):
		problem.Write(w, r, problem.Conflict([]string{"the subscription is not in a state that allows this change"},
			"Check the current subscription status; a cancelled subscription owns its anchor until it completes."))
	case errors.Is(err, subscription.ErrConcurrentModification):
		problem.Write(w, r, problem.Conflict([]string{"the subscription changed while this request was in flight"},
			"Re-read the subscription and retry."))
	case errors.As(err, new(validationError)):
		var ve validationError
		errors.As(err, &ve)
		problem.Write(w, r, problem.ValidationFailed(ve.fields))
	default:
		id := problem.NewEventID()
		problem.Write(w, r, problem.Internal(id))
	}
}

type validationError struct{ fields []problem.FieldError }

func (v validationError) Error() string { return "identity: validation failed" }

// ---- operations ------------------------------------------------------------

func (h *Handlers) Signup(ctx context.Context, req gen.SignupRequestObject) (gen.SignupResponseObject, error) {
	if req.Body == nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "body", Detail: "required"}}}
	}
	var fields []problem.FieldError
	email := string(req.Body.Email)
	if err := ValidateEmail(email); err != nil {
		fields = append(fields, problem.FieldError{Field: "email", Detail: err.Error()})
	}
	if strings.TrimSpace(req.Body.Name) == "" {
		fields = append(fields, problem.FieldError{Field: "name", Detail: "required"})
	}
	if len(fields) > 0 {
		return nil, validationError{fields: fields}
	}
	meta := session.MetaFrom(ctx)
	est, err := h.svc.Signup(ctx, email, req.Body.Password, req.Body.Name, meta.Device)
	if err != nil {
		return nil, err
	}
	h.setSessionCookie(ctx, est)
	return gen.Signup201JSONResponse(h.sessionInfo(est.User, est.Session, true)), nil
}

func (h *Handlers) Login(ctx context.Context, req gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	if req.Body == nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "body", Detail: "required"}}}
	}
	meta := session.MetaFrom(ctx)
	rateKey := meta.IP + "|" + strings.ToLower(strings.TrimSpace(string(req.Body.Email)))
	mfaCode := ""
	if req.Body.MfaCode != nil {
		mfaCode = *req.Body.MfaCode
	}
	est, err := h.svc.Login(ctx, string(req.Body.Email), req.Body.Password, meta.Device, rateKey, mfaCode)
	if err != nil {
		// Password correct but a second factor is needed → mfa_required (a
		// state, not an error): the client re-submits with mfa_code.
		if errors.Is(err, ErrMFACodeRequired) {
			methods := []gen.LoginResultMfaMethods{"totp", "recovery"}
			return gen.Login200JSONResponse(gen.LoginResult{
				Status:     gen.LoginResultStatus("mfa_required"),
				MfaMethods: &methods,
			}), nil
		}
		return nil, err
	}
	h.setSessionCookie(ctx, est)
	info := h.sessionInfo(est.User, est.Session, true)
	return gen.Login200JSONResponse(gen.LoginResult{
		Status:  gen.LoginResultStatus("session"),
		Session: &info,
	}), nil
}

func (h *Handlers) Logout(ctx context.Context, _ gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	p, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, ErrNoSession
	}
	if err := h.svc.Logout(ctx, p.SessionID); err != nil {
		return nil, err
	}
	if c := session.CarrierFrom(ctx); c != nil {
		c.Add(h.mgr.ClearCookie())
	}
	return gen.Logout204Response{}, nil
}

func (h *Handlers) GetSession(ctx context.Context, _ gen.GetSessionRequestObject) (gen.GetSessionResponseObject, error) {
	p, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, ErrNoSession
	}
	u, err := h.svc.UserByID(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	info := gen.SessionInfo{}
	info.User.Id = u.ID
	info.User.Email = u.Email
	if u.Name != "" {
		info.User.Name = &u.Name
	}
	info.Session.Id = p.SessionID
	info.Session.CreatedAt = p.CreatedAt
	cur := true
	info.Session.Current = &cur
	if p.Device != "" {
		info.Session.Device = &p.Device
	}
	ls := p.LastSeenAt
	info.Session.LastSeenAt = &ls
	return gen.GetSession200JSONResponse(info), nil
}

// ---- helpers ---------------------------------------------------------------

func (h *Handlers) setSessionCookie(ctx context.Context, est Established) {
	if c := session.CarrierFrom(ctx); c != nil {
		c.Add(h.mgr.Cookie(est.RawToken, est.Session.ExpiresAt.Time))
	}
}

func (h *Handlers) sessionInfo(u store.User, s store.Session, current bool) gen.SessionInfo {
	info := gen.SessionInfo{}
	info.User.Id = u.ID
	info.User.Email = u.Email
	if u.Name != "" {
		info.User.Name = &u.Name
	}
	info.Session.Id = s.ID
	info.Session.CreatedAt = s.CreatedAt.Time
	info.Session.Current = &current
	if s.Device != "" {
		info.Session.Device = &s.Device
	}
	ls := s.LastSeenAt.Time
	info.Session.LastSeenAt = &ls
	return info
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func deviceOf(r *http.Request) string {
	ua := r.UserAgent()
	if len(ua) > 120 {
		ua = ua[:120]
	}
	return ua
}
