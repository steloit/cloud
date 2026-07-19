package cli

// US-5.1's implicit-env rules: n=1 never asks; n≥2 reads default to
// production; n≥2 mutations never guess — TTY prompts, non-TTY exits 2.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func multiEnvPlatform(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/auth/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":"usr_1","email":"a@b.c"},"session":{"id":"","created_at":"2026-07-19T00:00:00Z"}}`))
	})
	mux.HandleFunc("GET /v1/projects/prj_m/envs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"env_p","project_id":"prj_m","name":"production","region":"aws/ap-south-1"},{"id":"env_s","project_id":"prj_m","name":"staging","region":"aws/ap-south-1"}]}`))
	})
	mux.HandleFunc("GET /v1/envs/env_p/services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("POST /v1/estimates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"est_1","monthly_total_cents":1900,"lines":[],"expires_at":"2026-07-19T01:00:00Z"}`))
	})
	mux.HandleFunc("POST /v1/envs/env_s/services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"svc_s","env_id":"env_s","name":"db","product":"postgres","status":"provisioning"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestImplicitEnvNeverGuessesOnMutation(t *testing.T) {
	isolate(t)
	srv := multiEnvPlatform(t)
	connect(t, srv)

	// n≥2 read: defaults to production silently
	code, _, _ := runCLI(t, "db", "list", "--project", "prj_m")
	if code != 0 {
		t.Fatalf("read default: %d", code)
	}

	// n≥2 mutation, non-TTY: exit 2 naming the envs and the remediation
	old0, oldTTY0 := stdinReader, ttyForTests
	stdinReader, ttyForTests = strings.NewReader(""), false
	code, _, errOut := runCLI(t, "db", "create", "db", "--project", "prj_m", "--yes")
	stdinReader, ttyForTests = old0, oldTTY0
	if code != ExitUsage || !strings.Contains(errOut, "staging") || !strings.Contains(errOut, "pass --env") {
		t.Fatalf("non-TTY mutation: %d %q", code, errOut)
	}

	// n≥2 mutation, TTY: prompts with the list; choosing staging proceeds
	old, oldTTY := stdinReader, ttyForTests
	stdinReader, ttyForTests = strings.NewReader("staging\n"), true
	code, out, errOut := runCLI(t, "db", "create", "db", "--project", "prj_m", "--yes")
	stdinReader, ttyForTests = old, oldTTY
	if code != 0 || !strings.Contains(errOut, "1) production") || !strings.Contains(out, "svc_s") {
		t.Fatalf("TTY prompt: %d %q %q", code, out, errOut)
	}

	// explicit --env staging: no prompt, straight through
	code, out, _ = runCLI(t, "db", "create", "db", "--project", "prj_m", "--env", "staging", "--yes")
	if code != 0 || !strings.Contains(out, "svc_s") {
		t.Fatalf("explicit env: %d %q", code, out)
	}
}
