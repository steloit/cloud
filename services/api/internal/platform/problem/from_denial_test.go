package problem

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// FromDenial was extracted to this package precisely so the denial→response rule
// is decided ONCE. It had no test in its own package — only transitive coverage
// through identity's integration tests — which left the half that matters most
// untested: making it classify every error as a denial survived the whole suite.
type stubDenial struct {
	noStanding bool
	reason     string
}

func (s stubDenial) Error() string                { return "identity: access denied: " + s.reason }
func (s stubDenial) AccessDeniedNoStanding() bool { return s.noStanding }
func (s stubDenial) AccessDeniedReason() string   { return s.reason }

func TestFromDenialMapsNoStandingTo404AndLackedPermissionTo403(t *testing.T) {
	got, ok := FromDenial(stubDenial{noStanding: true, reason: "role:billing"}, "environment", "Ask an owner.")
	if !ok {
		t.Fatal("a denial was not classified as one")
	}
	if got.Status != 404 {
		t.Fatalf("no standing produced %d, want 404 — anything else is an existence oracle: a 403 confirms the resource exists to someone who must not learn that", got.Status)
	}
	got, ok = FromDenial(stubDenial{noStanding: false, reason: "role:billing"}, "environment", "Ask an owner.")
	if !ok {
		t.Fatal("a denial was not classified as one")
	}
	if got.Status != 403 {
		t.Fatalf("has standing but lacks the permission produced %d, want an honest 403", got.Status)
	}
}

// The 403's detail is the REASON, not the error string. err.Error() carries a Go
// package prefix, and using it put "identity: access denied: role:billing" on
// the customer-facing API surface. Asserted EXACTLY — "contains role:billing"
// would pass for the leaking form too.
func TestFromDenialShowsTheReasonNotTheGoErrorString(t *testing.T) {
	got, _ := FromDenial(stubDenial{noStanding: false, reason: "role:billing"}, "environment", "Ask an owner.")
	if got.Detail != "role:billing" {
		t.Fatalf("403 detail is %q, want exactly %q — err.Error() leaks the Go package name onto the API surface", got.Detail, "role:billing")
	}
	if strings.Contains(got.Detail, "identity:") {
		t.Fatalf("403 detail leaks an internal package name: %q", got.Detail)
	}
	if got.Remediation == "" {
		t.Fatal("every problem must carry a remediation (AGENTS.md hard rule)")
	}
}

// THE PASS-THROUGH. An error that is not a denial must be reported as itself.
//
// Live consequence of getting this wrong: a database failure inside
// authz.Require would render as 404 "environment was not found" — an outage
// disguised as a not-found, invisible to anything watching 5xx.
func TestFromDenialDoesNotClassifyErrorsThatAreNotDenials(t *testing.T) {
	for _, err := range []error{
		errors.New("boom"),
		fmt.Errorf("query environments: %w", errors.New("connection refused")),
		nil,
	} {
		got, ok := FromDenial(err, "environment", "Ask an owner.")
		if ok {
			t.Fatalf("%v was classified as an authorization denial and mapped to %d — a non-denial must pass through so the caller can report it as itself", err, got.Status)
		}
		if got.Status != 0 || got.Type != "" || got.Title != "" || got.Detail != "" {
			t.Fatalf("a non-denial produced a non-zero Problem: %+v", got)
		}
	}
}

// errors.As unwraps, so a denial wrapped by a caller is still a denial. Without
// this, adding a `fmt.Errorf("...: %w", err)` anywhere on the path silently
// converts every 404 into whatever the generic arm does.
func TestFromDenialSeesThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("checking standing: %w", stubDenial{noStanding: true, reason: "role:billing"})
	got, ok := FromDenial(wrapped, "environment", "Ask an owner.")
	if !ok || got.Status != 404 {
		t.Fatalf("a wrapped denial was not classified (ok=%v status=%d)", ok, got.Status)
	}
}
