package identity_test

// T13.3 — the assistant thread store, and its AI Law 4 guarantee: disabling the
// ai-assistant policy HIDES the surface (404 empty-equivalent on create, omitted
// from list) but DELETES NOTHING — re-enabling restores every thread verbatim.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAssistantThreadsDisableHidesNeverDeletes(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, _ := w.signupUser(t, "ai-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"aico"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	// create two threads while enabled (no policy row = enabled by default)
	mk := func(page string) string {
		r, b := w.post(t, "/v1/assistant/threads",
			`{"context":{"org":"`+org.Id+`","page":"`+page+`"}}`, ownerCk)
		if r.StatusCode != 201 {
			t.Fatalf("createThread(%s): %d %s", page, r.StatusCode, b)
		}
		var thr struct{ Id string }
		_ = json.Unmarshal([]byte(b), &thr)
		if !strings.HasPrefix(thr.Id, "thr_") {
			t.Fatalf("thread id shape: %q", thr.Id)
		}
		return thr.Id
	}
	id1 := mk("overview")
	_ = mk("services")

	// list returns both
	r, b := w.get(t, "/v1/assistant/threads", ownerCk)
	if r.StatusCode != 200 || strings.Count(b, "thr_") < 2 {
		t.Fatalf("listThreads (enabled) should show 2: %d %s", r.StatusCode, b)
	}

	// DISABLE the ai-assistant policy
	if _, err := w.pool.Exec(ctx,
		`insert into policies (id, org_id, key, enforcement) values ('pol_ai', $1, 'ai-assistant', 'disabled')`,
		org.Id); err != nil {
		t.Fatal(err)
	}

	// create is now 404 empty-equivalent (Law 4: the AI surface is invisible)
	r, b = w.post(t, "/v1/assistant/threads", `{"context":{"org":"`+org.Id+`"}}`, ownerCk)
	if r.StatusCode != 404 {
		t.Fatalf("createThread while disabled must 404, got %d %s", r.StatusCode, b)
	}

	// list HIDES the org's threads (not deleted — still in the store)
	r, b = w.get(t, "/v1/assistant/threads", ownerCk)
	if r.StatusCode != 200 || strings.Contains(b, "thr_") {
		t.Fatalf("listThreads while disabled must hide all threads: %d %s", r.StatusCode, b)
	}
	var stored int
	_ = w.pool.QueryRow(ctx, "select count(*) from assistant_threads where org_id=$1", org.Id).Scan(&stored)
	if stored != 2 {
		t.Fatalf("disable DELETED threads (found %d, expected 2) — Law 4 violated", stored)
	}

	// RE-ENABLE → instant restore, verbatim
	if _, err := w.pool.Exec(ctx,
		`update policies set enforcement='enabled' where id='pol_ai'`); err != nil {
		t.Fatal(err)
	}
	r, b = w.get(t, "/v1/assistant/threads", ownerCk)
	if r.StatusCode != 200 || strings.Count(b, "thr_") < 2 || !strings.Contains(b, id1) {
		t.Fatalf("re-enable must restore both threads incl %s: %d %s", id1, r.StatusCode, b)
	}
}

func TestAssistantThreadNonMemberAndContextRequired(t *testing.T) {
	w := newWorld(t, time.Hour)

	ownerCk, _ := w.signupUser(t, "ai-m-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"aimco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	// missing context.org → 422 validation
	r, _ := w.post(t, "/v1/assistant/threads", `{"context":{"page":"x"}}`, ownerCk)
	if r.StatusCode != 422 {
		t.Fatalf("missing context.org must 422, got %d", r.StatusCode)
	}

	// a non-member gets 404 (the AI surface must not admit the org exists — the
	// same convention as GetOrg; never a 403 that leaks org-exists + AI-state).
	strangerCk, _ := w.signupUser(t, "ai-stranger@example.com")
	r, b := w.post(t, "/v1/assistant/threads", `{"context":{"org":"`+org.Id+`"}}`, strangerCk)
	if r.StatusCode != 404 {
		t.Fatalf("non-member must get 404 (no org-exists/AI-state leak), got %d %s", r.StatusCode, b)
	}
}

// TestAssistantProjectScopedDisable — AI3 per-project override (review H1): an
// org-level disable can be re-enabled for one project (and vice-versa), so the
// gate is closest-wins, not org-wide-any-disable.
func TestAssistantProjectScopedDisable(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, _ := w.signupUser(t, "ai-proj-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"aiproj"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	r, pb := w.post(t, "/v1/orgs/"+org.Id+"/projects", `{"name":"papp"}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("createProject: %d %s", r.StatusCode, pb)
	}
	var proj struct{ Id string }
	_ = json.Unmarshal([]byte(pb), &proj)

	// org-level DISABLE, but a project-level ENABLE override for papp.
	if _, err := w.pool.Exec(ctx,
		`insert into policies (id, org_id, key, enforcement) values ('pol_org', $1, 'ai-assistant', 'disabled')`, org.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx,
		`insert into policies (id, org_id, project_id, key, enforcement) values ('pol_proj', $1, $2, 'ai-assistant', 'enabled')`, org.Id, proj.Id); err != nil {
		t.Fatal(err)
	}
	// org-scoped create (no project) is disabled → 404
	r, b := w.post(t, "/v1/assistant/threads", `{"context":{"org":"`+org.Id+`"}}`, ownerCk)
	if r.StatusCode != 404 {
		t.Fatalf("org-scoped create should be disabled: %d %s", r.StatusCode, b)
	}
	// project-scoped create for papp is ENABLED by the override → 201
	r, b = w.post(t, "/v1/assistant/threads",
		`{"context":{"org":"`+org.Id+`","project":"`+proj.Id+`"}}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("project override should re-enable AI for papp: %d %s", r.StatusCode, b)
	}
}

// TestAssistantRejectsOrgKey — the assistant is a human surface; an org-key
// principal (even one granted ai.use) gets a clean 403, never a user_id FK 500
// (review M3).
func TestAssistantRejectsOrgKey(t *testing.T) {
	w := newWorld(t, time.Hour)
	ownerCk, _ := w.signupUser(t, "ai-key-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"aikey"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	r, kb := w.post(t, "/v1/orgs/"+org.Id+"/api-keys",
		`{"name":"ai","scope":"full","permissions":["ai.use"]}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("createApiKey: %d %s", r.StatusCode, kb)
	}
	var key struct{ Token string }
	_ = json.Unmarshal([]byte(kb), &key)
	// use the org key (bearer) on the assistant surface → 403, not 500
	req, _ := http.NewRequest(http.MethodPost, w.srv.URL+"/v1/assistant/threads",
		strings.NewReader(`{"context":{"org":"`+org.Id+`"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key.Token)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != 403 {
		t.Fatalf("org-key on assistant must be a clean 403, got %d %s", resp2.StatusCode, string(b))
	}
}
