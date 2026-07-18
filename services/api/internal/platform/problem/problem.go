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
	Reasons           []string     `json:"reasons,omitempty"`             // 409
	RequiredPlan      string       `json:"required_plan,omitempty"`       // 402 plan gate
	OveragePriceCents *int64       `json:"overage_price_cents,omitempty"` // 402 soft quota
	EventID           string       `json:"event_id,omitempty"`            // 5xx
	RetryAfterS       int          `json:"retry_after_s,omitempty"`       // 429
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

// Conflict — 409 with the blocking reasons (downgrade blockers, dependents…).
func Conflict(reasons []string, remediation string) Problem {
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
