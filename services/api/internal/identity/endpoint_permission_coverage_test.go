package identity

// Q4: the endpoint↔matrix drift guard. Every permission any HTTP endpoint
// enforces via authz.Require MUST be a known, non-delegated matrix permission —
// otherwise that endpoint is deny-by-default forever (a silent 403 nobody can
// satisfy). This table mirrors the Require call sites across the API modules;
// when an endpoint's required permission changes, this test is the tripwire
// that it still exists in the ceiling data.

import (
	"testing"

	"github.com/steloit/cloud/services/api/internal/identity/rbac"
)

// enforcedPermissions is every permission checked at an endpoint (identity +
// provisioning Require sites). Keep in sync with the handlers; the test below
// proves each is real, so a typo or a matrix removal fails CI, not production.
var enforcedPermissions = []rbac.Permission{
	// identity module
	"org.manage", "org.delete", "members.role_change", "members.invite",
	"api_keys.manage", "billing.view", "audit.read", "observe.read",
	// provisioning module
	"binding.manage", "deploy.promote", "deploy.rollback",
	"project.create", "project.delete", "env.manage",
	"service.create", "service.scale", "service.delete",
}

func TestEndpointPermissionsAreRegistered(t *testing.T) {
	m, err := rbac.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, perm := range enforcedPermissions {
		if !m.Known(perm) {
			t.Errorf("endpoint enforces %q which is NOT in the matrix — deny-by-default forever", perm)
		}
		if m.Delegated(perm) {
			t.Errorf("endpoint enforces delegated %q — delegated perms are never answerable directly", perm)
		}
	}
}

// A permission every endpoint enforces must be reachable by at least one role
// (a permission no role holds is dead weight and an always-403 endpoint).
func TestEnforcedPermissionsReachableBySomeRole(t *testing.T) {
	m, _ := rbac.Load()
	roles := []rbac.Role{rbac.RoleOwner, rbac.RoleAdmin, rbac.RoleDeveloper, rbac.RoleBilling}
	for _, perm := range enforcedPermissions {
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
// governance, observability). The log shrinks as those epics ship; a surprise
// entry flags a permission that lost its endpoint.
func TestMatrixCoverageReport(t *testing.T) {
	m, _ := rbac.Load()
	enforced := map[rbac.Permission]bool{}
	for _, p := range enforcedPermissions {
		enforced[p] = true
	}
	var unenforced []rbac.Permission
	for _, p := range m.Permissions() {
		if m.Delegated(p) || enforced[p] {
			continue
		}
		unenforced = append(unenforced, p)
	}
	t.Logf("%d/%d matrix permissions enforced at endpoints; %d awaiting their epics: %v",
		len(enforced), len(m.Permissions()), len(unenforced), unenforced)
}
