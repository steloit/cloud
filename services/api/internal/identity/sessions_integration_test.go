package identity_test

// T7.3: session list + revoke over live HTTP — current flag, owner scoping,
// immediate revocation.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSessionListAndRevoke(t *testing.T) {
	w := newWorld(t, time.Hour)

	// one user, two sessions (signup + a second login)
	resp, _ := w.post(t, "/v1/auth/signup", `{"email":"ses@example.com","password":"orbit-magnet-11","name":"S"}`, "")
	if resp.StatusCode != 201 {
		t.Fatalf("signup: %d", resp.StatusCode)
	}
	ck1 := sessionCookie(resp)
	resp, _ = w.post(t, "/v1/auth/login", `{"email":"ses@example.com","password":"orbit-magnet-11"}`, "")
	ck2 := sessionCookie(resp)

	// list from session 2: both shown, exactly one marked current (this one)
	resp, body := w.get(t, "/v1/me/sessions", ck2)
	if resp.StatusCode != 200 {
		t.Fatalf("list: %d %s", resp.StatusCode, body)
	}
	var list struct {
		Data []struct {
			Id      string
			Current bool
			Device  string
		}
	}
	_ = json.Unmarshal([]byte(body), &list)
	if len(list.Data) != 2 {
		t.Fatalf("sessions: %d", len(list.Data))
	}
	currents, var2 := 0, ""
	for _, s := range list.Data {
		if s.Current {
			currents++
			var2 = s.Id
		}
	}
	if currents != 1 || var2 == "" {
		t.Fatalf("current flag: %d", currents)
	}
	// the OTHER session's id
	other := ""
	for _, s := range list.Data {
		if s.Id != var2 {
			other = s.Id
		}
	}

	// revoke the other session → 204; it dies IMMEDIATELY (ck1 now 401)
	resp, _ = w.del(t, "/v1/me/sessions/"+other, ck2)
	if resp.StatusCode != 204 {
		t.Fatalf("revoke: %d", resp.StatusCode)
	}
	resp, _ = w.get(t, "/v1/auth/session", ck1)
	if resp.StatusCode != 401 {
		t.Fatalf("revoked session still valid: %d", resp.StatusCode)
	}
	// the acting session still works
	resp, _ = w.get(t, "/v1/auth/session", ck2)
	if resp.StatusCode != 200 {
		t.Fatalf("acting session died: %d", resp.StatusCode)
	}
	// list now shows one
	resp, body = w.get(t, "/v1/me/sessions", ck2)
	_ = json.Unmarshal([]byte(body), &list)
	if len(list.Data) != 1 {
		t.Fatalf("post-revoke list: %d", len(list.Data))
	}

	// owner scoping: another user cannot revoke this user's session (404)
	resp, _ = w.post(t, "/v1/auth/signup", `{"email":"other@example.com","password":"orbit-magnet-11","name":"O"}`, "")
	ckO := sessionCookie(resp)
	resp, _ = w.del(t, "/v1/me/sessions/"+var2, ckO)
	if resp.StatusCode != 404 {
		t.Fatalf("cross-user revoke: %d (want 404)", resp.StatusCode)
	}
	// unknown id → 404
	resp, body = w.del(t, "/v1/me/sessions/ses_nope", ck2)
	if resp.StatusCode != 404 || !strings.Contains(body, "remediation") {
		t.Fatalf("unknown session: %d %s", resp.StatusCode, body)
	}
}
