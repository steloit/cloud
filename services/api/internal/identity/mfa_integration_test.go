package identity_test

// T7.1: MFA over live HTTP — TOTP enroll → verify → the login mfa_required
// challenge → recovery-code login → regenerate → disable. MFA never
// plan-gated; the TOTP secret is never plaintext at rest.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/totp"
)

func TestMFATotpLifecycle(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	resp, _ := w.post(t, "/v1/auth/signup", `{"email":"mfa@example.com","password":"orbit-magnet-11","name":"M"}`, "")
	if resp.StatusCode != 201 {
		t.Fatalf("signup: %d", resp.StatusCode)
	}
	ck := sessionCookie(resp)
	var uid string
	if err := w.pool.QueryRow(ctx, "select id from users where email='mfa@example.com'").Scan(&uid); err != nil {
		t.Fatal(err)
	}

	// --- enroll: reveal-once secret + otpauth URI --------------------------
	resp, body := w.post(t, "/v1/auth/mfa/totp:enroll", "", ck)
	if resp.StatusCode != 200 {
		t.Fatalf("enroll: %d %s", resp.StatusCode, body)
	}
	var enr struct {
		Secret     string `json:"secret"`
		OtpauthUri string `json:"otpauth_uri"`
	}
	_ = json.Unmarshal([]byte(body), &enr)
	if enr.Secret == "" || !strings.HasPrefix(enr.OtpauthUri, "otpauth://totp/") {
		t.Fatalf("enroll shape: %+v", enr)
	}
	// secret is NOT plaintext at rest (the row is envelope-encrypted)
	var ct []byte
	if err := w.pool.QueryRow(ctx, "select ciphertext from mfa_totp where user_id=$1", uid).Scan(&ct); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ct), enr.Secret) {
		t.Fatal("TOTP secret plaintext at rest")
	}
	// not yet enabled (unconfirmed)
	var enabled bool
	_ = w.pool.QueryRow(ctx, "select mfa_enabled from users where id=$1", uid).Scan(&enabled)
	if enabled {
		t.Fatal("MFA enabled before verify")
	}

	// --- verify with a WRONG code fails; a live code enables ---------------
	resp, _ = w.post(t, "/v1/auth/mfa/totp:verify", `{"code":"000000"}`, ck)
	if resp.StatusCode == 204 {
		t.Fatal("wrong code enabled MFA")
	}
	good, _ := totp.Code(enr.Secret, time.Now())
	resp, body = w.post(t, "/v1/auth/mfa/totp:verify", `{"code":"`+good+`"}`, ck)
	if resp.StatusCode != 204 {
		t.Fatalf("verify: %d %s", resp.StatusCode, body)
	}
	_ = w.pool.QueryRow(ctx, "select mfa_enabled from users where id=$1", uid).Scan(&enabled)
	if !enabled {
		t.Fatal("MFA not enabled after verify")
	}

	// --- recovery codes: reveal-once, hash-stored --------------------------
	resp, body = w.post(t, "/v1/auth/mfa/recovery:regenerate", "", ck)
	if resp.StatusCode != 200 {
		t.Fatalf("recovery: %d %s", resp.StatusCode, body)
	}
	var rec struct {
		Codes []string `json:"codes"`
	}
	_ = json.Unmarshal([]byte(body), &rec)
	if len(rec.Codes) != 10 {
		t.Fatalf("recovery codes: %d", len(rec.Codes))
	}
	var stored int
	if err := w.pool.QueryRow(ctx, "select count(*) from recovery_codes where user_id=$1", uid).Scan(&stored); err != nil || stored != 10 {
		t.Fatalf("recovery rows: %d", stored)
	}
	var plain int
	_ = w.pool.QueryRow(ctx, "select count(*) from recovery_codes where user_id=$1 and code_hash=$2", uid, []byte(rec.Codes[0])).Scan(&plain)
	if plain != 0 {
		t.Fatal("recovery code stored as plaintext")
	}

	// --- login now REQUIRES the second factor ------------------------------
	resp, body = w.post(t, "/v1/auth/login", `{"email":"mfa@example.com","password":"orbit-magnet-11"}`, "")
	if resp.StatusCode != 200 || !strings.Contains(body, `"status":"mfa_required"`) {
		t.Fatalf("login without code: %d %s", resp.StatusCode, body)
	}
	if strings.Contains(body, `"session"`) && strings.Contains(body, `"id":"ses_`) {
		t.Fatal("session issued despite mfa_required")
	}
	// wrong code → auth failure
	resp, _ = w.post(t, "/v1/auth/login", `{"email":"mfa@example.com","password":"orbit-magnet-11","mfa_code":"000000"}`, "")
	if resp.StatusCode != 401 {
		t.Fatalf("login wrong code: %d", resp.StatusCode)
	}
	// --- live TOTP → session, and that same code can't be replayed ---------
	// The replay guard is monotonic on the RFC 6238 step, so a single fast test
	// can consume exactly one step above the one recorded at verify. Use the
	// NEXT window's code (accepted now via the +1 skew, higher step); the
	// verify step is already consumed, so a current-window code would collide.
	next, _ := totp.Code(enr.Secret, time.Now().Add(30*time.Second))
	resp, body = w.post(t, "/v1/auth/login", `{"email":"mfa@example.com","password":"orbit-magnet-11","mfa_code":"`+next+`"}`, "")
	if resp.StatusCode != 200 || !strings.Contains(body, `"status":"session"`) {
		t.Fatalf("login with TOTP: %d %s", resp.StatusCode, body)
	}
	// replay of the SAME code → rejected (step already consumed)
	resp, _ = w.post(t, "/v1/auth/login", `{"email":"mfa@example.com","password":"orbit-magnet-11","mfa_code":"`+next+`"}`, "")
	if resp.StatusCode != 401 {
		t.Fatalf("TOTP code replayed within window: %d", resp.StatusCode)
	}

	// --- a recovery code logs in ONCE (single-use) -------------------------
	rcode := rec.Codes[3]
	resp, _ = w.post(t, "/v1/auth/login", `{"email":"mfa@example.com","password":"orbit-magnet-11","mfa_code":"`+rcode+`"}`, "")
	if resp.StatusCode != 200 {
		t.Fatalf("recovery login: %d", resp.StatusCode)
	}
	resp, _ = w.post(t, "/v1/auth/login", `{"email":"mfa@example.com","password":"orbit-magnet-11","mfa_code":"`+rcode+`"}`, "")
	if resp.StatusCode != 401 {
		t.Fatalf("recovery code reused: %d", resp.StatusCode)
	}

	// --- recovery still works when the TOTP secret is gone (device loss) ----
	// Drop the mfa_totp row directly (simulating a lost/undecryptable secret)
	// while mfa_enabled stays true; a recovery code must still authenticate.
	if _, err := w.pool.Exec(ctx, "delete from mfa_totp where user_id=$1", uid); err != nil {
		t.Fatal(err)
	}
	rcode2 := rec.Codes[7]
	resp, body = w.post(t, "/v1/auth/login", `{"email":"mfa@example.com","password":"orbit-magnet-11","mfa_code":"`+rcode2+`"}`, "")
	if resp.StatusCode != 200 || !strings.Contains(body, `"status":"session"`) {
		t.Fatalf("recovery fallback without TOTP secret: %d %s", resp.StatusCode, body)
	}
	// re-enroll for the disable leg below
	resp, body = w.post(t, "/v1/auth/mfa/totp:enroll", "", ck)
	if resp.StatusCode != 200 {
		t.Fatalf("re-enroll: %d %s", resp.StatusCode, body)
	}
	_ = json.Unmarshal([]byte(body), &enr)
	// an UNCONFIRMED re-enrolled secret is NOT accepted at login (confirmed_at gate)
	pending, _ := totp.Code(enr.Secret, time.Now())
	resp, _ = w.post(t, "/v1/auth/login", `{"email":"mfa@example.com","password":"orbit-magnet-11","mfa_code":"`+pending+`"}`, "")
	if resp.StatusCode != 401 {
		t.Fatalf("unconfirmed secret accepted at login: %d", resp.StatusCode)
	}
	good, _ = totp.Code(enr.Secret, time.Now())
	if resp, _ = w.post(t, "/v1/auth/mfa/totp:verify", `{"code":"`+good+`"}`, ck); resp.StatusCode != 204 {
		t.Fatalf("re-verify: %d", resp.StatusCode)
	}

	// --- disable: MFA off; login no longer challenges ----------------------
	resp, _ = w.del(t, "/v1/auth/mfa", ck)
	if resp.StatusCode != 204 {
		t.Fatalf("disable: %d", resp.StatusCode)
	}
	_ = w.pool.QueryRow(ctx, "select mfa_enabled from users where id=$1", uid).Scan(&enabled)
	if enabled {
		t.Fatal("MFA still enabled after disable")
	}
	var n int
	_ = w.pool.QueryRow(ctx, "select count(*) from mfa_totp where user_id=$1", uid).Scan(&n)
	if n != 0 {
		t.Fatal("TOTP row survived disable")
	}
	resp, body = w.post(t, "/v1/auth/login", `{"email":"mfa@example.com","password":"orbit-magnet-11"}`, "")
	if resp.StatusCode != 200 || !strings.Contains(body, `"status":"session"`) {
		t.Fatalf("login after disable: %d %s", resp.StatusCode, body)
	}
}
