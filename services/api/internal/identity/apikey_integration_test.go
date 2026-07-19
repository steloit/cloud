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
		`insert into policies (id, org_id, key, enforcement) values ('pol_key', $1, 'ai-assistant', 'disabled')`,
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

// The delegation ceiling (ADR-0007, reviewer finding): a minter can never
// grant a permission it does not itself hold — no admin→owner escalation.
func TestOrgKeyDelegationCeiling(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "dc-owner@example.com")
	adminCk, adminID := w.signupUser(t, "dc-admin@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"dcco"}`, ownerCk)
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	if err := w.svc.AddMember(ctx, org.Id, adminID, "admin", ownerID); err != nil {
		t.Fatal(err)
	}

	// admin holds api_keys.manage but NOT org.delete (owner-only) → cannot
	// grant it (would be an admin→owner escalation)
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
		`{"name":"esc","scope":"full","permissions":["org.delete"]}`, adminCk)
	if resp.StatusCode != 422 || !strings.Contains(body, "do not hold it") {
		t.Fatalf("admin granting owner-only perm: %d %s", resp.StatusCode, body)
	}
	// admin CAN grant a permission it holds (project.create is admin-Y)
	resp, _ = w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
		`{"name":"ok","scope":"full","permissions":["project.create"]}`, adminCk)
	if resp.StatusCode != 201 {
		t.Fatalf("admin granting held perm: %d", resp.StatusCode)
	}
	// the owner CAN grant org.delete (holds it)
	resp, _ = w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
		`{"name":"own","scope":"full","permissions":["org.delete"]}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("owner granting owner-only perm: %d", resp.StatusCode)
	}
}

// Attack-class inputs at create time all fail closed (the matrix lookup is
// exact + case-sensitive; empties/dupes are handled).
func TestOrgKeyMalformedPermissions(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "mp-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"mpco"}`, ownerCk)
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	for _, bad := range []string{
		`["  "]`,                        // whitespace-only
		`["deploy.promote "]`,           // trailing space (not canonical)
		`["Deploy.Promote"]`,            // case-variant
		`["deploy.promote",""]`,         // an empty element among valid
		`[""]`,                          // empty element
	} {
		resp, body = w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
			`{"name":"x","scope":"full","permissions":`+bad+`}`, ownerCk)
		if resp.StatusCode != 422 {
			t.Fatalf("malformed %s accepted: %d %s", bad, resp.StatusCode, body)
		}
	}

	// duplicates are deduped, not rejected (no escalation; canonical storage)
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
		`{"name":"dup","scope":"full","permissions":["observe.read","observe.read"]}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("dupe perms: %d %s", resp.StatusCode, body)
	}
	var key struct{ Id string }
	_ = json.Unmarshal([]byte(body), &key)
	var stored []string
	if err := w.pool.QueryRow(ctx, "select permissions from tokens where id=$1", key.Id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 {
		t.Fatalf("permissions not deduped: %v", stored)
	}
	_ = ownerID

	// a read_only org key cannot perform a mutating op even if it holds the
	// permission — the coarse scope ceiling ∩ the permission subset
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
		`{"name":"ro","scope":"read_only","permissions":["project.create"]}`, ownerCk)
	var roKey struct{ Token string }
	_ = json.Unmarshal([]byte(body), &roKey)
	req, _ := http.NewRequest(http.MethodPost, w.srv.URL+"/v1/orgs/"+org.Id+"/projects",
		strings.NewReader(`{"name":"nope"}`))
	req.Header.Set("Authorization", "Bearer "+roKey.Token)
	req.Header.Set("Content-Type", "application/json")
	res, _ := http.DefaultClient.Do(req)
	rbBytes, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 403 || !strings.Contains(string(rbBytes), "read_only") {
		t.Fatalf("read_only key mutating: %d %s", res.StatusCode, string(rbBytes))
	}
}

// Fail-closed for the un-guarded mint path is now CONTRACTUAL (MintOrgKey
// rejects empty), and a personal token is never misclassified as an org key.
func TestOrgKeyMintGuardsAndPrincipalKind(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	_, ownerID := w.signupUser(t, "mg-owner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "mgco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	// service-layer guard: no empty-permission org key can be minted
	if _, err := w.svc.MintOrgKey(ctx, org.ID, "empty", "full", ownerID, nil, 90); err == nil {
		t.Fatal("MintOrgKey accepted an empty permission list")
	}
	// a personal token carries a UserID → never an org key, even though
	// ResolveBearer now populates OrgID for org tokens
	minted, err := w.svc.MintPersonalToken(ctx, ownerID, "pt", "full", 90)
	if err != nil {
		t.Fatal(err)
	}
	p, err := w.svc.ResolveBearer(ctx, minted.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if p.IsOrgKey() {
		t.Fatal("a personal token was misclassified as an org key")
	}
}
