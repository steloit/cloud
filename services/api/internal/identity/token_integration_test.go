package identity_test

// T2.2 full-stack: reveal-once minting, hash-only storage, bearer auth for
// BOTH credential kinds (org keys seeded at store level — the endpoints land
// with the org tasks), scope enforcement, revocation, expiry.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

func (w *world) req(t *testing.T, method, path, body string, hdr map[string]string) (*http.Response, string) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, w.srv.URL+path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b)
}

func TestPersonalTokenLifecycle(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	resp, _ := w.post(t, "/v1/auth/signup", `{"email":"tok@example.com","password":"orbit-magnet-11","name":"Tok"}`, "")
	if resp.StatusCode != 201 {
		t.Fatalf("signup: %d", resp.StatusCode)
	}
	ck := sessionCookie(resp)

	// --- unauthenticated create → 401 --------------------------------------
	resp, _ = w.req(t, "POST", "/v1/me/tokens", `{"name":"ci"}`, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("unauth create: %d", resp.StatusCode)
	}

	// --- create (session): 201 reveal-once ----------------------------------
	resp, body := w.req(t, "POST", "/v1/me/tokens", `{"name":"laptop","scope":"full"}`, map[string]string{"Cookie": ck})
	if resp.StatusCode != 201 {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(body), &m)
	secret := m["token"].(string)
	if !strings.HasPrefix(secret, "stp_") || m["shown_once"] != true || m["hash_stored"] != true {
		t.Fatalf("reveal-once contract violated: %s", body)
	}
	// DB: hash stored, plaintext absent
	var n int
	if err := w.pool.QueryRow(ctx, "select count(*) from tokens where token_hash=$1", session.HashToken(secret)).Scan(&n); err != nil || n != 1 {
		t.Fatalf("token hash row: n=%d err=%v", n, err)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from tokens where prefix=$1", secret).Scan(&n); err != nil || n != 0 {
		t.Fatal("full secret stored in prefix column")
	}

	// --- list: prefix + metadata, never the secret ---------------------------
	resp, body = w.req(t, "GET", "/v1/me/tokens", "", map[string]string{"Cookie": ck})
	if resp.StatusCode != 200 || strings.Contains(body, secret) {
		t.Fatalf("list leaked secret or failed: %d %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, secret[:12]) {
		t.Fatalf("list missing prefix: %s", body)
	}

	// --- bearer auth (personal, full): list via token ------------------------
	bearer := map[string]string{"Authorization": "Bearer " + secret}
	resp, _ = w.req(t, "GET", "/v1/me/tokens", "", bearer)
	if resp.StatusCode != 200 {
		t.Fatalf("bearer list: %d", resp.StatusCode)
	}

	// --- read_only scope: can list, cannot mint ------------------------------
	resp, body = w.req(t, "POST", "/v1/me/tokens", `{"name":"ro"}`, map[string]string{"Cookie": ck})
	if resp.StatusCode != 201 {
		t.Fatalf("ro create: %d", resp.StatusCode)
	}
	_ = json.Unmarshal([]byte(body), &m)
	roSecret := m["token"].(string)
	roBearer := map[string]string{"Authorization": "Bearer " + roSecret}
	resp, _ = w.req(t, "GET", "/v1/me/tokens", "", roBearer)
	if resp.StatusCode != 200 {
		t.Fatalf("ro bearer list: %d", resp.StatusCode)
	}
	resp, body = w.req(t, "POST", "/v1/me/tokens", `{"name":"nope"}`, roBearer)
	if resp.StatusCode != 403 || !strings.Contains(body, "read_only") {
		t.Fatalf("ro bearer mint not denied: %d %s", resp.StatusCode, body)
	}

	// --- revoke: 204; bearer stops working; foreign/missing id → 404 --------
	tokID := m["id"].(string)
	resp, _ = w.req(t, "DELETE", "/v1/me/tokens/"+tokID, "", map[string]string{"Cookie": ck})
	if resp.StatusCode != 204 {
		t.Fatalf("revoke: %d", resp.StatusCode)
	}
	resp, _ = w.req(t, "GET", "/v1/me/tokens", "", roBearer)
	if resp.StatusCode != 401 {
		t.Fatalf("revoked bearer still works: %d", resp.StatusCode)
	}
	resp, _ = w.req(t, "DELETE", "/v1/me/tokens/tok_doesnotexist", "", map[string]string{"Cookie": ck})
	if resp.StatusCode != 404 {
		t.Fatalf("missing token revoke: %d", resp.StatusCode)
	}

	// --- expired token → 401 -------------------------------------------------
	var uid string
	_ = w.pool.QueryRow(ctx, "select id from users where email='tok@example.com'").Scan(&uid)
	expSecret := "stp_expiredtokenexpiredtokenexpiredtoken12345678"
	_, err := store.New(w.pool).CreateToken(ctx, store.CreateTokenParams{
		ID: ids.New("tok"), Kind: "personal",
		UserID: pgtype.Text{String: uid, Valid: true},
		Name:   "expired", Scope: "full", Prefix: expSecret[:12],
		TokenHash: session.HashToken(expSecret),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, _ = w.req(t, "GET", "/v1/me/tokens", "", map[string]string{"Authorization": "Bearer " + expSecret})
	if resp.StatusCode != 401 {
		t.Fatalf("expired bearer accepted: %d", resp.StatusCode)
	}

	// --- BOTH KINDS (T2.2 AC): org key seeded at store level authenticates ---
	// T2.3's migration added the tokens.org_id FK — the org row must exist
	// (this line is the fix for the FK violation CI caught on #210).
	seededOrg, err := store.New(w.pool).CreateOrg(ctx, store.CreateOrgParams{ID: ids.New("org"), Name: "seeded"})
	if err != nil {
		t.Fatal(err)
	}
	orgSecret := "stk_orgkeyorgkeyorgkeyorgkeyorgkey1234567890ab"
	_, err = store.New(w.pool).CreateToken(ctx, store.CreateTokenParams{
		ID: ids.New("key"), Kind: "org",
		OrgID: pgtype.Text{String: seededOrg.ID, Valid: true},
		Name:  "automation", Scope: "read_only", Prefix: orgSecret[:12],
		TokenHash: session.HashToken(orgSecret),
	})
	if err != nil {
		t.Fatal(err)
	}
	// An org key has no user → /me/tokens is 401 for it (user-scoped surface),
	// which proves the middleware resolved and classified it rather than
	// rejecting the credential outright: a bogus secret must ALSO be 401 but
	// the DB row's last_used_at moves only for the real key.
	resp, _ = w.req(t, "GET", "/v1/me/tokens", "", map[string]string{"Authorization": "Bearer " + orgSecret})
	if resp.StatusCode != 401 {
		t.Fatalf("org-key on user surface: %d", resp.StatusCode)
	}
	var used bool
	if err := w.pool.QueryRow(ctx, "select last_used_at is not null from tokens where kind='org'").Scan(&used); err != nil || !used {
		t.Fatalf("org key was not resolved by bearer middleware (last_used_at empty): %v", err)
	}
}
