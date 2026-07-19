package identity_test

// T13.3 — the assistant thread store, and its AI Law 4 guarantee: disabling the
// ai-assistant policy HIDES the surface (404 empty-equivalent on create, omitted
// from list) but DELETES NOTHING — re-enabling restores every thread verbatim.

import (
	"context"
	"encoding/json"
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

	// a non-member gets 403/404 — never a thread in someone else's org
	strangerCk, _ := w.signupUser(t, "ai-stranger@example.com")
	r, b := w.post(t, "/v1/assistant/threads", `{"context":{"org":"`+org.Id+`"}}`, strangerCk)
	if r.StatusCode == 201 {
		t.Fatalf("non-member created a thread in another org: %s", b)
	}
}
