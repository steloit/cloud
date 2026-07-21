// Package reconcile is the internal-plane reconciler protocol (D9/A2.5,
// docs/plan/e1-substrate-design.md §2). The control plane writes DESIRED state;
// the cell-agent converges ACTUAL and reports back. Nothing in this package
// provisions anything — a handler that provisions is a defect (D9).
package reconcile

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// Auth resolves a reconciler-scoped bearer to the cells it may act on.
//
// This principal is deliberately SEPARATE from user sessions and org API keys:
// a user token must never reach these endpoints, and a cell token must never
// reach a customer surface. Keeping the two resolvers apart makes that a
// structural property rather than a check somebody has to remember.
//
// Alpha shape: one shared secret carried in config, scoped to an explicit cell
// list. Per-cell rotation and mTLS are deliberately deferred — with one cell
// they would be ceremony, and the seam here (Cells(token)) is what a real
// credential store plugs into later without touching the handlers.
type Auth struct {
	// digest of the configured secret; empty disables the endpoints entirely.
	digest [32]byte
	set    bool
	cells  map[string]bool
}

// NewAuth builds the resolver. An empty secret leaves the endpoints closed —
// absent config must never mean "open", which is the fail-open shape that
// keeps biting this repo.
func NewAuth(secret string, cells []string) *Auth {
	a := &Auth{cells: make(map[string]bool, len(cells))}
	for _, c := range cells {
		if c = strings.TrimSpace(c); c != "" {
			a.cells[c] = true
		}
	}
	if secret != "" {
		a.digest = sha256.Sum256([]byte(secret))
		a.set = true
	}
	return a
}

// Enabled reports whether a reconciler secret is configured at all.
func (a *Auth) Enabled() bool { return a != nil && a.set }

// Allows reports whether the request carries the reconciler secret AND that
// principal owns the named cell. Comparison is constant-time over a digest, so
// neither the secret's value nor its length leaks through timing.
//
// The two failures are deliberately NOT distinguished to the caller: an
// unknown cell and an unauthorized cell both come back false, and the handler
// renders 404 for each. A reconciler token must not be able to enumerate cells.
func (a *Auth) Allows(r *http.Request, cell string) bool {
	if !a.Enabled() || cell == "" {
		return false
	}
	tok := bearer(r)
	if tok == "" {
		return false
	}
	got := sha256.Sum256([]byte(tok))
	if subtle.ConstantTimeCompare(got[:], a.digest[:]) != 1 {
		return false
	}
	return a.cells[cell]
}

// Authenticated reports whether the bearer is the reconciler secret, ignoring
// cell scope — the seam that lets a handler answer 401 (bad token) separately
// from 404 (good token, not your cell).
func (a *Auth) Authenticated(r *http.Request) bool {
	if !a.Enabled() {
		return false
	}
	tok := bearer(r)
	if tok == "" {
		return false
	}
	got := sha256.Sum256([]byte(tok))
	return subtle.ConstantTimeCompare(got[:], a.digest[:]) == 1
}

func bearer(r *http.Request) string {
	if r == nil {
		return ""
	}
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if len(h) <= len(p) || !strings.EqualFold(h[:len(p)], p) {
		return ""
	}
	return strings.TrimSpace(h[len(p):])
}
