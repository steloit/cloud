package identity

// Q4: the ADR-0007 org-key authorization path, swept against the matrix with no
// database — the org-key branch of Require/Holds authorizes purely from the
// key's granted subset + the evaluator, never touching membership.

import (
	"context"
	"strings"
	"testing"

	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
)

// denyPolicy narrows a single permission, to prove policies still tighten a
// service principal (tighten-only holds for org keys too).
type denyOnePolicy struct{ perm rbac.Permission }

func (p denyOnePolicy) Evaluate(_ context.Context, _ rbac.Role, perm rbac.Permission, _ rbac.Scope) rbac.Decision {
	if perm == p.perm {
		return rbac.Decision{Allowed: false, DeniedBy: "policy:pin narrows " + string(perm)}
	}
	return rbac.Decision{Allowed: true}
}

func orgKey(orgID string, perms ...string) session.Principal {
	return session.Principal{Kind: "token", UserID: "", OrgID: orgID, TokenID: "tok_k", Permissions: perms}
}

func newOrgKeyAuthorizer(t *testing.T, policy rbac.PolicyLayer) *Authorizer {
	t.Helper()
	m, err := rbac.Load()
	if err != nil {
		t.Fatal(err)
	}
	// nil store + nil recorder: the org-key path reads neither.
	return NewAuthorizer(nil, rbac.NewEvaluator(m, policy), nil)
}

// TestOrgKeySweep: for a key granted a subset of the matrix, Require allows a
// permission IFF it is granted, known, non-delegated, and the scope org matches
// — swept over EVERY matrix permission.
func TestOrgKeySweep(t *testing.T) {
	m, _ := rbac.Load()
	az := newOrgKeyAuthorizer(t, nil)
	ctx := context.Background()

	// grant a representative real subset (must be non-delegated matrix perms)
	granted := []string{"project.create", "service.create", "observe.read", "deploy.promote"}
	grantedSet := map[string]bool{}
	for _, g := range granted {
		grantedSet[g] = true
	}
	key := orgKey("org_k", granted...)
	scope := rbac.Scope{OrgID: "org_k"}

	swept := 0
	for _, perm := range m.Permissions() {
		ps := string(perm)
		err := az.Require(ctx, key, perm, scope)
		wantAllow := grantedSet[ps] && !m.Delegated(perm) && m.Known(perm)
		if wantAllow && err != nil {
			t.Errorf("granted %s denied: %v", ps, err)
		}
		if !wantAllow && err == nil {
			t.Errorf("ungranted/ineligible %s allowed", ps)
		}
		// Holds must agree with Require (both are the same ceiling, audit aside)
		if got := az.Holds(ctx, key, perm, scope); got != (err == nil) {
			t.Errorf("Holds(%s)=%v disagrees with Require err==nil=%v", ps, got, err == nil)
		}
		swept++
	}
	t.Logf("org-key path swept over %d permissions", swept)
}

// A granted permission is denied when the scope org differs from the key's org.
func TestOrgKeyForeignScopeDenied(t *testing.T) {
	az := newOrgKeyAuthorizer(t, nil)
	key := orgKey("org_k", "project.create")
	err := az.Require(context.Background(), key, "project.create", rbac.Scope{OrgID: "org_other"})
	var denied AccessDeniedError
	if err == nil || !asDenied(err, &denied) || !strings.Contains(denied.DeniedBy, "different organization") {
		t.Fatalf("foreign-scope not denied with explanation: %v", err)
	}
}

// A delegated permission is never grantable, even if listed on the key.
func TestOrgKeyDelegatedDenied(t *testing.T) {
	az := newOrgKeyAuthorizer(t, nil)
	key := orgKey("org_k", "ai.apply_proposal")
	err := az.Require(context.Background(), key, "ai.apply_proposal", rbac.Scope{OrgID: "org_k"})
	var denied AccessDeniedError
	if err == nil || !asDenied(err, &denied) || !strings.Contains(denied.DeniedBy, "underlying action") {
		t.Fatalf("delegated perm not denied with AI-Law-1 grammar: %v", err)
	}
}

// An unknown permission on a key is deny-by-default (as for roles).
func TestOrgKeyUnknownDenied(t *testing.T) {
	az := newOrgKeyAuthorizer(t, nil)
	key := orgKey("org_k", "made.up.perm")
	err := az.Require(context.Background(), key, "made.up.perm", rbac.Scope{OrgID: "org_k"})
	var denied AccessDeniedError
	if err == nil || !asDenied(err, &denied) || !strings.Contains(denied.DeniedBy, "not registered") {
		t.Fatalf("unknown perm not denied with explanation: %v", err)
	}
}

// A granted-but-not-listed permission names least-privilege.
func TestOrgKeyUngrantedNamesLeastPrivilege(t *testing.T) {
	az := newOrgKeyAuthorizer(t, nil)
	key := orgKey("org_k", "observe.read")
	err := az.Require(context.Background(), key, "project.create", rbac.Scope{OrgID: "org_k"})
	var denied AccessDeniedError
	if err == nil || !asDenied(err, &denied) || !strings.Contains(denied.DeniedBy, "least-privilege") {
		t.Fatalf("ungranted perm did not name least-privilege: %v", err)
	}
}

// Policies still narrow an org key (tighten-only applies to service principals).
func TestOrgKeyPolicyNarrows(t *testing.T) {
	az := newOrgKeyAuthorizer(t, denyOnePolicy{perm: "project.create"})
	key := orgKey("org_k", "project.create", "service.create")
	ctx := context.Background()
	if err := az.Require(ctx, key, "project.create", rbac.Scope{OrgID: "org_k"}); err == nil {
		t.Fatal("policy did not narrow a granted org-key permission")
	}
	// a different granted perm is unaffected
	if err := az.Require(ctx, key, "service.create", rbac.Scope{OrgID: "org_k"}); err != nil {
		t.Fatalf("policy over-narrowed: %v", err)
	}
}

// ValidGrantable is exactly Known ∧ !Delegated over the whole matrix.
func TestValidGrantableMatchesMatrix(t *testing.T) {
	m, _ := rbac.Load()
	az := newOrgKeyAuthorizer(t, nil)
	for _, perm := range m.Permissions() {
		want := m.Known(perm) && !m.Delegated(perm)
		if got := az.ValidGrantable(perm); got != want {
			t.Errorf("ValidGrantable(%s)=%v want %v", perm, got, want)
		}
	}
	if az.ValidGrantable("made.up") {
		t.Error("unknown permission reported grantable")
	}
}

func asDenied(err error, target *AccessDeniedError) bool {
	if d, ok := err.(AccessDeniedError); ok {
		*target = d
		return true
	}
	return false
}
