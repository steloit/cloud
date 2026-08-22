// Package problem is the ONE place API errors are born (architecture §16).
// Every error is RFC 9457 problem+json and carries remediation by
// construction — there is no exported path to a Problem without it. The
// constructor set mirrors openapi.yaml's x-error-catalog exactly; a new
// error class is an S-process contract change, never a local addition.
package problem

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// FieldError renders under errors[] on 422 (client shows it inline under the field).
type FieldError struct {
	Field  string `json:"field"`
	Detail string `json:"detail"`
}

// Problem is an RFC 9457 body with the Steloit extension fields. Fields are
// set only by the catalog constructors below; Remediation is always present.
type Problem struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Status      int    `json:"status"`
	Detail      string `json:"detail,omitempty"`
	Remediation string `json:"remediation"`

	Errors            []FieldError `json:"errors,omitempty"`              // 422
	Reasons           []Reason     `json:"reasons,omitempty"`             // 409: ALL blockers, contract object shape
	RequiredPlan      string       `json:"required_plan,omitempty"`       // 402 plan gate
	OveragePriceCents *int64       `json:"overage_price_cents,omitempty"` // 402 soft quota
	EventID           string       `json:"event_id,omitempty"`            // 5xx
	RetryAfterS       int          `json:"-"`                             // 429: the Retry-After HEADER is the contract channel, never the body
}

const typeBase = "https://api.steloit.app/problems/"

// ValidationFailed — 422 with field errors; the field list IS the remediation surface.
func ValidationFailed(errs []FieldError) Problem {
	return Problem{
		Type: typeBase + "validation_failed", Title: "Validation failed", Status: http.StatusUnprocessableEntity,
		Errors:      errs,
		Remediation: "Fix the listed fields and retry the request.",
	}
}

// AuthFailed — 401 (catalog auth_failed, S-process 2026-07-19): invalid
// credentials or missing/expired session. Never discloses account existence.
func AuthFailed(detail, remediation string) Problem {
	return Problem{
		Type: typeBase + "auth_failed", Title: "Authentication failed", Status: http.StatusUnauthorized,
		Detail: detail, Remediation: nonEmpty(remediation, "Sign in and try again."),
	}
}

// PermissionDenied — 403; detail must name the missing role OR the denying policy (E3 grammar).
func PermissionDenied(detail, remediation string) Problem {
	return Problem{
		Type: typeBase + "permission_denied", Title: "Permission denied", Status: http.StatusForbidden,
		Detail: detail, Remediation: nonEmpty(remediation, "Ask an org admin for the required role, or review the denying policy."),
	}
}

// PlanGated — 402 carrying the plan that unlocks the capability (B6 lock grammar).
func PlanGated(requiredPlan string) Problem {
	return Problem{
		Type: typeBase + "plan_gated", Title: "Plan required", Status: http.StatusPaymentRequired,
		RequiredPlan: requiredPlan,
		Remediation:  fmt.Sprintf("Upgrade to the %s plan to use this capability.", requiredPlan),
	}
}

// QuotaSoft — 402: soft quota proceeds only with confirm=true; the price is printed, never discovered.
func QuotaSoft(overagePriceCents int64) Problem {
	return Problem{
		Type: typeBase + "quota_exceeded", Title: "Included quota exceeded", Status: http.StatusPaymentRequired,
		OveragePriceCents: &overagePriceCents,
		Remediation:       "Retry with confirm=true to proceed at the shown overage price, or raise the quota.",
	}
}

// QuotaHard — 402: hard quota fails loudly with a way forward.
func QuotaHard(detail, remediation string) Problem {
	return Problem{
		Type: typeBase + "quota_exceeded", Title: "Quota exceeded", Status: http.StatusPaymentRequired,
		Detail: detail, Remediation: nonEmpty(remediation, "Raise the quota or reduce usage, then retry."),
	}
}

// Carrier lets module-local error types carry their own catalog problem to
// the one strict-server error seam (identity.responseError checks it first).
type Carrier interface {
	error
	Problem() Problem
}

// Reason is one 409 blocker — the contract's object shape (Problem schema:
// {code, message, remediation}); conformance fix 2026-07-19, closing the
// Phase-2 review finding (server emitted bare strings).
type Reason struct {
	Code        string `json:"code,omitempty"`
	Message     string `json:"message,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// Conflict — 409 with the blocking reasons (downgrade blockers, dependents…).
// Callers pass messages; richer blockers use ConflictReasons.
func Conflict(reasons []string, remediation string) Problem {
	structured := make([]Reason, 0, len(reasons))
	for _, r := range reasons {
		structured = append(structured, Reason{Message: r})
	}
	return ConflictReasons(structured, remediation)
}

// ConflictReasons — 409 with fully structured blockers.
func ConflictReasons(reasons []Reason, remediation string) Problem {
	return Problem{
		Type: typeBase + "conflict", Title: "Conflict", Status: http.StatusConflict,
		Reasons: reasons, Remediation: nonEmpty(remediation, "Resolve the listed blockers and retry."),
	}
}

// NotFound — 404.
func NotFound(what string) Problem {
	return Problem{
		Type: typeBase + "not_found", Title: "Not found", Status: http.StatusNotFound,
		Detail:      fmt.Sprintf("%s was not found.", what),
		Remediation: "Check the identifier; list the parent collection to find the current one.",
	}
}

// RateLimited — 429; Write sets the Retry-After header from RetryAfterS.
func RateLimited(retryAfterS int) Problem {
	return Problem{
		Type: typeBase + "rate_limited", Title: "Rate limited", Status: http.StatusTooManyRequests,
		RetryAfterS: retryAfterS,
		Remediation: fmt.Sprintf("Wait %d seconds and retry; honor the Retry-After header.", retryAfterS),
	}
}

// ProviderError — 502 with the incident-grade event id in detail (catalog note).
func ProviderError(eventID string) Problem {
	return Problem{
		Type: typeBase + "provider_error", Title: "Upstream provider error", Status: http.StatusBadGateway,
		Detail: "An upstream provider failed; reference event " + eventID + ".", EventID: eventID,
		Remediation: "Retry shortly; if it persists, contact support with the event id.",
	}
}

// Internal — 500; the body carries the event id, the LOG carries the truth (§16).
func Internal(eventID string) Problem {
	return Problem{
		Type: typeBase + "internal", Title: "Internal error", Status: http.StatusInternalServerError,
		EventID:     eventID,
		Remediation: "Retry; if it persists, contact support with the event id.",
	}
}

// NewEventID mints the id that ties a 5xx body to its single boundary log line.
func NewEventID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "evt_" + hex.EncodeToString(b)
}

// Write is terminal: one response, correct media type, headers from the
// problem, and — for 5xx — the single boundary log line (§16: log once).
func Write(w http.ResponseWriter, r *http.Request, p Problem) {
	if p.RetryAfterS > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", p.RetryAfterS))
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
	if p.Status >= 500 {
		slog.ErrorContext(r.Context(), "request failed",
			"event_id", p.EventID, "status", p.Status, "type", p.Type,
			"method", r.Method, "path", r.URL.Path)
	}
}

// Recover converts panics into 500 problems with a fresh event id; the panic
// value is logged once, here, with that id.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				id := NewEventID()
				slog.ErrorContext(r.Context(), "panic recovered",
					"event_id", id, "panic", fmt.Sprint(rec),
					"method", r.Method, "path", r.URL.Path)
				Write(w, r, Internal(id))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func nonEmpty(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

// deniedNoStanding is satisfied by an authorization error that can classify
// itself. The method name is deliberately distinctive so nothing satisfies it by
// accident, and the interface lives HERE rather than in the authorization
// package so that every enforcement point can reach the mapping without
// importing it — `events.Streamer` cannot import `identity`, because `identity`
// imports `events`.
type deniedNoStanding interface {
	AccessDeniedNoStanding() bool
}

// FromDenial maps an authorization failure to the response it must produce.
//
// This is the single ENFORCEMENT mapping for the whole API: one decision (made
// where the denial is constructed, by AccessDeniedNoStanding) turned into one
// response, in one place. The rule it implements is the repo's stated
// convention and the industry norm — GitHub answers 404 for a private
// repository you cannot see, because a 403 would confirm it exists:
//
//   - NO STANDING (not a member; a key scoped elsewhere) → 404, indistinguishable
//     from an id that does not exist. Anything else is an existence oracle.
//   - HAS STANDING but lacks the permission → an honest 403 that names what
//     denied it, so the remediation is reachable.
//
// It returns ok=false for an error that is not an authorization denial, so
// callers pass those through unchanged rather than mapping them by accident.
//
// WHY THIS EXISTS. The classification and this mapping were previously written
// out at each call site. That produced exactly the defect the pattern predicts:
// the conversion was added to one handler and not to the second transport of the
// SAME operation, so `Accept: text/event-stream` answered 403 where the JSON
// half answered 404 — one request header reopening the oracle the fix had just
// closed. A rule enforced by repetition is a rule enforced unevenly.
func FromDenial(err error, resource, remediation string) (Problem, bool) {
	var d deniedNoStanding
	if !errorsAs(err, &d) {
		return Problem{}, false
	}
	if d.AccessDeniedNoStanding() {
		return NotFound(resource), true
	}
	return PermissionDenied(err.Error(), remediation), true
}

// errorsAs is errors.As, wrapped so this file states its one dependency on the
// errors package explicitly rather than importing it for a single call.
func errorsAs(err error, target any) bool { return errors.As(err, target) }
