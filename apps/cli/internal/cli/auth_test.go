package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAPI serves /v1/auth/session accepting exactly one bearer token.
func fakeAPI(t *testing.T, goodToken string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/session" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+goodToken {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":"usr_1","email":"asha@example.com"},"session":{"id":"","created_at":"2026-07-19T00:00:00Z"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAuthLoginFlagFlow(t *testing.T) {
	isolate(t)
	srv := fakeAPI(t, "stp_good")

	// wrong-looking token → usage
	code, _, errOut := runCLI(t, "auth", "login", "--token", "nope", "--api", srv.URL)
	if code != ExitUsage || !strings.Contains(errOut, "stp_") {
		t.Fatalf("bad-shape token: %d %s", code, errOut)
	}
	// rejected token → exit 3, not stored
	code, _, errOut = runCLI(t, "auth", "login", "--token", "stp_bad", "--api", srv.URL)
	if code != ExitPermission || !strings.Contains(errOut, "rejected") {
		t.Fatalf("rejected: %d %s", code, errOut)
	}
	cfg, _ := LoadConfig()
	if cfg.Token != "" {
		t.Fatal("rejected token stored")
	}
	// good token → verified, stored 0600, identity printed
	code, out, _ := runCLI(t, "auth", "login", "--token", "stp_good", "--api", srv.URL)
	if code != ExitOK || !strings.Contains(out, "asha@example.com") {
		t.Fatalf("login: %d %q", code, out)
	}
	cfg, _ = LoadConfig()
	if cfg.Token != "stp_good" || cfg.APIURL != srv.URL {
		t.Fatalf("stored config: %+v", cfg)
	}
	info, err := os.Stat(cfg.path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config perms: %v %v", info.Mode(), err)
	}

	// connect verify uses the stored token (A9)
	code, out, _ = runCLI(t, "connect", "verify")
	if code != ExitOK || !strings.Contains(out, "connected just now · personal token · asha@example.com") {
		t.Fatalf("verify: %d %q", code, out)
	}

	// logout forgets locally and says how to kill it server-side
	code, out, _ = runCLI(t, "auth", "logout")
	if code != ExitOK || !strings.Contains(out, "revoke it under your profile") {
		t.Fatalf("logout: %d %q", code, out)
	}
	cfg, _ = LoadConfig()
	if cfg.Token != "" {
		t.Fatal("token survived logout")
	}
	// verify after logout → exit 3
	if code, _, _ = runCLI(t, "connect", "verify"); code != ExitPermission {
		t.Fatalf("verify after logout: %d", code)
	}
}

func TestAuthLoginPasteFlow(t *testing.T) {
	dir := isolate(t)
	srv := fakeAPI(t, "stp_pasted")
	old := stdinReader
	stdinReader = strings.NewReader("stp_pasted\n")
	t.Cleanup(func() { stdinReader = old })

	code, out, _ := runCLI(t, "auth", "login", "--api", srv.URL)
	if code != ExitOK || !strings.Contains(out, "asha@example.com") {
		t.Fatalf("paste login: %d %q", code, out)
	}
	// the token never appears in any output
	if strings.Contains(out, "stp_pasted") {
		t.Fatal("token echoed")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if !strings.Contains(string(raw), "stp_pasted") {
		t.Fatal("token not stored")
	}
}
