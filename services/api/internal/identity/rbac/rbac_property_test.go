package rbac

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// predicatePolicy is a test PolicyLayer that denies exactly the (role, perm,
// scope) triples for which deny() returns true — a controllable narrowing seam.
type predicatePolicy struct {
	name string
	deny func(Role, Permission, Scope) bool
}

func (p predicatePolicy) Evaluate(_ context.Context, r Role, perm Permission, s Scope) Decision {
	if p.deny(r, perm, s) {
		return Decision{Allowed: false, DeniedBy: "policy:" + p.name + " narrows " + string(perm)}
	}
	return Decision{Allowed: true}
}

// policyFamily is a deterministic set of narrowing layers spanning the extremes
// (allow-all, deny-all) and index-derived subsets — no randomness (scripts and
// resumable tests forbid it), but broad enough to falsify a widening bug.
func policyFamily(perms []Permission) []predicatePolicy {
	fam := []predicatePolicy{
		{"allow-all", func(Role, Permission, Scope) bool { return false }},
		{"deny-all", func(Role, Permission, Scope) bool { return true }},
		{"deny-owner", func(r Role, _ Permission, _ Scope) bool { return r == RoleOwner }},
		{"deny-in-project-scope", func(_ Role, _ Permission, s Scope) bool { return s.ProjectID != "" }},
	}
	// One "deny every k-th permission" layer per small modulus — index-derived
	// subsets that still cover the whole permission space between them.
	index := map[Permission]int{}
	for i, p := range perms {
		index[p] = i
	}
	for _, k := range []int{2, 3, 5} {
		fam = append(fam, predicatePolicy{
			name: fmt.Sprintf("deny-every-%d", k),
			deny: func(_ Role, perm Permission, _ Scope) bool { return index[perm]%k == 0 },
		})
	}
	return fam
}

// TestTightenOnlyProperty is the core security invariant: a policy layer can
// only NARROW. For every role × permission × scope × policy, an Allowed result
// implies (a) the matrix ceiling was Y and non-delegated, and (b) the same
// check with no policy was also Allowed. No policy ever widens an N.
func TestTightenOnlyProperty(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	roles := []Role{RoleOwner, RoleAdmin, RoleDeveloper, RoleBilling}
	scopes := []Scope{
		{OrgID: "org_x"},
		{OrgID: "org_x", ProjectID: "prj_1"},
		{OrgID: "org_x", ProjectID: "prj_1", EnvID: "env_1"},
	}
	base := NewEvaluator(m, nil)
	fam := policyFamily(m.Permissions())
	checks, allows := 0, 0
	for _, pol := range fam {
		e := NewEvaluator(m, pol)
		for _, perm := range m.Permissions() {
			for _, role := range roles {
				for _, sc := range scopes {
					got := e.Check(context.Background(), role, perm, sc)
					if got.Allowed {
						allows++
					}
					if !got.Allowed {
						// a denial must always name its cause (E3 grammar)
						if got.DeniedBy == "" {
							t.Errorf("policy %s: %s×%s denied with empty DeniedBy", pol.name, perm, role)
						}
						checks++
						continue
					}
					// (a) Allowed ⟹ ceiling Y and not delegated
					if m.Delegated(perm) {
						t.Errorf("policy %s: delegated %s allowed for %s — must never answer directly", pol.name, perm, role)
					}
					if !m.Ceiling(role, perm) {
						t.Errorf("policy %s WIDENED an N cell: %s×%s allowed but matrix ceiling is N", pol.name, perm, role)
					}
					// (b) Allowed ⟹ allowed without the policy (policy only narrows)
					if !base.Check(context.Background(), role, perm, sc).Allowed {
						t.Errorf("policy %s WIDENED: %s×%s allowed with policy but denied by matrix alone", pol.name, perm, role)
					}
					// (c) if this policy denies the triple, it must not be allowed
					if pol.deny(role, perm, sc) {
						t.Errorf("policy %s: %s×%s allowed despite the policy denying it", pol.name, perm, role)
					}
					checks++
				}
			}
		}
	}
	if allows == 0 {
		t.Fatal("no Allowed result across the whole sweep — the widening assertions never ran (vacuous)")
	}
	t.Logf("tighten-only verified over %d checks (%d allowed) — widening assertions exercised", checks, allows)
}

// TestDenyByDefaultFuzz: inputs that must never authorize — an empty
// permission, an event-vocabulary name that is not a matrix row, and a garbage
// role. Deny-by-default is the whole safety net; prove it holds at the edges.
func TestDenyByDefaultFuzz(t *testing.T) {
	m, _ := Load()
	e := NewEvaluator(m, nil)
	ctx := context.Background()
	// permissions that are NOT matrix rows (empty, event names, typos)
	for _, perm := range []Permission{"", "authz.denied", "member.added", "org.created", "project.create.extra", "OBSERVE.READ"} {
		if m.Known(perm) {
			t.Errorf("%q unexpectedly Known — matrix row leaked into the event vocabulary?", perm)
		}
		for _, role := range []Role{RoleOwner, RoleAdmin, RoleDeveloper, RoleBilling} {
			d := e.Check(ctx, role, perm, Scope{OrgID: "org_x"})
			if d.Allowed {
				t.Errorf("deny-by-default breached: %q×%s allowed", perm, role)
			}
		}
	}
	// a role the matrix never defines must fail closed on a real permission
	for _, bad := range []Role{"", "Owner", "superuser", "root"} {
		if d := e.Check(ctx, bad, "project.create", Scope{OrgID: "org_x"}); d.Allowed {
			t.Errorf("unknown role %q authorized project.create", bad)
		}
	}
}

// TestDelegatedNeverAnswerable: every delegated permission denies for every
// role with the AI-Law-1 instruction, regardless of policy.
func TestDelegatedNeverAnswerable(t *testing.T) {
	m, _ := Load()
	e := NewEvaluator(m, allowAll{}) // even an allow-all policy can't answer it
	roles := []Role{RoleOwner, RoleAdmin, RoleDeveloper, RoleBilling}
	found := 0
	for _, perm := range m.Permissions() {
		if !m.Delegated(perm) {
			continue
		}
		found++
		for _, role := range roles {
			d := e.Check(context.Background(), role, perm, Scope{OrgID: "org_x"})
			if d.Allowed || !strings.Contains(d.DeniedBy, "underlying action") {
				t.Errorf("delegated %s×%s: %+v", perm, role, d)
			}
		}
	}
	if found == 0 {
		t.Fatal("no delegated permission in the matrix — expected ai.apply_proposal")
	}
}

// TestDenialGrammarNamesCause: a role denial names the role; a policy denial
// names the policy — the two branches a 403 body must distinguish (E3).
func TestDenialGrammarNamesCause(t *testing.T) {
	m, _ := Load()
	// role denial (N cell, no policy)
	roleDenied := NewEvaluator(m, nil).Check(context.Background(), RoleBilling, "project.create", Scope{OrgID: "org_x"})
	if roleDenied.Allowed || !strings.HasPrefix(roleDenied.DeniedBy, "role:") {
		t.Fatalf("role denial not grammatical: %+v", roleDenied)
	}
	// policy denial (Y cell narrowed)
	polDenied := NewEvaluator(m, denyAll{}).Check(context.Background(), RoleOwner, "project.create", Scope{OrgID: "org_x"})
	if polDenied.Allowed || !strings.HasPrefix(polDenied.DeniedBy, "policy:") {
		t.Fatalf("policy denial not grammatical: %+v", polDenied)
	}
}
