package problem

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// every constructor, one row each — the closed x-error-catalog set.
func catalog() map[string]Problem {
	return map[string]Problem{
		"validation_failed": ValidationFailed([]FieldError{{Field: "name", Detail: "required"}}),
		"permission_denied": PermissionDenied("missing role: Admin", ""),
		"auth_failed":       AuthFailed("invalid credentials", ""),
		"plan_gated":        PlanGated("business"),
		"quota_soft":        QuotaSoft(162),
		"quota_hard":        QuotaHard("preview quota reached", ""),
		"conflict":          Conflict([]string{"2 services exist"}, ""),
		"not_found":         NotFound("svc_x"),
		"rate_limited":      RateLimited(30),
		"provider_error":    ProviderError("evt_upstream1"),
		"internal":          Internal("evt_boom1"),
	}
}

// AC: remediation is unforgeable — every constructor output carries it.
func TestEveryConstructorHasRemediation(t *testing.T) {
	for name, p := range catalog() {
		if strings.TrimSpace(p.Remediation) == "" {
			t.Errorf("%s: empty remediation", name)
		}
		if p.Status < 400 {
			t.Errorf("%s: non-error status %d", name, p.Status)
		}
	}
}

// AC: golden shapes per catalog entry.
func TestGoldenShapes(t *testing.T) {
	cases := []struct {
		name   string
		p      Problem
		status int
		check  func(t *testing.T, m map[string]any)
	}{
		{"422 errors[]", ValidationFailed([]FieldError{{Field: "email", Detail: "invalid"}}), 422, func(t *testing.T, m map[string]any) {
			errs := m["errors"].([]any)
			if len(errs) != 1 || errs[0].(map[string]any)["field"] != "email" {
				t.Fatalf("errors[] wrong: %v", m["errors"])
			}
		}},
		{"409 reasons[]", Conflict([]string{"downgrade blocked: 12 seats in use"}, ""), 409, func(t *testing.T, m map[string]any) {
			if len(m["reasons"].([]any)) != 1 {
				t.Fatalf("reasons[] wrong")
			}
		}},
		{"402 plan", PlanGated("business"), 402, func(t *testing.T, m map[string]any) {
			if m["required_plan"] != "business" {
				t.Fatalf("required_plan wrong")
			}
		}},
		{"402 soft quota", QuotaSoft(162), 402, func(t *testing.T, m map[string]any) {
			if m["overage_price_cents"].(float64) != 162 {
				t.Fatalf("overage_price_cents wrong")
			}
		}},
		{"5xx event_id", Internal("evt_x"), 500, func(t *testing.T, m map[string]any) {
			if m["event_id"] != "evt_x" {
				t.Fatalf("event_id wrong")
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/x", nil)
			Write(w, r, tc.p)
			if w.Code != tc.status {
				t.Fatalf("status %d want %d", w.Code, tc.status)
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
				t.Fatalf("content-type %q", ct)
			}
			var m map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(m["remediation"].(string)) == "" {
				t.Fatal("remediation missing on the wire")
			}
			tc.check(t, m)
		})
	}
}

// AC: 429 sets BOTH the header and the field.
func TestRetryAfterHeaderAndField(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	Write(w, r, RateLimited(30))
	if w.Header().Get("Retry-After") != "30" {
		t.Fatalf("Retry-After header missing")
	}
	if strings.Contains(w.Body.String(), "retry_after_s") {
		t.Fatalf("retry_after_s must never be in the body (header-only contract): %s", w.Body.String())
	}
}

// AC: panic -> 500 problem with event_id.
func TestRecoverMiddleware(t *testing.T) {
	h := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != 500 {
		t.Fatalf("status %d", w.Code)
	}
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	if !strings.HasPrefix(m["event_id"].(string), "evt_") {
		t.Fatalf("event_id missing: %v", m)
	}
}
