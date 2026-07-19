package identity

// Q4: the endpoint↔matrix drift guard. Every permission any HTTP endpoint
// enforces via authz.Require / requireOrg MUST be a known, non-delegated matrix
// permission — otherwise that endpoint is deny-by-default forever (a silent 403
// nobody can satisfy). The enforced set is DERIVED FROM SOURCE at test time
// (TestEnforcedListMatchesSource), so a new endpoint enforcing an unregistered
// permission fails CI here, not in production.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/steloit/cloud/services/api/internal/identity/rbac"
)

// enforcedPermissions is the human-readable contract: every permission checked
// at an endpoint (identity + provisioning + the SSE streamer). It is kept
// honest by TestEnforcedListMatchesSource, which regenerates the same set by
// scanning the handler source — the two must agree, so this list can't rot.
var enforcedPermissions = []rbac.Permission{
	// identity module
	"org.manage", "org.delete", "members.role_change", "members.invite",
	"api_keys.manage", "billing.view", "audit.read", "observe.read", "policy.manage",
	// provisioning module
	"binding.manage", "deploy.promote", "deploy.rollback",
	"project.create", "project.delete", "env.manage",
	"service.create", "service.scale", "service.delete",
}

// permLiteral matches the permission string argument of a Require/requireOrg
// call: `requireOrg(ctx, org, "project.create", true)` / `Authorize(ctx, p,
// "observe.read", scope)`. The `perm`-variable indirection (no literal) is
// intentionally not matched — the literal lives at the requireOrg call site.
var permLiteral = regexp.MustCompile(`(?:requireOrg|\.Require|\.Authorize)\(ctx,[^"\n]*"([a-z_]+\.[a-z_]+)"`)

// scanEnforcedPermissions walks the handler packages (relative to this test's
// package dir) and returns every permission literal passed to an enforcement
// call. Failure modes it catches: a new endpoint enforcing a permission missing
// from enforcedPermissions, or one that isn't in the matrix at all.
func scanEnforcedPermissions(t *testing.T) map[rbac.Permission]bool {
	t.Helper()
	// CWD at test time is the package dir (…/internal/identity).
	dirs := []string{".", "../provisioning", "../events"}
	found := map[rbac.Permission]bool{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range permLiteral.FindAllStringSubmatch(string(b), -1) {
				found[rbac.Permission(m[1])] = true
			}
		}
	}
	return found
}

// TestEnforcedListMatchesSource is the real tripwire: the hand-maintained
// enforcedPermissions must equal the set actually enforced in the handler
// source. Add an endpoint with a new permission and forget this list → fail.
func TestEnforcedListMatchesSource(t *testing.T) {
	fromSource := scanEnforcedPermissions(t)
	listed := map[rbac.Permission]bool{}
	for _, p := range enforcedPermissions {
		listed[p] = true
	}
	for p := range fromSource {
		if !listed[p] {
			t.Errorf("source enforces %q but it is absent from enforcedPermissions (add it, and confirm it is in the matrix)", p)
		}
	}
	for p := range listed {
		if !fromSource[p] {
			t.Errorf("enforcedPermissions lists %q but no handler enforces it (stale entry?)", p)
		}
	}
	if len(fromSource) == 0 {
		t.Fatal("scanner found zero enforcement sites — regex or paths are wrong")
	}
}

func TestEndpointPermissionsAreRegistered(t *testing.T) {
	m, err := rbac.Load()
	if err != nil {
		t.Fatal(err)
	}
	// Prove the SOURCE-derived set (not just the hand list) is all registered,
	// so the deny-by-default bug is caught even before the list is updated.
	for perm := range scanEnforcedPermissions(t) {
		if !m.Known(perm) {
			t.Errorf("endpoint enforces %q which is NOT in the matrix — deny-by-default forever", perm)
		}
		if m.Delegated(perm) {
			t.Errorf("endpoint enforces delegated %q — delegated perms are never answerable directly", perm)
		}
	}
}

// A permission every endpoint enforces must be reachable by at least one role
// (a permission no role holds is an always-403 endpoint).
func TestEnforcedPermissionsReachableBySomeRole(t *testing.T) {
	m, _ := rbac.Load()
	roles := []rbac.Role{rbac.RoleOwner, rbac.RoleAdmin, rbac.RoleDeveloper, rbac.RoleBilling}
	for perm := range scanEnforcedPermissions(t) {
		reachable := false
		for _, r := range roles {
			if m.Ceiling(r, perm) {
				reachable = true
				break
			}
		}
		if !reachable {
			t.Errorf("enforced %q is granted to no role — endpoint is unreachable", perm)
		}
	}
}

// Coverage report: which matrix permissions are not yet enforced at an
// endpoint. Not a failure — they belong to epics not yet built (billing, AI,
// governance, observability). The log shrinks as those epics ship.
func TestMatrixCoverageReport(t *testing.T) {
	m, _ := rbac.Load()
	enforced := scanEnforcedPermissions(t)
	var unenforced []string
	for _, p := range m.Permissions() {
		if m.Delegated(p) || enforced[p] {
			continue
		}
		unenforced = append(unenforced, string(p))
	}
	sort.Strings(unenforced)
	t.Logf("%d/%d matrix permissions enforced at endpoints; %d awaiting their epics: %v",
		len(enforced), len(m.Permissions()), len(unenforced), unenforced)
}
