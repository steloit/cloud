package idempotency

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// PrincipalResolver resolves the caller from a raw request. Idempotency runs
// PRE-strict (it must see the raw body and the raw response bytes), so it
// cannot use the strict middleware's context principal — it resolves the same
// way the SSE path does, through the identity service.
type PrincipalResolver interface {
	PrincipalFromRequest(ctx context.Context, r *http.Request) (session.Principal, bool)
}

// idempotentRoutes is the set of operations this middleware ENFORCES: the
// estimate-gated provisioning path US-3.6 exists to protect.
//
// TestRouteTableMatchesOpenAPI parses openapi.yaml and checks this table
// against the operations that declare `parameters: [idempotencyKey]`, so a new
// declaration cannot silently go unenforced.
var idempotentRoutes = []string{
	"POST /v1/estimates",
	"POST /v1/envs/{env}/services",
}

// deferredRoutes DECLARE the parameter in openapi.yaml but are deliberately not
// enforced here, because replaying their 2xx response would mean STORING A
// CREDENTIAL at rest for the 24h replay window:
//
//   - createWebhook returns the reveal-once signing secret in its 201 body
//     (`WebhookCreated.secret`), which the same handler envelope-encrypts before
//     persisting. Recording the response verbatim would put that plaintext
//     secret in `idempotency_keys.response`, defeating the reveal-once contract.
//   - signup's 201 sets the session cookie through the carrier. Replaying it
//     requires storing a live session credential; NOT replaying it returns a 201
//     with no `Set-Cookie`, leaving the client convinced it is signed in when it
//     holds no session. Both options are wrong, so neither is shipped.
//
// This is a SPEC CONFLICT, recorded rather than silently resolved (US-3.6a):
// the ruling promises dedupe on operations whose responses cannot be stored as
// they stand. Resolving it needs a redaction or credential-pointer design, not
// a wiring change.
//
// Until then the header PASSES THROUGH on these routes rather than being
// refused. Refusing was the first instinct — a client told nothing may believe
// its retry is deduped — but openapi.yaml DECLARES the parameter here, so a
// 422 breaks every spec-conformant client that sets the header globally, and
// breaks signup outright. Passing through is exactly the behaviour these routes
// have today; refusing would be a live regression traded for a warning. The
// divergence belongs in the contract, which is an owner-level change (US-3.6a),
// not in a status code that fails the first request.
var deferredRoutes = []string{
	"POST /v1/auth/signup",
	"POST /v1/orgs/{org}/webhooks",
}

// Middleware enforces the S7 ruling in front of the API.
//
// It is deliberately OUTSIDE the generated strict server: the ruling is about
// bytes ("replay returns the ORIGINAL response"), and only a layer that sees the
// raw request body and the raw response body can honor that literally. A strict
// middleware runs after the body is decoded and returns a typed response object,
// so it could only reconstruct an equivalent response — not replay the original.
//
// Requests without the header, and requests to operations the contract does not
// declare as idempotent, pass through untouched with no buffering.
func Middleware(svc *Service, principals PrincipalResolver) func(http.Handler) http.Handler {
	// A ServeMux used purely as a MATCHER, so route matching is exactly stdlib
	// routing (same precedence, same wildcard semantics) rather than a
	// hand-rolled path comparison that could disagree with the real router.
	matcher := http.NewServeMux()
	for _, pat := range idempotentRoutes {
		matcher.HandleFunc(pat, func(http.ResponseWriter, *http.Request) {})
	}
	deferred := http.NewServeMux()
	for _, pat := range deferredRoutes {
		deferred.HandleFunc(pat, func(http.ResponseWriter, *http.Request) {})
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if _, pattern := matcher.Handler(r); pattern == "" {
				if _, d := deferred.Handler(r); d != "" {
					// DECLARED in the contract but not enforceable yet
					// (US-3.6a). Pass through unchanged — see deferredRoutes.
					next.ServeHTTP(w, r)
					return
				}
				// Not declared at all: refuse rather than ignore, so a client
				// cannot believe a retry is deduped when nothing dedupes it.
				problem.Write(w, r, problem.ValidationFailed([]problem.FieldError{{
					Field:  "Idempotency-Key",
					Detail: "this operation does not accept an idempotency key",
				}}))
				return
			}
			if len(key) > 255 {
				problem.Write(w, r, problem.ValidationFailed([]problem.FieldError{{
					Field: "Idempotency-Key", Detail: "must be at most 255 characters",
				}}))
				return
			}

			// Capped like every other body on this spine (reconcile/http.go):
			// the middleware buffers what it reads, so an uncapped read is a
			// memory amplifier reachable before any handler-level limit.
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
			r.Body.Close()
			if err != nil {
				// 422 rather than a dedicated 413: the problem catalog is
				// owner-level (docs/product/08-api) and inventing a type here
				// would be a silent contract change. The detail names the real
				// cause so the response is not misleading.
				problem.Write(w, r, problem.ValidationFailed([]problem.FieldError{{
					Field: "body", Detail: "could not be read, or exceeds the 1 MiB limit",
				}}))
				return
			}
			// Rewind: the handler chain must still see the body it would have.
			r.Body = io.NopCloser(bytes.NewReader(body))

			principal := principalOf(r, principals)
			// The endpoint key is the CONCRETE path, not the route pattern. With
			// the pattern, one key replayed against a different {env} would
			// return the first env's service — a cross-resource leak. The path
			// makes "same key, different resource" a distinct claim.
			endpoint := endpointKey(r.Method, r.URL.EscapedPath())

			replay, claim, err := svc.Begin(r.Context(), principal, endpoint, key, body)
			if err != nil {
				if err == ErrBodyMismatch {
					problem.Write(w, r, problem.Conflict(
						[]string{"idempotency key already used with a different request body"},
						"Generate a new Idempotency-Key for a different request, or resend the original body to replay the original response."))
					return
				}
				problem.Write(w, r, problem.Internal(problem.NewEventID()))
				return
			}
			if replay != nil {
				if replay.StatusCode == 0 {
					// Claimed but not completed: the first request is still
					// in flight. Executing now would double-provision, so the
					// client is told to retry rather than served a guess.
					problem.Write(w, r, problem.Conflict(
						[]string{"a request with this idempotency key is still in progress"},
						"Retry in a few seconds with the same Idempotency-Key to receive the original response."))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Idempotent-Replay", "true")
				w.WriteHeader(replay.StatusCode)
				_, _ = w.Write(replay.Body)
				return
			}

			rw, rec := newRecorder(w)

			// DEFERRED, so no exit path can skip it. problem.Recover sits
			// ABOVE this middleware, so a panic in a handler unwinds straight
			// past here — a straight-line call after ServeHTTP would leave the
			// claim held, and the client would be refused "still in progress"
			// for the whole abandonment window on a request that definitively
			// failed. On a panic the status is unknown-but-not-success, so it
			// is recorded as 500, which RELEASES the claim.
			returned := false
			defer func() {
				status := rec.status
				if !returned {
					status = http.StatusInternalServerError
				}
				// context.WithoutCancel: r.Context() is cancelled the moment the
				// client's connection closes, which is EXACTLY the scenario S7
				// exists for (a client times out mid-createService and retries).
				// Completing on the request context would abort the write, leave
				// the claim un-recorded, and let the retry re-execute after the
				// abandonment window — a second service and a second billing
				// span. The handler already committed; the record must follow it.
				ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
				defer cancel()
				if err := svc.Complete(ctx, principal, endpoint, key, claim, status, rec.body.Bytes()); err != nil {
					// The response is already written, so this cannot be turned
					// into an error for the client — but it must never be
					// silent: an unrecorded success is a latent duplicate.
					slog.ErrorContext(ctx, "idempotency: recording the response failed; a retry of this key may re-execute",
						"endpoint", endpoint, "status", status, "err", err)
				}
			}()
			next.ServeHTTP(rw, r)
			returned = true
		})
	}
}

// principalOf produces the stable dedupe identity. Distinct prefixes keep the
// namespaces from colliding: a user id can never be read as an org-key id.
func principalOf(r *http.Request, principals PrincipalResolver) string {
	if principals != nil {
		if p, ok := principals.PrincipalFromRequest(r.Context(), r); ok {
			if p.IsOrgKey() {
				return "orgkey:" + p.TokenID
			}
			if p.UserID != "" {
				return "user:" + p.UserID
			}
		}
	}
	// Unauthenticated (signup). Scoping by client IP keeps one caller's key
	// from ever replaying another caller's response, which an "anon" bucket
	// shared across the internet would allow.
	return "anon:" + clientIP(r)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// endpointKey derives the dedupe bucket from a request path, normalizing it
// EXACTLY as ServeMux does: per-SEGMENT unescaping.
//
// Both directions are defects, and each is the mirror of the other:
//
//   - Decoding the whole path (path.Clean on r.URL.Path) COLLAPSES spellings
//     that address different resources — `a%2Fb` and `a%2F%2E%2Fb` become one
//     bucket, replaying one env's service to another.
//   - Using the raw escaped path SPLITS spellings the router treats as
//     identical — `env_1` and `env%5F1` route to the same handler with the same
//     PathValue, so a retry that re-encodes escapes the bucket and the same key
//     executes twice.
//
// Unescaping segment-by-segment matches the router precisely, then each
// segment is CANONICALLY re-encoded — only `%` and `/`, the two characters that
// would otherwise make the joined form ambiguous. So `%5F` and `%2f`-vs-`%2F`
// fold together exactly as the router folds them, while a decoded separator
// stays escaped and remains distinct from a real one.
//
// One dependency this leans on, stated rather than assumed: unclean paths
// (`/v1//estimates`, `/v1/./estimates`) are answered by stdlib's ServeMux with
// a 307 before any handler runs, so they can never produce a recorded 2xx and
// never need to share a bucket with their clean spelling. If the router is ever
// replaced with one that serves unclean paths directly, this function must
// clean as well as unescape.
//
// Re-encoding rather than joining on a sentinel byte: this value is stored in a
// Postgres `text` column, which cannot hold NUL — a NUL separator makes every
// write fail, which is precisely how the first version of this function was
// caught.
func endpointKey(method, escapedPath string) string {
	segs := strings.Split(escapedPath, "/")
	for i, seg := range segs {
		decoded, err := url.PathUnescape(seg)
		if err != nil {
			continue // not valid encoding; keep the spelling as sent
		}
		// Order matters: escape `%` first, or the `/` escape's own `%` is
		// double-encoded on the next pass.
		canonical := strings.ReplaceAll(decoded, "%", "%25")
		canonical = strings.ReplaceAll(canonical, "/", "%2F")
		segs[i] = canonical
	}
	return method + " " + strings.Join(segs, "/")
}

// maxBodyBytes matches the platform's request-body cap.
const maxBodyBytes = 1 << 20

// recorder buffers the response so it can be recorded for replay. It buffers
// ONLY for requests that carry an idempotency key on a declared route, so
// ordinary traffic (including SSE streams) is never held in memory.
type recorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (rec *recorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *recorder) Write(b []byte) (int, error) {
	rec.body.Write(b)
	return rec.ResponseWriter.Write(b)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (rec *recorder) Unwrap() http.ResponseWriter { return rec.ResponseWriter }

func (rec *recorder) Flush() {
	if f, ok := rec.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// hijackRecorder is a recorder over a writer that CAN hijack.
//
// Two types rather than one, because a capability must be advertised only when
// it exists. A single recorder declaring Hijack unconditionally makes
// `w.(http.Hijacker)` succeed even when the writer underneath cannot hijack —
// so a handler that would have taken a graceful fallback branch takes the
// hijack branch and fails at runtime. That is worse than the stripping it was
// meant to fix: stripping loses a capability, lying about one breaks the
// caller's control flow.
type hijackRecorder struct{ *recorder }

func (rec *hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rec.ResponseWriter.(http.Hijacker).Hijack()
}

// newRecorder wraps w, preserving hijackability exactly when w has it.
func newRecorder(w http.ResponseWriter) (http.ResponseWriter, *recorder) {
	rec := &recorder{ResponseWriter: w, status: http.StatusOK}
	if _, ok := w.(http.Hijacker); ok {
		return &hijackRecorder{recorder: rec}, rec
	}
	return rec, rec
}
