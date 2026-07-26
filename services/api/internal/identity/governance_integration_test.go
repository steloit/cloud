package identity_test

// T2.7: org / member / invite governance against real Postgres.
// QA scenario 8: expire at 7d; renew notifies inviter; accept from wrong
// email blocked; already-used → "sign in" path.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func (w *world) patch(t *testing.T, path, body, cookie string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPatch, w.srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b)
}

func (w *world) put(t *testing.T, path, body, cookie string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, w.srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b)
}

func (w *world) del(t *testing.T, path, cookie string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, w.srv.URL+path, nil)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b)
}

func (w *world) signupUser(t *testing.T, email string) (cookie, userID string) {
	t.Helper()
	resp, body := w.post(t, "/v1/auth/signup",
		fmt.Sprintf(`{"email":%q,"password":"orbit-magnet-11","name":"U"}`, email), "")
	if resp.StatusCode != 201 {
		t.Fatalf("signup %s: %d %s", email, resp.StatusCode, body)
	}
	if err := w.pool.QueryRow(context.Background(), "select id from users where email=$1", email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	return sessionCookie(resp), userID
}

func TestOrgGovernance(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "gov-owner@example.com")

	// --- createOrg: 201, slug derived + immutable, subscription row, events -
	resp, body := w.post(t, "/v1/orgs", `{"name":"Gov Co"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct {
		Id, Slug, Name, Plan string
	}
	_ = json.Unmarshal([]byte(body), &org)
	if org.Slug != "gov-co" || org.Plan != "free" {
		t.Fatalf("org shape: %+v", org)
	}
	var subs int
	if err := w.pool.QueryRow(ctx, "select count(*) from subscriptions where org_id=$1", org.Id).Scan(&subs); err != nil || subs != 1 {
		t.Fatalf("subscription row: %d %v", subs, err)
	}
	// duplicate slug → 409
	resp, body = w.post(t, "/v1/orgs", `{"name":"gov co"}`, ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "slug") {
		t.Fatalf("dup slug: %d %s", resp.StatusCode, body)
	}

	// --- listMyOrgs / getOrg / updateOrg (slug never changes) ---------------
	resp, body = w.get(t, "/v1/orgs", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, org.Id) {
		t.Fatalf("listMyOrgs: %d %s", resp.StatusCode, body)
	}
	resp, body = w.patch(t, "/v1/orgs/"+org.Id, `{"name":"Gov Company"}`, ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, `"slug":"gov-co"`) || !strings.Contains(body, "Gov Company") {
		t.Fatalf("updateOrg: %d %s", resp.StatusCode, body)
	}

	// outsiders can't even see the org (404, not 403 — no id probing)
	strangerCk, _ := w.signupUser(t, "gov-stranger@example.com")
	resp, _ = w.get(t, "/v1/orgs/"+org.Id, strangerCk)
	if resp.StatusCode != 404 {
		t.Fatalf("stranger getOrg: %d", resp.StatusCode)
	}

	// --- members: list carries seats (free = 3 included) --------------------
	resp, body = w.get(t, "/v1/orgs/"+org.Id+"/members", ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("listMembers: %d %s", resp.StatusCode, body)
	}
	var ml struct {
		Data  []struct{ Id, Role string }
		Seats struct {
			Included          int `json:"included"`
			Used              int `json:"used"`
			OveragePriceCents int `json:"overage_price_cents"`
		}
	}
	_ = json.Unmarshal([]byte(body), &ml)
	if ml.Seats.Included != 3 || ml.Seats.Used != 1 || ml.Seats.OveragePriceCents != 700 {
		t.Fatalf("seats: %+v", ml.Seats)
	}

	// --- invite lifecycle (QA 8) --------------------------------------------
	// invite → accept from the RIGHT account
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/invites", `{"email":"gov-dev@example.com","role":"developer"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createInvite: %d %s", resp.StatusCode, body)
	}
	var inv struct{ Id, Status string }
	_ = json.Unmarshal([]byte(body), &inv)
	if inv.Status != "pending" || !strings.HasPrefix(inv.Id, "inv_") {
		t.Fatalf("invite: %+v", inv)
	}

	// dedupe: same email again → 409
	resp, _ = w.post(t, "/v1/orgs/"+org.Id+"/invites", `{"email":"gov-dev@example.com","role":"admin"}`, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("dup invite: %d", resp.StatusCode)
	}

	// public view: no auth, masked email, role consequences
	resp, body = w.get(t, "/v1/invites/"+inv.Id, "")
	if resp.StatusCode != 200 || !strings.Contains(body, "g***@example.com") || !strings.Contains(body, "role_consequences") {
		t.Fatalf("getInvitePublic: %d %s", resp.StatusCode, body)
	}

	// wrong-account acceptance blocked (403)
	resp, _ = w.post(t, "/v1/invites/"+inv.Id, "", strangerCk)
	if resp.StatusCode != 403 {
		t.Fatalf("wrong-account accept: %d", resp.StatusCode)
	}

	// right account accepts; member exists; invite used up
	devCk, devID := w.signupUser(t, "gov-dev@example.com")
	resp, _ = w.post(t, "/v1/invites/"+inv.Id, "", devCk)
	if resp.StatusCode != 200 {
		t.Fatalf("accept: %d", resp.StatusCode)
	}
	var role string
	if err := w.pool.QueryRow(ctx, "select role from members where org_id=$1 and user_id=$2", org.Id, devID).Scan(&role); err != nil || role != "developer" {
		t.Fatalf("membership after accept: %s %v", role, err)
	}
	// already-used → 409 with the "sign in" path
	resp, body = w.post(t, "/v1/invites/"+inv.Id, "", devCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "sign in") {
		t.Fatalf("re-accept: %d %s", resp.StatusCode, body)
	}

	// invited-but-already-member → 409 "already a member"
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/invites", `{"email":"gov-dev@example.com","role":"admin"}`, ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "already a member") {
		t.Fatalf("invite existing member: %d %s", resp.StatusCode, body)
	}

	// expiry at 7d: age a pending invite past expires_at → public view 410,
	// renew notifies the inviter (spine event)
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/invites", `{"email":"gov-late@example.com","role":"developer"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createInvite late: %d %s", resp.StatusCode, body)
	}
	var late struct{ Id string }
	_ = json.Unmarshal([]byte(body), &late)
	if _, err := w.pool.Exec(ctx, "update invites set expires_at = now() - interval '1 day' where id=$1", late.Id); err != nil {
		t.Fatal(err)
	}
	resp, _ = w.get(t, "/v1/invites/"+late.Id, "")
	if resp.StatusCode != 410 {
		t.Fatalf("expired public view: %d", resp.StatusCode)
	}
	resp, _ = w.post(t, "/v1/invites/"+late.Id+"/renew", "", "")
	if resp.StatusCode != 202 {
		t.Fatalf("renew: %d", resp.StatusCode)
	}
	var n int
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='invite.renewal_requested'", org.Id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("renewal event: %d %v", n, err)
	}

	// decline invalidates and notifies (spine event)
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/invites", `{"email":"gov-decline@example.com","role":"developer"}`, ownerCk)
	var dec struct{ Id string }
	_ = json.Unmarshal([]byte(body), &dec)
	resp, _ = w.del(t, "/v1/invites/"+dec.Id, "")
	if resp.StatusCode != 204 {
		t.Fatalf("decline: %d", resp.StatusCode)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='invite.declined'", org.Id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("decline event: %d %v", n, err)
	}

	// seat overage: fill to 3 seats then invite #4 → 402 with price; confirm=true proceeds
	inviteAccept := func(email string) {
		resp, body := w.post(t, "/v1/orgs/"+org.Id+"/invites?confirm=true", fmt.Sprintf(`{"email":%q,"role":"developer"}`, email), ownerCk)
		if resp.StatusCode != 201 {
			t.Fatalf("invite %s: %d %s", email, resp.StatusCode, body)
		}
		var i struct{ Id string }
		_ = json.Unmarshal([]byte(body), &i)
		ck, _ := w.signupUser(t, email)
		if r, _ := w.post(t, "/v1/invites/"+i.Id, "", ck); r.StatusCode != 200 {
			t.Fatalf("accept %s failed", email)
		}
	}
	inviteAccept("gov-3@example.com") // seat 3 of 3
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/invites", `{"email":"gov-4@example.com","role":"developer"}`, ownerCk)
	if resp.StatusCode != 402 || !strings.Contains(body, "overage_price_cents") || !strings.Contains(body, "700") {
		t.Fatalf("seat 402: %d %s", resp.StatusCode, body)
	}
	resp, _ = w.post(t, "/v1/orgs/"+org.Id+"/invites?confirm=true", `{"email":"gov-4@example.com","role":"developer"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("seat confirm: %d", resp.StatusCode)
	}

	// --- role change: audited before→after; last owner protected -----------
	var devMemberID string
	if err := w.pool.QueryRow(ctx, "select id from members where org_id=$1 and user_id=$2", org.Id, devID).Scan(&devMemberID); err != nil {
		t.Fatal(err)
	}
	resp, body = w.patch(t, "/v1/orgs/"+org.Id+"/members/"+devMemberID, `{"role":"admin"}`, ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, `"role":"admin"`) {
		t.Fatalf("changeRole: %d %s", resp.StatusCode, body)
	}
	if err := w.pool.QueryRow(ctx, `select count(*) from events where org_id=$1 and action='member.role_changed' and detail->>'before'='developer' and detail->>'after'='admin'`, org.Id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("role-change audit: %d %v", n, err)
	}
	var ownerMemberID string
	if err := w.pool.QueryRow(ctx, "select id from members where org_id=$1 and user_id=$2", org.Id, ownerID).Scan(&ownerMemberID); err != nil {
		t.Fatal(err)
	}
	resp, body = w.patch(t, "/v1/orgs/"+org.Id+"/members/"+ownerMemberID, `{"role":"developer"}`, ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "owner") {
		t.Fatalf("last-owner demote: %d %s", resp.StatusCode, body)
	}
	resp, body = w.del(t, "/v1/orgs/"+org.Id+"/members/"+ownerMemberID, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("last-owner remove: %d %s", resp.StatusCode, body)
	}

	// --- G6 removal semantics: sessions + personal tokens die ---------------
	resp, body = w.post(t, "/v1/me/tokens", `{"name":"devtok","scope":"full"}`, devCk)
	if resp.StatusCode != 201 {
		t.Fatalf("dev token: %d %s", resp.StatusCode, body)
	}
	resp, body = w.del(t, "/v1/orgs/"+org.Id+"/members/"+devMemberID, ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, "flagged_resources") {
		t.Fatalf("removeMember: %d %s", resp.StatusCode, body)
	}
	resp, _ = w.get(t, "/v1/auth/session", devCk) // session revoked
	if resp.StatusCode != 401 {
		t.Fatalf("removed member session survives: %d", resp.StatusCode)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from tokens where kind='personal' and user_id=$1 and revoked_at is null", devID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("removed member tokens survive: %d %v", n, err)
	}

	// --- org API keys (G8): reveal-once, listed hash-only, RBAC-gated -------
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/api-keys", `{"name":"ci","scope":"read_only","permissions":["observe.read"]}`, ownerCk)
	if resp.StatusCode != 201 || !strings.Contains(body, `"token":"stp_`) {
		t.Fatalf("createApiKey: %d %s", resp.StatusCode, body)
	}
	var key struct{ Token string }
	_ = json.Unmarshal([]byte(body), &key)
	resp, body = w.get(t, "/v1/orgs/"+org.Id+"/api-keys", ownerCk)
	if resp.StatusCode != 200 || strings.Contains(body, key.Token) {
		t.Fatalf("listApiKeys leaked secret: %d", resp.StatusCode)
	}
	// the minted org key authenticates as a bearer
	req, _ := http.NewRequest(http.MethodGet, w.srv.URL+"/v1/orgs/"+org.Id+"/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+key.Token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	// org keys have no membership row: RBAC denies them beyond auth — that
	// scoping model is the recorded follow-up, but auth itself must work
	if res.StatusCode == 401 {
		t.Fatalf("org key failed to authenticate: %d", res.StatusCode)
	}

	// --- deleteOrg: 202 scheduled; repeat → 409 -----------------------------
	resp, _ = w.del(t, "/v1/orgs/"+org.Id, ownerCk)
	if resp.StatusCode != 202 {
		t.Fatalf("deleteOrg: %d", resp.StatusCode)
	}
	resp, _ = w.del(t, "/v1/orgs/"+org.Id, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("re-deleteOrg: %d", resp.StatusCode)
	}
}
