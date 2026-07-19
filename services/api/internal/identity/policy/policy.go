// Package policy is the narrowing layer of the two-layer evaluator
// (ADR-0006): typed Go over the finite platform set — no DSL (Cedar is
// reserved for customer-authored policies, v3). Policies TIGHTEN only:
// the layer is consulted exclusively when the matrix says Y, so a matrix N
// is structurally unwidenable (property-tested). Resolution is closest-wins
// per key: a project-level policy overrides the org-level one of the same
// key; org sets the floor. Authoring surfaces are E12; this is evaluation.
package policy

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/steloit/cloud/services/api/internal/identity/rbac"
)

// Row is the storage-shaped policy (kept storage-agnostic here; the identity
// module adapts store.Policy into it).
type Row struct {
	ID          string
	OrgID       string
	ProjectID   string // "" = org-wide
	Key         string
	Enforcement string // enabled | opt_in | disabled
	Config      []byte // jsonb; typed kinds parse what they need
}

// Kind decides for one policy key. Returning an empty string permits;
// returning a reason denies (the reason feeds the 403's policy naming).
type Kind func(ctx context.Context, p Row, role rbac.Role, perm rbac.Permission, scope rbac.Scope) (denyReason string)

// Engine implements rbac.PolicyLayer over a policy source.
type Engine struct {
	mu     sync.RWMutex
	kinds  map[string]Kind
	source Source
}

// Source lists the policies attached to an org (org- and project-level rows).
type Source interface {
	PoliciesForOrg(ctx context.Context, orgID string) ([]Row, error)
}

func NewEngine(source Source) *Engine {
	e := &Engine{kinds: map[string]Kind{}, source: source}
	// The registered key MUST match the authored `key` verbatim (the DB stores
	// what the API accepts) — the contract spells it "ai-assistant" (hyphen).
	e.Register("ai-assistant", aiAssistantKind)
	return e
}

// Register adds a typed kind. Modules owning later kinds (spend caps, region
// allowlists, approval gates) register from their own packages — the finite
// set grows by registration, never by DSL.
func (e *Engine) Register(key string, k Kind) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.kinds[key] = k
}

// Knows reports whether a policy key has a registered evaluator. The authoring
// layer (T12.1) uses it to refuse promoting an unimplemented kind to `enforce`:
// a policy the platform cannot evaluate must never be a hard block.
func (e *Engine) Knows(key string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.kinds[key]
	return ok
}

// Evaluate implements rbac.PolicyLayer: resolve closest-wins per key, then
// every applicable kind must permit (intersection semantics).
func (e *Engine) Evaluate(ctx context.Context, role rbac.Role, perm rbac.Permission, scope rbac.Scope) rbac.Decision {
	if scope.OrgID == "" {
		return rbac.Decision{Allowed: true} // nothing to attach to yet
	}
	rows, err := e.source.PoliciesForOrg(ctx, scope.OrgID)
	if err != nil {
		// Fail closed: an unreadable policy source must never widen access.
		return rbac.Decision{Allowed: false, DeniedBy: "policy:source unavailable — denied fail-closed"}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	// closest-wins per key: project-level beats org-level.
	effective := map[string]Row{}
	for _, r := range rows {
		if r.ProjectID != "" && r.ProjectID != scope.ProjectID {
			continue // attached to a different project
		}
		cur, seen := effective[r.Key]
		if !seen || (cur.ProjectID == "" && r.ProjectID != "") {
			effective[r.Key] = r
		}
	}
	for key, row := range effective {
		// Warn-first (G9): a policy in `warn` posture is telemetry only — it
		// counts violations, never denies. Only enforcing postures affect authZ.
		if row.Enforcement == "warn" {
			continue
		}
		kind, known := e.kinds[key]
		if !known {
			// An unimplemented kind is inert UNLESS it is actively enforcing.
			// Authoring refuses `enforce` for an unknown kind (T12.1), so this
			// is defense-in-depth: an enforcing policy we cannot evaluate fails
			// closed rather than silently permitting; a non-enforcing one (a
			// warn/opt_in row for a future kind) is ignored, never a lockout.
			if row.Enforcement == "enforce" {
				return rbac.Decision{Allowed: false, DeniedBy: fmt.Sprintf("policy:%s is enforcing but not a recognized kind — denied fail-closed", key)}
			}
			continue
		}
		if reason := kind(ctx, row, role, perm, scope); reason != "" {
			return rbac.Decision{Allowed: false, DeniedBy: fmt.Sprintf("policy:%s %s", key, reason)}
		}
	}
	return rbac.Decision{Allowed: true}
}

// aiAssistantKind — the `ai-assistant` org policy (AI Law 4): `disabled`
// removes the AI layer; every ai.* permission denies. `opt_in`/`enabled`
// permit (opt-in presentation is a UI concern, not an authZ denial).
func aiAssistantKind(_ context.Context, p Row, _ rbac.Role, perm rbac.Permission, _ rbac.Scope) string {
	if !strings.HasPrefix(string(perm), "ai.") {
		return ""
	}
	if p.Enforcement == "disabled" {
		return "disables the AI layer (Law 4); the platform is whole without it"
	}
	return ""
}
