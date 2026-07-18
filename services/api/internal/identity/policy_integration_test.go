package identity_test

// T2.4: the policy layer against real DB rows — closest-wins resolution over
// the policies table, tighten-only through the full Authorizer path.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
)

func TestPolicyLayerAgainstDB(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	resp, _ := w.post(t, "/v1/auth/signup", `{"email":"pol@example.com","password":"orbit-magnet-11","name":"P"}`, "")
	if resp.StatusCode != 201 {
		t.Fatalf("signup: %d", resp.StatusCode)
	}
	var uid string
	if err := w.pool.QueryRow(ctx, "select id from users where email='pol@example.com'").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	org, err := w.svc.CreateOrgWithOwner(ctx, "polco", uid)
	if err != nil {
		t.Fatal(err)
	}

	check := func(perm rbac.Permission, scope rbac.Scope) error {
		return w.authz.Require(ctx, session.Principal{Kind: "session", UserID: uid}, perm, scope)
	}
	orgScope := rbac.Scope{OrgID: org.ID}

	// No policies: owner ceiling governs alone.
	if err := check("ai.use", orgScope); err != nil {
		t.Fatalf("owner ai.use with no policies: %v", err)
	}

	// Org-wide ai_assistant=disabled narrows ai.* for EVERY role, owner included.
	if _, err := w.pool.Exec(ctx,
		`insert into policies (id, org_id, key, enforcement) values ('pol_org1', $1, 'ai_assistant', 'disabled')`,
		org.ID); err != nil {
		t.Fatal(err)
	}
	err = check("ai.use", orgScope)
	var denied identity.AccessDeniedError
	if !errors.As(err, &denied) || !strings.Contains(denied.DeniedBy, "policy:ai_assistant") {
		t.Fatalf("org-wide disabled did not narrow owner: %v", err)
	}
	// …and does not touch non-ai permissions (tighten is per-key, not blanket).
	if err := check("org.manage", orgScope); err != nil {
		t.Fatalf("policy leaked onto org.manage: %v", err)
	}

	// Closest wins from real rows: project-level 'enabled' overrides in that
	// project only; the org floor holds elsewhere. Projects are real rows
	// since T3.2 (policies.project_id is FK'd).
	orgRow, err := w.svc.GetOrg(ctx, org.ID)
	if err != nil {
		t.Fatal(err)
	}
	prjA, _, err := w.prov.CreateProject(ctx, orgRow, "pol-a", "", uid)
	if err != nil {
		t.Fatal(err)
	}
	prjB, _, err := w.prov.CreateProject(ctx, orgRow, "pol-b", "", uid)
	if err == nil {
		// free plan allows 1 project; the gate must have fired
		t.Fatalf("plan gate did not fire for the second free project: %v", prjB)
	}
	if _, err := w.pool.Exec(ctx, "update orgs set plan='pro' where id=$1", org.ID); err != nil {
		t.Fatal(err)
	}
	orgRow.Plan = "pro"
	prjB, _, err = w.prov.CreateProject(ctx, orgRow, "pol-b", "", uid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx,
		`insert into policies (id, org_id, project_id, key, enforcement) values ('pol_prj1', $1, $2, 'ai_assistant', 'enabled')`,
		org.ID, prjA.ID); err != nil {
		t.Fatal(err)
	}
	if err := check("ai.use", rbac.Scope{OrgID: org.ID, ProjectID: prjA.ID}); err != nil {
		t.Fatalf("project-level enabled did not win in prj_a: %v", err)
	}
	if err := check("ai.use", rbac.Scope{OrgID: org.ID, ProjectID: prjB.ID}); !errors.As(err, &denied) {
		t.Fatalf("org floor ignored in prj_b: %v", err)
	}

	// UNIQUE (org_id, project_id, key): a second org-wide row of the same key
	// must be rejected by the schema, not deduplicated in code.
	if _, err := w.pool.Exec(ctx,
		`insert into policies (id, org_id, key, enforcement) values ('pol_org2', $1, 'ai_assistant', 'enabled')`,
		org.ID); err == nil {
		t.Fatal("duplicate org-wide policy row accepted")
	}
}
