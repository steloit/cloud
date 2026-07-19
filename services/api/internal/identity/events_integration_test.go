package identity_test

// T2.5: the spine against real Postgres — append-only trigger, audit view
// with RBAC + cursor pagination, and the SSE stream (replay + live).

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/events"
)

func TestEventsLedger(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	// owner + org via the real paths (org.created + member.added recorded)
	resp, _ := w.post(t, "/v1/auth/signup", `{"email":"spine@example.com","password":"orbit-magnet-11","name":"S"}`, "")
	if resp.StatusCode != 201 {
		t.Fatalf("signup: %d", resp.StatusCode)
	}
	ownerCk := sessionCookie(resp)
	var uid string
	if err := w.pool.QueryRow(ctx, "select id from users where email='spine@example.com'").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	org, err := w.svc.CreateOrgWithOwner(ctx, "spineco", uid)
	if err != nil {
		t.Fatal(err)
	}

	// --- every identity state change landed, with via ----------------------
	rows, err := w.pool.Query(ctx, "select action, via, actor from events where org_id=$1 order by at", org.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var action, via, actor string
		if err := rows.Scan(&action, &via, &actor); err != nil {
			t.Fatal(err)
		}
		if via != "user" || actor != uid {
			t.Fatalf("event %s has via=%s actor=%s, want user/%s", action, via, actor, uid)
		}
		got = append(got, action)
	}
	rows.Close()
	if strings.Join(got, ",") != "org.created,member.added" {
		t.Fatalf("spine rows: %v", got)
	}

	// --- append-only at the DB level ---------------------------------------
	if _, err := w.pool.Exec(ctx, "update events set action='tampered' where org_id=$1", org.ID); err == nil ||
		!strings.Contains(err.Error(), "append-only") {
		t.Fatalf("UPDATE did not raise: %v", err)
	}
	if _, err := w.pool.Exec(ctx, "delete from events where org_id=$1", org.ID); err == nil ||
		!strings.Contains(err.Error(), "append-only") {
		t.Fatalf("DELETE did not raise: %v", err)
	}

	// --- audit view: RBAC + pagination -------------------------------------
	resp, body := w.get(t, "/v1/orgs/"+org.ID+"/audit", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, "org.created") || !strings.Contains(body, "member.added") {
		t.Fatalf("owner audit: %d %s", resp.StatusCode, body)
	}
	var page struct {
		Data       []map[string]any `json:"data"`
		NextCursor *string          `json:"next_cursor"`
	}
	if err := json.Unmarshal([]byte(body), &page); err != nil || len(page.Data) != 2 {
		t.Fatalf("audit page shape: %v %s", err, body)
	}
	if page.Data[0]["action"] != "member.added" {
		t.Fatalf("audit not newest-first: %v", page.Data[0]["action"])
	}

	// developer: audit.read is N — 403 naming the role
	resp, _ = w.post(t, "/v1/auth/signup", `{"email":"sdev@example.com","password":"orbit-magnet-11","name":"D"}`, "")
	devCk := sessionCookie(resp)
	var devID string
	if err := w.pool.QueryRow(ctx, "select id from users where email='sdev@example.com'").Scan(&devID); err != nil {
		t.Fatal(err)
	}
	if err := w.svc.AddMember(ctx, org.ID, devID, "developer", uid); err != nil {
		t.Fatal(err)
	}
	resp, body = w.get(t, "/v1/orgs/"+org.ID+"/audit", devCk)
	if resp.StatusCode != 403 || !strings.Contains(body, "role:developer") {
		t.Fatalf("developer audit denial: %d %s", resp.StatusCode, body)
	}

	// malformed cursor → 422
	resp, _ = w.get(t, "/v1/orgs/"+org.ID+"/audit?cursor=%21%21", ownerCk)
	if resp.StatusCode != 422 {
		t.Fatalf("bad cursor: %d", resp.StatusCode)
	}

	// --- /envs/{env}/events JSON via the REAL resolver (T3.2) ---------------
	orgRow, err := w.svc.GetOrg(ctx, org.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := w.prov.CreateProject(ctx, orgRow, "spine", "", uid)
	if err != nil {
		t.Fatal(err)
	}
	resp, body = w.get(t, "/v1/envs/"+env.ID+"/events", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, "org.created") {
		t.Fatalf("env events: %d %s", resp.StatusCode, body)
	}
	resp, _ = w.get(t, "/v1/envs/env_missing/events", ownerCk)
	if resp.StatusCode != 404 {
		t.Fatalf("unknown env: %d", resp.StatusCode)
	}
}

func TestEventsSSE(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	resp, _ := w.post(t, "/v1/auth/signup", `{"email":"sse@example.com","password":"orbit-magnet-11","name":"E"}`, "")
	if resp.StatusCode != 201 {
		t.Fatalf("signup: %d", resp.StatusCode)
	}
	ck := sessionCookie(resp)
	var uid string
	if err := w.pool.QueryRow(ctx, "select id from users where email='sse@example.com'").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	org, err := w.svc.CreateOrgWithOwner(ctx, "sseco", uid) // 2 events pre-stream
	if err != nil {
		t.Fatal(err)
	}
	orgRow, err := w.svc.GetOrg(ctx, org.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := w.prov.CreateProject(ctx, orgRow, "ssedemo", "", uid) // +1 event
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodGet, w.srv.URL+"/v1/envs/"+env.ID+"/events", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cookie", ck)
	streamCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := http.DefaultClient.Do(req.WithContext(streamCtx))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream open: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}

	type frame struct{ id, event, data string }
	frames := make(chan frame, 16)
	go func() {
		sc := bufio.NewScanner(res.Body)
		var f frame
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "id: "):
				f.id = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				f.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				f.data = strings.TrimPrefix(line, "data: ")
			case line == "" && f.data != "":
				frames <- f
				f = frame{}
			}
		}
		close(frames)
	}()
	read := func(what string) frame {
		select {
		case f, open := <-frames:
			if !open {
				t.Fatalf("stream closed waiting for %s", what)
			}
			return f
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for %s", what)
		}
		return frame{}
	}

	// replay: the two pre-stream events, in order, with cursor ids
	f1 := read("replay org.created")
	if f1.event != "lifecycle" || !strings.Contains(f1.data, "org.created") || f1.id == "" {
		t.Fatalf("replay frame 1: %+v", f1)
	}
	f2 := read("replay member.added")
	if f2.event != "membership" || !strings.Contains(f2.data, "member.added") {
		t.Fatalf("replay frame 2: %+v", f2)
	}
	f3 := read("replay project.created")
	if f3.event != "lifecycle" || !strings.Contains(f3.data, "project.created") {
		t.Fatalf("replay frame 3: %+v", f3)
	}

	// live: a new state change arrives on the open stream
	var live string
	if err := w.pool.QueryRow(ctx, "select id from users where email='sse@example.com'").Scan(&live); err != nil {
		t.Fatal(err)
	}
	resp, _ = w.post(t, "/v1/auth/signup", `{"email":"sse2@example.com","password":"orbit-magnet-11","name":"E2"}`, "")
	var u2 string
	if err := w.pool.QueryRow(ctx, "select id from users where email='sse2@example.com'").Scan(&u2); err != nil {
		t.Fatal(err)
	}
	if err := w.svc.AddMember(ctx, org.ID, u2, "developer", uid); err != nil {
		t.Fatal(err)
	}
	f4 := read("live member.added")
	if f4.event != "membership" || !strings.Contains(f4.data, u2) {
		t.Fatalf("live frame: %+v", f4)
	}

	// reconnect from f3's cursor replays ONLY the live event — no gap, no dup
	req2, _ := http.NewRequest(http.MethodGet, w.srv.URL+"/v1/envs/"+env.ID+"/events?cursor="+f3.id, nil)
	req2.Header.Set("Accept", "text/event-stream")
	req2.Header.Set("Cookie", ck)
	ctx2, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()
	res2, err := http.DefaultClient.Do(req2.WithContext(ctx2))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	sc := bufio.NewScanner(res2.Body)
	var datas []string
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "data: ") {
			datas = append(datas, strings.TrimPrefix(sc.Text(), "data: "))
			break
		}
	}
	if len(datas) != 1 || !strings.Contains(datas[0], u2) {
		t.Fatalf("resume from cursor: %v", datas)
	}

	// T6.4 hardening: the browser's native Last-Event-ID header resumes
	// identically (EventSource sends it, not ?cursor=), and the stream opens
	// with a `retry:` hint so a dropped connection reconnects in 2s.
	req4, _ := http.NewRequest(http.MethodGet, w.srv.URL+"/v1/envs/"+env.ID+"/events", nil)
	req4.Header.Set("Accept", "text/event-stream")
	req4.Header.Set("Cookie", ck)
	req4.Header.Set("Last-Event-ID", f3.id)
	ctx4, cancel4 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel4()
	res4, err := http.DefaultClient.Do(req4.WithContext(ctx4))
	if err != nil {
		t.Fatal(err)
	}
	defer res4.Body.Close()
	sc4 := bufio.NewScanner(res4.Body)
	var sawRetry bool
	var replayed []string
	for sc4.Scan() {
		line := sc4.Text()
		if strings.HasPrefix(line, "retry: ") {
			sawRetry = true
		}
		if strings.HasPrefix(line, "data: ") {
			replayed = append(replayed, strings.TrimPrefix(line, "data: "))
			break
		}
	}
	if !sawRetry {
		t.Fatal("stream did not emit a retry: hint")
	}
	if len(replayed) != 1 || !strings.Contains(replayed[0], u2) {
		t.Fatalf("Last-Event-ID resume: %v", replayed)
	}

	// no credentials → 401 before any stream starts
	req3, _ := http.NewRequest(http.MethodGet, w.srv.URL+"/v1/envs/"+env.ID+"/events", nil)
	req3.Header.Set("Accept", "text/event-stream")
	res3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	res3.Body.Close()
	if res3.StatusCode != 401 {
		t.Fatalf("anonymous stream: %d", res3.StatusCode)
	}
	_ = events.ErrEnvNotFound // package anchor
}
