package identity_test

// T7.6: leave-org + account deletion — last-owner rule holds, self-deletion
// never plan-gated, grace-window scheduling, sessions revoked on delete.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLeaveOrgAndAccountDeletion(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "acc-owner@example.com")
	memberCk, memberID := w.signupUser(t, "acc-member@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"accco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	if err := w.svc.AddMember(ctx, org.Id, memberID, "developer", ownerID); err != nil {
		t.Fatal(err)
	}

	// --- leave-org: a member leaves; account + session untouched -----------
	resp, _ = w.post(t, "/v1/orgs/"+org.Id+"/leave", "", memberCk)
	if resp.StatusCode != 204 {
		t.Fatalf("leave: %d", resp.StatusCode)
	}
	// the member's session still works (leaving one org ≠ signed out)
	resp, _ = w.get(t, "/v1/auth/session", memberCk)
	if resp.StatusCode != 200 {
		t.Fatalf("member signed out by leaving: %d", resp.StatusCode)
	}
	var left int
	if err := w.pool.QueryRow(ctx, "select count(*) from members where org_id=$1 and user_id=$2", org.Id, memberID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("membership survived leave: %d", left)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='member.left'", org.Id).Scan(&left); err != nil || left != 1 {
		t.Fatalf("leave event: %d", left)
	}

	// --- the last owner cannot leave (F1) ----------------------------------
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/leave", "", ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "owner") {
		t.Fatalf("last-owner leave: %d %s", resp.StatusCode, body)
	}

	// --- account deletion: sole owner blocked, naming the org --------------
	resp, body = w.del(t, "/v1/me", ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "accco") || !strings.Contains(body, "sole owner") {
		t.Fatalf("sole-owner delete: %d %s", resp.StatusCode, body)
	}

	// a plain user (no orgs) deletes fine — NEVER plan-gated -----------------
	loneCk, loneID := w.signupUser(t, "acc-lone@example.com")
	resp, _ = w.del(t, "/v1/me", loneCk)
	if resp.StatusCode != 202 {
		t.Fatalf("lone delete: %d", resp.StatusCode)
	}
	// scheduled (grace window), session revoked immediately
	var scheduled bool
	if err := w.pool.QueryRow(ctx, "select deletion_scheduled_at is not null from users where id=$1", loneID).Scan(&scheduled); err != nil || !scheduled {
		t.Fatalf("deletion not scheduled: %v %v", scheduled, err)
	}
	resp, _ = w.get(t, "/v1/auth/session", loneCk)
	if resp.StatusCode != 401 {
		t.Fatalf("session survived account deletion: %d", resp.StatusCode)
	}

	// --- the sole owner can delete after transferring ownership away -------
	// promote the (still-member) owner path: add a second owner, then delete
	_, coOwnerID := w.signupUser(t, "acc-co@example.com")
	if err := w.svc.AddMember(ctx, org.Id, coOwnerID, "owner", ownerID); err != nil {
		t.Fatal(err)
	}
	resp, _ = w.del(t, "/v1/me", ownerCk)
	if resp.StatusCode != 202 {
		t.Fatalf("delete after co-owner added: %d", resp.StatusCode)
	}
}
