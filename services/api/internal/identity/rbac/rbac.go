// Package rbac is the two-layer authorization evaluator (ADR-0006/036):
// the role matrix grants a CEILING, the policy layer may only NARROW —
// composing as intersection (the AWS Organizations SCP model). The matrix is
// DATA: matrix.csv is copied verbatim from docs/product/11-permissions/ by
// `make gen-go` (drift fails CI); it is parsed strictly at boot and never
// hard-coded. Y is a ceiling; N is final — nothing widens.
package rbac

import (
	"context"
	_ "embed"
	"encoding/csv"
	"fmt"
	"strings"
)

//go:embed matrix.csv
var matrixCSV string

// Role is an org-level role (the matrix columns, in file order).
type Role string

const (
	RoleOwner     Role = "owner"
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
	RoleBilling   Role = "billing"
)

// Permission is a `resource.action` string (the matrix rows).
type Permission string

// Scope is where the check applies; environment-sensitive restrictions are
// ALWAYS policies, never role rows (the matrix has no env column by design).
type Scope struct {
	OrgID     string
	ProjectID string
	EnvID     string
}

// Decision explains itself: a 403 must name the missing role OR the denying
// policy (E3 grammar) — DeniedBy carries exactly that.
type Decision struct {
	Allowed  bool
	DeniedBy string // "role:<role> lacks <permission>" | "policy:<name> …" | "membership:none"
}

// PolicyLayer is the narrowing seam (implemented by T2.4). It is consulted
// only when the matrix says Y; it can deny, never grant.
type PolicyLayer interface {
	Evaluate(ctx context.Context, role Role, perm Permission, scope Scope) Decision
}

// Matrix is the parsed ceiling table.
type Matrix struct {
	cells     map[Permission]map[Role]bool
	delegated map[Permission]bool
	perms     []Permission
}

// Load parses the embedded matrix strictly: exactly the four known role
// columns, Y/N cells only, no duplicate rows. Any deviation fails boot.
func Load() (*Matrix, error) {
	r := csv.NewReader(strings.NewReader(matrixCSV))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("rbac: parse matrix: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("rbac: matrix empty")
	}
	header := records[0]
	want := []string{"permission", "owner", "admin", "developer", "billing"}
	if len(header) != len(want) {
		return nil, fmt.Errorf("rbac: matrix header has %d columns, want %d", len(header), len(want))
	}
	for i, h := range header {
		if h != want[i] {
			return nil, fmt.Errorf("rbac: matrix column %d is %q, want %q", i, h, want[i])
		}
	}
	roles := []Role{RoleOwner, RoleAdmin, RoleDeveloper, RoleBilling}
	m := &Matrix{cells: make(map[Permission]map[Role]bool, len(records)-1), delegated: map[Permission]bool{}}
	for n, row := range records[1:] {
		perm := Permission(row[0])
		if _, dup := m.cells[perm]; dup {
			return nil, fmt.Errorf("rbac: duplicate permission row %q", perm)
		}
		// "per underlying action" marks a DELEGATED permission (the matrix's
		// own encoding of AI Law 1: applying a proposal re-runs RBAC for the
		// underlying change). It is never answerable directly.
		if row[1] == "per underlying action" {
			m.delegated[perm] = true
			m.perms = append(m.perms, perm)
			continue
		}
		cells := make(map[Role]bool, 4)
		for i, cell := range row[1:] {
			switch cell {
			case "Y":
				cells[roles[i]] = true
			case "N":
				cells[roles[i]] = false
			default:
				return nil, fmt.Errorf("rbac: row %d (%s) cell %q is %q, want Y|N|per underlying action", n+2, perm, roles[i], cell)
			}
		}
		m.cells[perm] = cells
		m.perms = append(m.perms, perm)
	}
	return m, nil
}

// Delegated reports whether a permission must be re-evaluated as its
// underlying action (ai.apply_proposal). Checking it directly always denies
// with that instruction.
func (m *Matrix) Delegated(p Permission) bool { return m.delegated[p] }

// Permissions returns every known permission (test sweep + validation).
func (m *Matrix) Permissions() []Permission { return m.perms }

// Known reports whether a permission exists in the matrix.
func (m *Matrix) Known(p Permission) bool {
	if m.delegated[p] {
		return true
	}
	_, ok := m.cells[p]
	return ok
}

// Ceiling is the raw matrix cell; unknown permissions are always false
// (deny-by-default: an unregistered permission is a programming error
// surfaced as a denial, never a silent allow).
func (m *Matrix) Ceiling(role Role, perm Permission) bool {
	cells, ok := m.cells[perm]
	if !ok {
		return false
	}
	return cells[role]
}

// Evaluator composes the matrix ceiling with the narrowing policy layer.
type Evaluator struct {
	matrix   *Matrix
	policies PolicyLayer // nil until T2.4 registers the typed-Go layer
}

func NewEvaluator(m *Matrix, policies PolicyLayer) *Evaluator {
	return &Evaluator{matrix: m, policies: policies}
}

// Check runs the two layers: matrix N is final; matrix Y consults policies
// (no policy layer registered == no narrowing exists yet == permit).
func (e *Evaluator) Check(ctx context.Context, role Role, perm Permission, scope Scope) Decision {
	if !e.matrix.Known(perm) {
		return Decision{Allowed: false, DeniedBy: fmt.Sprintf("permission:%s is not registered in the matrix", perm)}
	}
	if e.matrix.Delegated(perm) {
		return Decision{Allowed: false, DeniedBy: fmt.Sprintf("permission:%s is evaluated per underlying action — check that permission instead (AI Law 1)", perm)}
	}
	if !e.matrix.Ceiling(role, perm) {
		return Decision{Allowed: false, DeniedBy: fmt.Sprintf("role:%s lacks %s", role, perm)}
	}
	if e.policies != nil {
		if d := e.policies.Evaluate(ctx, role, perm, scope); !d.Allowed {
			return d
		}
	}
	return Decision{Allowed: true}
}

// Known / Delegated expose the matrix predicates for principal types that
// authorize outside the role model (org keys — ADR-0007) but must still honor
// the same deny-by-default and delegated-permission rules.
func (e *Evaluator) Known(perm Permission) bool     { return e.matrix.Known(perm) }
func (e *Evaluator) Delegated(perm Permission) bool { return e.matrix.Delegated(perm) }

// CheckPolicies runs ONLY the narrowing layer (no role/ceiling): for a
// principal whose ceiling is its own granted subset (an org key), policies
// still tighten. The policy layer is role-agnostic in practice; "" is passed
// where a role would be. Permit when no layer is registered.
func (e *Evaluator) CheckPolicies(ctx context.Context, perm Permission, scope Scope) Decision {
	if e.policies == nil {
		return Decision{Allowed: true}
	}
	return e.policies.Evaluate(ctx, Role(""), perm, scope)
}
