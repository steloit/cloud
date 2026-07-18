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
	"github.com/steloit/cloud/services/api/internal/platform/problem"
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
}

func NewHandlers(svc *Service, mgr *session.Manager, authz *Authorizer, reader *events.Reader, envs events.EnvResolver) *Handlers {
	return &Handlers{svc: svc, mgr: mgr, authz: authz, reader: reader, envs: envs}
}

// Mount wires the strict server onto mux under /v1 with the module's
// middleware chain and the problem-catalog error mapping.
func (h *Handlers) Mount(mux *http.ServeMux) {
	strict := gen.NewStrictHandlerWithOptions(h,
		[]gen.StrictMiddlewareFunc{h.contextMiddleware},
		gen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  h.requestError,
			ResponseErrorHandlerFunc: h.responseError,
		})
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
func (h *Handlers) responseError(w http.ResponseWriter, r *http.Request, err error) {
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
	case errors.Is(err, ErrInvalidCredentials):
		problem.Write(w, r, problem.AuthFailed("invalid credentials",
			"Check the email and password and try again."))
	case errors.Is(err, ErrNoSession):
		problem.Write(w, r, problem.AuthFailed("no active session", "Sign in first."))
	case errors.Is(err, ErrScopeDenied):
		problem.Write(w, r, problem.PermissionDenied("read_only token",
			"Use a full-scope token or a browser session for this operation."))
	case errors.As(err, new(AccessDeniedError)):
		var ad AccessDeniedError
		errors.As(err, &ad)
		problem.Write(w, r, problem.PermissionDenied(ad.DeniedBy,
			"Ask an org owner or admin to grant the missing role, or adjust the denying policy."))
	case errors.As(err, new(notFoundError)):
		var nf notFoundError
		errors.As(err, &nf)
		problem.Write(w, r, problem.NotFound(nf.what))
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
	est, err := h.svc.Login(ctx, string(req.Body.Email), req.Body.Password, meta.Device, rateKey)
	if err != nil {
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
