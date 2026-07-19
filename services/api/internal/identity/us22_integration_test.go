package identity_test

// US-2.2 — the acceptance test IS the 11-permissions sentence:
//   allow = matrix[role][perm]==Y AND policies.evaluate(actor, perm,
//           {org,project,env})==permit
// Matrix denials name the missing role; policy denials name the policy;
// BOTH are audited (rows on the spine, visible via /orgs/{org}/audit).

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

func TestUS22TwoLayerAuthorizationSentence(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	_, ownerID := w.signupUser(t, "us22-owner@example.com")
	_, devID := w.signupUser(t, "us22-dev@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "us22co", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.svc.AddMember(ctx, org.ID, devID, "developer", ownerID); err != nil {
		t.Fatal(err)
	}

	matrix, err := rbac.Load()
	if err != nil {
		t.Fatal(err)
	}
	scope := rbac.Scope{OrgID: org.ID}
	users := map[rbac.Role]string{rbac.RoleOwner: ownerID, rbac.RoleDeveloper: devID}

	// --- Layer 1 sweep, no policies attached: allow ⇔ matrix Y --------------
	for _, perm := range matrix.Permissions() {
		if matrix.Delegated(perm) {
			continue
		}
		for role, uid := range users {
			err := w.authz.Require(ctx, session.Principal{Kind: "session", UserID: uid}, perm, scope)
			if matrix.Ceiling(role, perm) != (err == nil) {
				t.Fatalf("sentence violated (no policies): %s × %s → err=%v, matrix=%v",
					role, perm, err, matrix.Ceiling(role, perm))
			}
			var denied identity.AccessDeniedError
			if err != nil && (!errors.As(err, &denied) || !strings.Contains(denied.DeniedBy, "role:"+string(role))) {
				t.Fatalf("matrix denial does not name the role: %v", err)
			}
		}
	}

	// --- Layer 2: attach ai_assistant=disabled — matrix Y AND policy deny ---
	if _, err := w.pool.Exec(ctx,
		`insert into policies (id, org_id, key, enforcement) values ('pol_us22', $1, 'ai-assistant', 'disabled')`,
		org.ID); err != nil {
		t.Fatal(err)
	}
	err = w.authz.Require(ctx, session.Principal{Kind: "session", UserID: ownerID}, "ai.use", scope)
	var denied identity.AccessDeniedError
	if !errors.As(err, &denied) || !strings.HasPrefix(denied.DeniedBy, "policy:ai-assistant") {
		t.Fatalf("policy denial does not name the policy: %v", err)
	}

	// --- both audited: matrix + policy denial rows on the spine -------------
	var matrixDenials, policyDenials int
	if err := w.pool.QueryRow(ctx,
		`select count(*) from events where org_id=$1 and action='authz.denied' and kind='membership'`,
		org.ID).Scan(&matrixDenials); err != nil || matrixDenials == 0 {
		t.Fatalf("matrix denials not audited: %d %v", matrixDenials, err)
	}
	if err := w.pool.QueryRow(ctx,
		`select count(*) from events where org_id=$1 and action='authz.denied' and kind='policy_trigger'
		   and detail->>'denied_by' like 'policy:ai-assistant%'`,
		org.ID).Scan(&policyDenials); err != nil || policyDenials != 1 {
		t.Fatalf("policy denial not audited: %d %v", policyDenials, err)
	}

	// …and visible through the audit view like any other spine fact.
	ownerCk, _ := w.loginCookie(t, "us22-owner@example.com")
	resp, body := w.get(t, "/v1/orgs/"+org.ID+"/audit?action=authz.denied", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, "authz.denied") {
		t.Fatalf("denials not in audit view: %d %s", resp.StatusCode, body)
	}
}

// loginCookie logs an existing user in and returns the session cookie.
func (w *world) loginCookie(t *testing.T, email string) (string, string) {
	t.Helper()
	resp, body := w.post(t, "/v1/auth/login",
		`{"email":"`+email+`","password":"orbit-magnet-11"}`, "")
	if resp.StatusCode != 200 {
		t.Fatalf("login %s: %d %s", email, resp.StatusCode, body)
	}
	return sessionCookie(resp), body
}
