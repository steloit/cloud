package identity_test

// T10.3: the routing matrix against a real DB — the bell projection gated by
// per-user prefs (routing never gags a recording), and signed, idempotent,
// SSRF-guarded webhook delivery with an inline test.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/notify"
)

func TestNotifyRouting(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)

	ownerCk, ownerID := w.signupUser(t, "ntf-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"ntfco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var orgID string
	if err := w.pool.QueryRow(ctx, "select id from orgs where slug='ntfco'").Scan(&orgID); err != nil {
		t.Fatal(err)
	}

	// email OFF, bell ON (a partial pref update never flips the unmentioned one).
	if r, b := w.patch(t, "/v1/me/notification-prefs", `{"channels":{"email":false,"inapp":true}}`, ownerCk); r.StatusCode != 200 {
		t.Fatalf("set prefs: %d %s", r.StatusCode, b)
	}
	// PATCH merge: a quiet-hours-only update must NOT silently re-enable email.
	if r, _ := w.patch(t, "/v1/me/notification-prefs", `{"quiet_hours":{"start":"22:00","end":"07:00","timezone":"UTC"}}`, ownerCk); r.StatusCode != 200 {
		t.Fatal("patch quiet hours")
	}
	if _, pb := w.get(t, "/v1/me/notification-prefs", ownerCk); !strings.Contains(pb, `"email":false`) {
		t.Fatalf("quiet-hours PATCH silently flipped email back on: %s", pb)
	}

	// a recording EmailSender proves the email route is GATED by the pref.
	fake := &recordingSender{}
	router := notify.NewRouter(q, w.kek).WithEmail(fake)

	if err := router.Notify(ctx, notify.NotifyInput{
		OrgID: orgID, Kind: "lifecycle", Title: "Deploy finished", Body: "svc_x is ready", Link: "/x",
	}); err != nil {
		t.Fatalf("notify: %v", err)
	}

	// bell present (routing happened) …
	_, lb := w.get(t, "/v1/me/notifications", ownerCk)
	if !strings.Contains(lb, "Deploy finished") {
		t.Fatalf("bell missing the notification: %s", lb)
	}
	// … but the email route was suppressed by the pref (recording ≠ routing).
	if fake.count() != 0 {
		t.Fatalf("email route fired despite email pref off (%d calls)", fake.count())
	}

	// flip email on → the next projection DOES route to email.
	if r, _ := w.patch(t, "/v1/me/notification-prefs", `{"channels":{"email":true,"inapp":true}}`, ownerCk); r.StatusCode != 200 {
		t.Fatal("re-enable email prefs")
	}
	if err := router.Notify(ctx, notify.NotifyInput{OrgID: orgID, Kind: "lifecycle", Title: "Second"}); err != nil {
		t.Fatal(err)
	}
	if fake.count() != 1 {
		t.Fatalf("email route did not fire with email on: %d", fake.count())
	}

	// mark-read is owner-scoped: a foreign notification is never touched.
	var otherUser string
	_ = w.pool.QueryRow(ctx, "select id from users where email=$1", "ntf-owner@example.com").Scan(&otherUser)
	// insert a bell row owned by a DIFFERENT (synthetic) user id.
	if _, err := w.pool.Exec(ctx,
		`insert into users (id, email, password_hash, name) values ('usr_foreign','foreign@example.com','x','F')`); err != nil {
		t.Fatal(err)
	}
	if _, err := q.InsertNotification(ctx, store.InsertNotificationParams{
		ID: "ntf_foreign", UserID: "usr_foreign", Kind: "lifecycle", Title: "not yours",
	}); err != nil {
		t.Fatal(err)
	}
	// owner marks BOTH their own and the foreign id read.
	var ownID string
	_ = w.pool.QueryRow(ctx, "select id from notifications where user_id=$1 limit 1", ownerID).Scan(&ownID)
	if r, b := w.post(t, "/v1/me/notifications:read", `{"ids":["`+ownID+`","ntf_foreign"]}`, ownerCk); r.StatusCode != 204 {
		t.Fatalf("mark-read: %d %s", r.StatusCode, b)
	}
	var foreignRead bool
	_ = w.pool.QueryRow(ctx, "select read from notifications where id='ntf_foreign'").Scan(&foreignRead)
	if foreignRead {
		t.Fatal("mark-read leaked across users: foreign notification was marked read")
	}
}

func TestWebhookDelivery(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)

	ownerCk, _ := w.signupUser(t, "wbh-owner@example.com")
	if r, b := w.post(t, "/v1/orgs", `{"name":"wbhco"}`, ownerCk); r.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", r.StatusCode, b)
	}
	var orgID string
	_ = w.pool.QueryRow(ctx, "select id from orgs where slug='wbhco'").Scan(&orgID)

	// --- the SSRF guard rejects a non-routable / non-https target at create ---
	if r, _ := w.post(t, "/v1/orgs/"+orgID+"/webhooks", `{"url":"http://127.0.0.1:9/hook","events":[]}`, ownerCk); r.StatusCode != 422 {
		t.Fatalf("SSRF guard let a loopback http url through: %d", r.StatusCode)
	}

	// --- delivery to an httptest target (insecure router bypasses the guard) --
	var mu sync.Mutex
	var hits int
	var gotSig, gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		hits++
		gotSig = r.Header.Get("X-Steloit-Signature")
		gotBody = string(b)
		mu.Unlock()
		rw.WriteHeader(200)
	}))
	defer ts.Close()

	router := notify.NewRouter(q, w.kek).WithInsecureTargets().WithHTTPClient(ts.Client())
	id := "wbh_test1"
	secret, sealed, err := router.MintSecret(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateWebhook(ctx, store.CreateWebhookParams{
		ID: id, OrgID: orgID, Url: ts.URL, Events: []string{}, // {} = all kinds
		Ciphertext: sealed.Ciphertext, Nonce: sealed.Nonce,
		WrappedDek: sealed.WrappedDEK, DekNonce: sealed.DEKNonce, KekID: sealed.KEKID,
	}); err != nil {
		t.Fatal(err)
	}

	// seed a spine event the webhook subscribes to (empty filter = all).
	if _, err := w.pool.Exec(ctx,
		`insert into events (id, org_id, kind, via, actor, action, subject) values ('evt_wh1',$1,'lifecycle','system','system','service.ready','svc_x')`,
		orgID); err != nil {
		t.Fatal(err)
	}

	if _, err := router.ProcessWebhooks(ctx); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	h1, sig, gb := hits, gotSig, gotBody
	mu.Unlock()
	if h1 != 1 {
		t.Fatalf("expected 1 delivery, got %d", h1)
	}
	// the signature verifies with the reveal-once secret over the exact body.
	want := "sha256=" + hmacHex(secret, gb)
	if sig != want {
		t.Fatalf("bad signature: got %s want %s", sig, want)
	}

	// idempotent: a re-scan does NOT re-deliver the sent (webhook, event).
	if _, err := router.ProcessWebhooks(ctx); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	h2 := hits
	mu.Unlock()
	if h2 != 1 {
		t.Fatalf("outbox re-delivered a sent event: %d", h2)
	}

	// --- no backfill: an event predating the webhook is never delivered -------
	if _, err := w.pool.Exec(ctx,
		`insert into events (id, org_id, kind, via, actor, action, subject, at) values ('evt_old',$1,'lifecycle','system','system','old.thing','x', now() - interval '1 hour')`,
		orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := router.ProcessWebhooks(ctx); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	hBackfill := hits
	mu.Unlock()
	if hBackfill != 1 {
		t.Fatalf("webhook backfilled an event predating it: %d deliveries", hBackfill)
	}

	// --- testWebhook: synchronous, inline result, does NOT consume the slot ---
	res, err := router.Test(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Delivered || res.StatusCode != 200 {
		t.Fatalf("test delivery: %+v", res)
	}
	mu.Lock()
	h3 := hits
	mu.Unlock()
	if h3 != 2 { // the test POST landed, on top of the one real delivery
		t.Fatalf("test delivery count: %d", h3)
	}
}

// recordingSender is a test EmailSender that counts sends.
type recordingSender struct {
	mu sync.Mutex
	n  int
}

func (s *recordingSender) Send(_ context.Context, _, _, _ string) error {
	s.mu.Lock()
	s.n++
	s.mu.Unlock()
	return nil
}
func (s *recordingSender) count() int { s.mu.Lock(); defer s.mu.Unlock(); return s.n }

func hmacHex(secret, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return hex.EncodeToString(m.Sum(nil))
}
