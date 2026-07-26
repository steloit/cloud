package identity_test

// US-2.3 — QA scenario 6: create → plaintext exactly once; GET returns
// prefix + metadata only; a role shrink narrows the LIVE token immediately
// (tokens act as their user and re-evaluate against current roles at use
// time — no re-issue, no propagation delay).

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
)

func TestUS23TokenRevealOnceAndLiveShrink(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "us23-owner@example.com")
	adminCk, adminID := w.signupUser(t, "us23-admin@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "us23co", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.svc.AddMember(ctx, org.ID, adminID, "admin", ownerID); err != nil {
		t.Fatal(err)
	}

	// --- create → plaintext exactly once ------------------------------------
	resp, body := w.post(t, "/v1/me/tokens", `{"name":"us23","scope":"full"}`, adminCk)
	if resp.StatusCode != 201 {
		t.Fatalf("mint: %d %s", resp.StatusCode, body)
	}
	var minted struct {
		Id, Token, Prefix string
		ShownOnce         bool `json:"shown_once"`
		HashStored        bool `json:"hash_stored"`
	}
	if err := json.Unmarshal([]byte(body), &minted); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(minted.Token, "stp_") || !minted.ShownOnce || !minted.HashStored {
		t.Fatalf("reveal contract: %+v", minted)
	}

	// GET returns prefix + metadata only — the plaintext never appears again
	resp, body = w.get(t, "/v1/me/tokens", adminCk)
	if resp.StatusCode != 200 || strings.Contains(body, minted.Token) || !strings.Contains(body, minted.Prefix) {
		t.Fatalf("list leaked or lost prefix: %d %s", resp.StatusCode, body)
	}
	// …and the DB holds a hash, never the plaintext
	var n int
	if err := w.pool.QueryRow(ctx, "select count(*) from tokens where prefix=$1 and token_hash=$2",
		minted.Prefix, []byte(minted.Token)).Scan(&n); err != nil || n != 0 {
		t.Fatal("plaintext stored as hash column value")
	}

	// --- the token ACTS AS ITS USER with the CURRENT role -------------------
	tokenPrincipal := func() session.Principal {
		p, err := w.svc.ResolveBearer(ctx, minted.Token)
		if err != nil {
			t.Fatalf("bearer resolve: %v", err)
		}
		return p
	}
	scope := rbac.Scope{OrgID: org.ID}
	if err := w.authz.Require(ctx, tokenPrincipal(), "members.invite", scope); err != nil {
		t.Fatalf("admin token members.invite: %v", err)
	}

	// …and over HTTP: the bearer can list invites (members.invite Y for admin)
	bearerGet := func(path string) int {
		req, _ := http.NewRequest(http.MethodGet, w.srv.URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+minted.Token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := bearerGet("/v1/orgs/" + org.ID + "/invites"); code != 200 {
		t.Fatalf("bearer list invites before demotion: %d", code)
	}

	// --- demotion shrinks the LIVE token immediately ------------------------
	var adminMemberID string
	if err := w.pool.QueryRow(ctx, "select id from members where org_id=$1 and user_id=$2", org.ID, adminID).Scan(&adminMemberID); err != nil {
		t.Fatal(err)
	}
	resp, body = w.patch(t, "/v1/orgs/"+org.ID+"/members/"+adminMemberID, `{"role":"billing"}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("demote: %d %s", resp.StatusCode, body)
	}
	err = w.authz.Require(ctx, tokenPrincipal(), "members.invite", scope)
	var denied identity.AccessDeniedError
	if err == nil || !strings.Contains(err.Error(), "role:billing") {
		t.Fatalf("live token did not shrink: %v", err)
	}
	_ = denied
	if code := bearerGet("/v1/orgs/" + org.ID + "/invites"); code != 403 {
		t.Fatalf("bearer list invites after demotion: %d (want 403)", code)
	}
	// …while permissions billing retains keep working (shrink, not kill)
	if err := w.authz.Require(ctx, tokenPrincipal(), "billing.view", scope); err != nil {
		t.Fatalf("billing.view after demotion: %v", err)
	}
}
