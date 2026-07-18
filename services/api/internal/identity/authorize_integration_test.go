package identity_test

// T2.3: the two-layer evaluator against real membership rows.

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

func TestAuthorizeAgainstMembership(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	// users via the real signup path
	mk := func(email string) string {
		resp, _ := w.post(t, "/v1/auth/signup", `{"email":"`+email+`","password":"orbit-magnet-11","name":"U"}`, "")
		if resp.StatusCode != 201 {
			t.Fatalf("signup %s: %d", email, resp.StatusCode)
		}
		var id string
		if err := w.pool.QueryRow(ctx, "select id from users where email=$1", email).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	owner := mk("owner@example.com")
	dev := mk("dev@example.com")
	billing := mk("bill@example.com")
	outsider := mk("out@example.com")

	org, err := w.svc.CreateOrgWithOwner(ctx, "acme", owner)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(org.ID, "org_") {
		t.Fatalf("org id not prefixed: %s", org.ID)
	}
	if err := w.svc.AddMember(ctx, org.ID, dev, "developer", owner); err != nil {
		t.Fatal(err)
	}
	if err := w.svc.AddMember(ctx, org.ID, billing, "billing", owner); err != nil {
		t.Fatal(err)
	}

	scope := rbac.Scope{OrgID: org.ID}
	check := func(userID string, perm rbac.Permission) error {
		return w.authz.Require(ctx, session.Principal{Kind: "session", UserID: userID}, perm, scope)
	}

	// owner ceiling: org.manage Y
	if err := check(owner, "org.manage"); err != nil {
		t.Fatalf("owner org.manage: %v", err)
	}
	// developer: project.create Y, org.manage N — denial names the role
	if err := check(dev, "project.create"); err != nil {
		t.Fatalf("dev project.create: %v", err)
	}
	err = check(dev, "org.manage")
	var denied identity.AccessDeniedError
	if !errors.As(err, &denied) || !strings.Contains(denied.DeniedBy, "role:developer") {
		t.Fatalf("dev org.manage denial wrong: %v", err)
	}
	// billing: billing.view Y, service.create N
	if err := check(billing, "billing.view"); err != nil {
		t.Fatalf("billing billing.view: %v", err)
	}
	if err := check(billing, "service.create"); !errors.As(err, &denied) {
		t.Fatalf("billing service.create not denied: %v", err)
	}
	// outsider: membership denial names membership, not a role
	err = check(outsider, "project.create")
	if !errors.As(err, &denied) || !strings.Contains(denied.DeniedBy, "membership:none") {
		t.Fatalf("outsider denial wrong: %v", err)
	}
	// duplicate membership rejected by the unique constraint
	if err := w.svc.AddMember(ctx, org.ID, dev, "admin", owner); err == nil {
		t.Fatal("duplicate membership accepted")
	}
}
