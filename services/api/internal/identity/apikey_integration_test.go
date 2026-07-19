package identity_test

// T7.4 / ADR-0007: org API keys authorize against an explicit permission
// subset of the one canonical matrix — no role, no membership, policies still
// narrow, keys never grant outside their list.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
)

func TestOrgApiKeyAuthorization(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "key-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"keyco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	// --- create requires an explicit non-empty, VALID permission subset -----
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/api-keys", `{"name":"nc","scope":"full"}`, ownerCk)
	if resp.StatusCode != 422 || !strings.Contains(body, "permissions") {
		t.Fatalf("no-permissions create: %d %s", resp.StatusCode, body)
	}
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
		`{"name":"bad","scope":"full","permissions":["not.a.perm"]}`, ownerCk)
	if resp.StatusCode != 422 || !strings.Contains(body, "not a grantable") {
		t.Fatalf("bogus permission: %d %s", resp.StatusCode, body)
	}
	// delegated permissions cannot be granted directly (AI Law 1)
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
		`{"name":"del","scope":"full","permissions":["ai.apply_proposal"]}`, ownerCk)
	if resp.StatusCode != 422 || !strings.Contains(body, "delegated") {
		t.Fatalf("delegated permission: %d %s", resp.StatusCode, body)
	}

	// --- a least-privilege key: deploy.promote + observe.read ONLY ----------
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
		`{"name":"ci","scope":"full","permissions":["deploy.promote","observe.read"]}`, ownerCk)
	if resp.StatusCode != 201 || !strings.Contains(body, `"token":"stp_`) {
		t.Fatalf("createApiKey: %d %s", resp.StatusCode, body)
	}
	var key struct{ Id, Token string }
	_ = json.Unmarshal([]byte(body), &key)

	// resolve the key to a principal and check the evaluator directly
	p, err := w.svc.ResolveBearer(ctx, key.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsOrgKey() || p.OrgID != org.Id {
		t.Fatalf("principal not an org key: %+v", p)
	}
	scope := rbac.Scope{OrgID: org.Id}
	mustAuthz := func(perm rbac.Permission, want bool) {
		err := w.authz.Require(ctx, p, perm, scope)
		if (err == nil) != want {
			t.Fatalf("authz %s: got err=%v want-allowed=%v", perm, err, want)
		}
	}
	mustAuthz("deploy.promote", true)  // granted
	mustAuthz("observe.read", true)    // granted
	mustAuthz("service.delete", false) // not in the subset — denied
	mustAuthz("org.manage", false)     // not in the subset — denied even though owner-only
	// the denial names the key, not a role
	err = w.authz.Require(ctx, p, "service.delete", scope)
	var denied identity.AccessDeniedError
	if !errors.As(err, &denied) || !strings.Contains(denied.DeniedBy, "key:not granted") {
		t.Fatalf("denial should name the key: %v", err)
	}
	// a key cannot act in ANOTHER org's scope
	if err := w.authz.Require(ctx, p, "deploy.promote", rbac.Scope{OrgID: "org_other"}); err == nil {
		t.Fatal("key authorized in a foreign org scope")
	}

	// --- the key authenticates as a bearer over HTTP and is RBAC-gated ------
	// observe.read is granted → the audit endpoint... needs audit.read, which
	// is NOT granted, so 403 naming the key (proves gating, not just auth)
	req, _ := http.NewRequest(http.MethodGet, w.srv.URL+"/v1/orgs/"+org.Id+"/audit", nil)
	req.Header.Set("Authorization", "Bearer "+key.Token)
	res, _ := http.DefaultClient.Do(req)
	rbBytes, _ := io.ReadAll(res.Body)
	res.Body.Close()
	rb := string(rbBytes)
	if res.StatusCode != 403 || !strings.Contains(rb, "key:not granted") {
		t.Fatalf("ungranted HTTP op: %d %s", res.StatusCode, rb)
	}

	// --- policies still NARROW an org key (tighten-only) --------------------
	// grant ai.use to a fresh key, then disable ai_assistant org-wide → denied
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
		`{"name":"ai","scope":"full","permissions":["ai.use"]}`, ownerCk)
	var aiKey struct{ Token string }
	_ = json.Unmarshal([]byte(body), &aiKey)
	pAI, _ := w.svc.ResolveBearer(ctx, aiKey.Token)
	if err := w.authz.Require(ctx, pAI, "ai.use", scope); err != nil {
		t.Fatalf("ai.use granted key denied before policy: %v", err)
	}
	if _, err := w.pool.Exec(ctx,
		`insert into policies (id, org_id, key, enforcement) values ('pol_key', $1, 'ai_assistant', 'disabled')`,
		org.Id); err != nil {
		t.Fatal(err)
	}
	if err := w.authz.Require(ctx, pAI, "ai.use", scope); err == nil {
		t.Fatal("policy did not narrow the org key")
	}

	// --- list shows the granted permissions, never the secret ---------------
	resp, body = w.get(t, "/v1/orgs/"+org.Id+"/api-keys", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, "deploy.promote") || strings.Contains(body, key.Token) {
		t.Fatalf("listApiKeys: %d %s", resp.StatusCode, body)
	}

	// --- revoke: immediate; the key stops authenticating --------------------
	resp, _ = w.del(t, "/v1/orgs/"+org.Id+"/api-keys/"+key.Id, ownerCk)
	if resp.StatusCode != 204 {
		t.Fatalf("revokeApiKey: %d", resp.StatusCode)
	}
	if _, err := w.svc.ResolveBearer(ctx, key.Token); err == nil {
		t.Fatal("revoked key still resolves")
	}
	// re-revoke → 404
	resp, _ = w.del(t, "/v1/orgs/"+org.Id+"/api-keys/"+key.Id, ownerCk)
	if resp.StatusCode != 404 {
		t.Fatalf("re-revoke: %d", resp.StatusCode)
	}
	_ = ownerID
}
