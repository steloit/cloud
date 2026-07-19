package identity_test

// T7.2: the password reset flow end-to-end — request (202, no disclosure) →
// email carries a single-use token → reset (204, all sessions revoked) →
// old password dead, new password works, token single-use.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/mailer"
)

func TestPasswordResetFlow(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)

	resp, _ := w.post(t, "/v1/auth/signup", `{"email":"reset@example.com","password":"orbit-magnet-11","name":"R"}`, "")
	if resp.StatusCode != 201 {
		t.Fatalf("signup: %d", resp.StatusCode)
	}
	oldCookie := sessionCookie(resp)

	// --- request: always 202, even for an unknown email (no disclosure) -----
	resp, _ = w.post(t, "/v1/auth/password:reset-request", `{"email":"reset@example.com"}`, "")
	if resp.StatusCode != 202 {
		t.Fatalf("reset-request: %d", resp.StatusCode)
	}
	resp, _ = w.post(t, "/v1/auth/password:reset-request", `{"email":"nobody@example.com"}`, "")
	if resp.StatusCode != 202 {
		t.Fatalf("reset-request unknown email should still be 202: %d", resp.StatusCode)
	}
	// the unknown email minted no token
	var tokens int
	_ = w.pool.QueryRow(ctx, "select count(*) from password_reset_tokens").Scan(&tokens)
	if tokens != 1 {
		t.Fatalf("unknown email minted a token: %d total", tokens)
	}

	// --- the email carries the token (dispatch the outbox to a spy) ---------
	spy := &spyProvider{}
	disp := mailer.NewDispatcher(spy, q, identity.NewMailDirectory(q, "https://c", w.kek), "noreply@steloit.app")
	if _, err := disp.ProcessAccountEmails(ctx); err != nil {
		t.Fatal(err)
	}
	if spy.count() != 1 {
		t.Fatalf("reset email not sent: %d", spy.count())
	}
	body := spy.sent[0].Text
	idx := strings.Index(body, "token=")
	if idx < 0 {
		t.Fatalf("no token in email: %q", body)
	}
	token := strings.Fields(body[idx+len("token="):])[0]

	// --- reset with the token → 204 ----------------------------------------
	resp, out := w.post(t, "/v1/auth/password:reset", `{"token":"`+token+`","password":"new-strong-pw-22"}`, "")
	if resp.StatusCode != 204 {
		t.Fatalf("reset: %d %s", resp.StatusCode, out)
	}

	// every prior session was revoked
	resp, _ = w.get(t, "/v1/auth/session", oldCookie)
	if resp.StatusCode == 200 {
		t.Fatal("prior session survived a password reset")
	}
	// old password no longer works, new one does
	resp, _ = w.post(t, "/v1/auth/login", `{"email":"reset@example.com","password":"orbit-magnet-11"}`, "")
	if resp.StatusCode != 401 {
		t.Fatalf("old password still works: %d", resp.StatusCode)
	}
	resp, out = w.post(t, "/v1/auth/login", `{"email":"reset@example.com","password":"new-strong-pw-22"}`, "")
	if resp.StatusCode != 200 || !strings.Contains(out, `"status":"session"`) {
		t.Fatalf("new password login: %d %s", resp.StatusCode, out)
	}

	// --- single-use: the token can't be replayed ---------------------------
	resp, _ = w.post(t, "/v1/auth/password:reset", `{"token":"`+token+`","password":"another-pw-333"}`, "")
	if resp.StatusCode != 422 {
		t.Fatalf("reused reset token: %d", resp.StatusCode)
	}
	// an unknown token is indistinguishable (422, no probing)
	resp, _ = w.post(t, "/v1/auth/password:reset", `{"token":"deadbeef","password":"another-pw-333"}`, "")
	if resp.StatusCode != 422 {
		t.Fatalf("unknown token: %d", resp.StatusCode)
	}
	// a weak new password is rejected
	resp, _ = w.post(t, "/v1/auth/password:reset", `{"token":"`+token+`","password":"short"}`, "")
	if resp.StatusCode != 422 {
		t.Fatalf("weak password: %d", resp.StatusCode)
	}
}
