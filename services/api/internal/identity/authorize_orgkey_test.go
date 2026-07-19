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

	swept, allowed := 0, 0
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
		if err == nil {
			allowed++
		}
		// Holds must agree with Require (both are the same ceiling, audit aside)
		if got := az.Holds(ctx, key, perm, scope); got != (err == nil) {
			t.Errorf("Holds(%s)=%v disagrees with Require err==nil=%v", ps, got, err == nil)
		}
		swept++
	}
	// vacuity guard: the grant path must actually have exercised some allows,
	// else the whole sweep proved nothing (e.g. a mistyped grant list).
	if allowed != len(granted) {
		t.Fatalf("expected %d allowed permissions, got %d — sweep may be vacuous", len(granted), allowed)
	}
	t.Logf("org-key path swept over %d permissions, %d allowed", swept, allowed)
}

// TestOrgKeyMinterDelegationCeiling is the store-free half of the escalation
// guard (ADR-0007: you cannot grant what you do not hold). An org-key MINTER's
// Holds — the ceiling MintOrgKey checks per requested permission — is true only
// for permissions actually on the minter's own key. The user-minter half needs
// membership and stays in the Docker-gated integration test; this covers the
// service-principal path with no DB, so the ceiling is never masked green.
func TestOrgKeyMinterDelegationCeiling(t *testing.T) {
	az := newOrgKeyAuthorizer(t, nil)
	ctx := context.Background()
	scope := rbac.Scope{OrgID: "org_k"}
	minter := orgKey("org_k", "api_keys.manage", "observe.read")

	// holds what it was granted
	if !az.Holds(ctx, minter, "observe.read", scope) {
		t.Fatal("minter does not hold a permission on its own key")
	}
	// cannot delegate a permission it never held (would be admin→owner escalation)
	if az.Holds(ctx, minter, "org.delete", scope) {
		t.Fatal("minter Holds a permission it was never granted — delegation ceiling breached")
	}
	// nor a delegated one, even if somehow listed
	if az.Holds(ctx, minter, "ai.apply_proposal", scope) {
		t.Fatal("minter Holds a delegated permission — never grantable")
	}
	// nor anything in a foreign org
	if az.Holds(ctx, minter, "observe.read", rbac.Scope{OrgID: "org_other"}) {
		t.Fatal("minter Holds across a foreign org scope")
	}
}

// TestOrgKeyEmptyScopeDenied covers the `scope.OrgID == ""` half of the guard
// (foreign-scope test only hit the mismatch half).
func TestOrgKeyEmptyScopeDenied(t *testing.T) {
	az := newOrgKeyAuthorizer(t, nil)
	ctx := context.Background()
	key := orgKey("org_k", "observe.read")
	if err := az.Require(ctx, key, "observe.read", rbac.Scope{OrgID: ""}); err == nil {
		t.Fatal("empty-scope org not denied")
	}
	if az.Holds(ctx, key, "observe.read", rbac.Scope{OrgID: ""}) {
		t.Fatal("Holds allowed an empty-scope org")
	}
}

// TestOrgKeyGrantMatchIsExact: a granted permission is matched byte-for-byte —
// case, whitespace, and near-misses do not authorize (fails closed).
func TestOrgKeyGrantMatchIsExact(t *testing.T) {
	az := newOrgKeyAuthorizer(t, nil)
	ctx := context.Background()
	scope := rbac.Scope{OrgID: "org_k"}
	// the key literally grants "observe.read"; a request for a mangled form
	// (which isn't even a known matrix perm) must deny.
	key := orgKey("org_k", "observe.read")
	for _, bad := range []rbac.Permission{"Observe.Read", " observe.read", "observe.read ", "observe.write"} {
		if err := az.Require(ctx, key, bad, scope); err == nil {
			t.Errorf("mangled/near-miss permission %q authorized", bad)
		}
	}
	// and a key whose GRANTED entry is non-canonical does not authorize the
	// canonical permission (exact-match both directions).
	sloppy := orgKey("org_k", " observe.read ")
	if err := az.Require(ctx, sloppy, "observe.read", scope); err == nil {
		t.Fatal("a non-canonical granted entry authorized the canonical permission")
	}
}

// TestPrincipalKindIsNotOrgKey: only a token with no user and an org is an org
// key. A user token (even one carrying OrgID/Permissions) must NOT take the
// org-key branch — otherwise its Permissions would grant authority the role
// model never gave it.
func TestPrincipalKindIsNotOrgKey(t *testing.T) {
	cases := []session.Principal{
		{Kind: "token", UserID: "usr_1", OrgID: "org_k", Permissions: []string{"org.delete"}}, // has a user
		{Kind: "token", UserID: "", OrgID: "", Permissions: []string{"org.delete"}},            // no org
		{Kind: "session", UserID: "usr_1", OrgID: "org_k"},                                     // a live session
	}
	for i, p := range cases {
		if p.IsOrgKey() {
			t.Errorf("case %d wrongly classified as an org key: %+v", i, p)
		}
	}
	// the positive control
	if !orgKey("org_k", "observe.read").IsOrgKey() {
		t.Fatal("a real org key was not recognized")
	}
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
