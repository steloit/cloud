package rbac

import (
	"context"
	"encoding/csv"
	"strings"
	"testing"
)

// AC: table-driven test over EVERY cell — the embedded matrix must agree with
// its own source, cell by cell, through the public Check path.
func TestEveryCell(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	e := NewEvaluator(m, nil)
	records, err := csv.NewReader(strings.NewReader(matrixCSV)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	roles := []Role{RoleOwner, RoleAdmin, RoleDeveloper, RoleBilling}
	cells := 0
	for _, row := range records[1:] {
		perm := Permission(row[0])
		if row[1] == "per underlying action" {
			d := e.Check(context.Background(), RoleOwner, perm, Scope{})
			if d.Allowed || !strings.Contains(d.DeniedBy, "underlying action") {
				t.Errorf("delegated %s not denied with instruction: %+v", perm, d)
			}
			continue
		}
		for i, want := range row[1:] {
			d := e.Check(context.Background(), roles[i], perm, Scope{OrgID: "org_x"})
			if d.Allowed != (want == "Y") {
				t.Errorf("%s × %s: got %v want %s", perm, roles[i], d.Allowed, want)
			}
			if !d.Allowed && !strings.Contains(d.DeniedBy, string(roles[i])) {
				t.Errorf("%s × %s: denial does not name the role: %q", perm, roles[i], d.DeniedBy)
			}
			cells++
		}
	}

	t.Logf("swept %d cells over %d permissions", cells, len(records)-1)
}

func TestUnknownPermissionDenies(t *testing.T) {
	m, _ := Load()
	e := NewEvaluator(m, nil)
	d := e.Check(context.Background(), RoleOwner, "made.up", Scope{})
	if d.Allowed || !strings.Contains(d.DeniedBy, "not registered") {
		t.Fatalf("unknown permission not denied with explanation: %+v", d)
	}
}

type denyAll struct{}

func (denyAll) Evaluate(_ context.Context, _ Role, p Permission, _ Scope) Decision {
	return Decision{Allowed: false, DeniedBy: "policy:test-deny narrows " + string(p)}
}

type allowAll struct{}

func (allowAll) Evaluate(context.Context, Role, Permission, Scope) Decision {
	return Decision{Allowed: true}
}

// Policies narrow Y but can NEVER widen N.
func TestPolicyNarrowsNeverWidens(t *testing.T) {
	m, _ := Load()

	deny := NewEvaluator(m, denyAll{})
	d := deny.Check(context.Background(), RoleOwner, "project.create", Scope{})
	if d.Allowed || !strings.Contains(d.DeniedBy, "policy:test-deny") {
		t.Fatalf("policy did not narrow a Y cell: %+v", d)
	}

	widen := NewEvaluator(m, allowAll{})
	d = widen.Check(context.Background(), RoleBilling, "project.create", Scope{})
	if d.Allowed {
		t.Fatal("policy widened an N cell — N must be final")
	}
}
