package policy

import (
	"context"
	"math/rand"
	"strings"
	"testing"

	"github.com/steloit/cloud/services/api/internal/identity/rbac"
)

type staticSource struct{ rows []Row }

func (s staticSource) PoliciesForOrg(context.Context, string) ([]Row, error) { return s.rows, nil }

func eval(rows ...Row) *rbac.Evaluator {
	m, err := rbac.Load()
	if err != nil {
		panic(err)
	}
	return rbac.NewEvaluator(m, NewEngine(staticSource{rows: rows}))
}

// AC (property test): NO policy set can widen a matrix N. Randomized policy
// rows over every N cell — the outcome must remain denied, naming the role.
func TestPropertyNoPolicyWidensN(t *testing.T) {
	m, err := rbac.Load()
	if err != nil {
		t.Fatal(err)
	}
	roles := []rbac.Role{rbac.RoleOwner, rbac.RoleAdmin, rbac.RoleDeveloper, rbac.RoleBilling}
	rng := rand.New(rand.NewSource(42)) // deterministic property sweep
	keys := []string{"ai-assistant", "unknown_future_key"}
	enfs := []string{"enabled", "opt_in", "disabled"}

	checked := 0
	for _, perm := range m.Permissions() {
		if m.Delegated(perm) {
			continue
		}
		for _, role := range roles {
			if m.Ceiling(role, perm) {
				continue // sweep N cells only
			}
			// randomized policy environment, 5 variants per N cell
			for v := 0; v < 5; v++ {
				var rows []Row
				for n := rng.Intn(4); n > 0; n-- {
					rows = append(rows, Row{
						OrgID: "org_x", Key: keys[rng.Intn(len(keys))],
						Enforcement: enfs[rng.Intn(len(enfs))],
					})
				}
				d := eval(rows...).Check(context.Background(), role, perm, rbac.Scope{OrgID: "org_x"})
				if d.Allowed {
					t.Fatalf("policy set widened N: %s × %s with %+v", perm, role, rows)
				}
				if !strings.Contains(d.DeniedBy, "role:") && !strings.Contains(d.DeniedBy, "policy:") {
					t.Fatalf("denial unexplained: %q", d.DeniedBy)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no N cells swept")
	}
	t.Logf("property held over %d randomized N-cell checks", checked)
}

// TestAIAssistantDisabledNarrowsAI: disabled denies ai.*, permits others.
func TestAIAssistantKind(t *testing.T) {
	off := Row{OrgID: "org_x", Key: "ai-assistant", Enforcement: "disabled"}
	e := eval(off)
	d := e.Check(context.Background(), rbac.RoleOwner, "ai.use", rbac.Scope{OrgID: "org_x"})
	if d.Allowed || !strings.Contains(d.DeniedBy, "policy:ai-assistant") {
		t.Fatalf("disabled ai-assistant did not narrow ai.*: %+v", d)
	}
	d = e.Check(context.Background(), rbac.RoleOwner, "project.create", rbac.Scope{OrgID: "org_x"})
	if !d.Allowed {
		t.Fatalf("ai-assistant narrowed a non-ai permission: %+v", d)
	}
	on := Row{OrgID: "org_x", Key: "ai-assistant", Enforcement: "enabled"}
	d = eval(on).Check(context.Background(), rbac.RoleOwner, "ai.use", rbac.Scope{OrgID: "org_x"})
	if !d.Allowed {
		t.Fatalf("enabled ai-assistant denied ai.*: %+v", d)
	}
}

// Closest wins: a project-level row overrides the org-level row of the same key.
func TestClosestWins(t *testing.T) {
	orgOff := Row{OrgID: "org_x", Key: "ai-assistant", Enforcement: "disabled"}
	projOn := Row{OrgID: "org_x", ProjectID: "prj_a", Key: "ai-assistant", Enforcement: "enabled"}
	e := eval(orgOff, projOn)
	// In prj_a the project-level 'enabled' is closest → permit.
	d := e.Check(context.Background(), rbac.RoleOwner, "ai.use", rbac.Scope{OrgID: "org_x", ProjectID: "prj_a"})
	if !d.Allowed {
		t.Fatalf("closest-wins failed (project enabled): %+v", d)
	}
	// In another project only the org-level 'disabled' applies → deny.
	d = e.Check(context.Background(), rbac.RoleOwner, "ai.use", rbac.Scope{OrgID: "org_x", ProjectID: "prj_b"})
	if d.Allowed {
		t.Fatalf("org floor ignored in prj_b: %+v", d)
	}
}

// An ENFORCING policy whose kind is unimplemented fails CLOSED — but a
// non-enforcing (warn/opt_in) row for an unknown future kind is inert, never a
// lockout (warn-first). Authoring refuses enforce+unknown, so the fail-closed
// path is defense-in-depth.
func TestFailClosed(t *testing.T) {
	enforcing := Row{OrgID: "org_x", Key: "mystery_key", Enforcement: "enforce"}
	d := eval(enforcing).Check(context.Background(), rbac.RoleOwner, "project.create", rbac.Scope{OrgID: "org_x"})
	if d.Allowed || !strings.Contains(d.DeniedBy, "mystery_key") {
		t.Fatalf("enforcing unknown key did not fail closed: %+v", d)
	}
	// the same unknown kind in warn posture is inert — the org is not bricked.
	warn := Row{OrgID: "org_x", Key: "mystery_key", Enforcement: "warn"}
	d = eval(warn).Check(context.Background(), rbac.RoleOwner, "project.create", rbac.Scope{OrgID: "org_x"})
	if !d.Allowed {
		t.Fatalf("warn-mode unknown kind bricked authorization: %+v", d)
	}
}
