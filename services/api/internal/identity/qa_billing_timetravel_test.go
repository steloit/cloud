package identity_test

// Q9 — the billing time-travel harness: a warped clock injected into every
// billing clock seam (subscription + identity), so QA scenarios 3 and 4 run in
// SECONDS, never real days. The clock only moves when a test calls Advance.

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// warpedClock is the Q9 harness: a mutable now shared by every WithClock seam.
type warpedClock struct {
	mu  sync.Mutex
	now time.Time
}

func newWarpedClock() *warpedClock { return &warpedClock{now: time.Now()} }
func (c *warpedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}
func (c *warpedClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

const day = 24 * time.Hour

// TestQAScenario3Dunning — qa.md #3: "fail card → day 8 = banner, provisioning
// paused, retries listed; payment clears everything instantly; day-90 rule
// never earlier." All time travel is warped — the test runs in seconds.
func TestQAScenario3Dunning(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	clock := newWarpedClock()
	w.subs.WithClock(clock.Now)
	w.svc.WithClock(clock.Now)

	ownerCk, _ := w.signupUser(t, "dun-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"dunco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	if _, err := w.pool.Exec(ctx, "update orgs set plan='business' where id=$1", org.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, "update subscriptions set plan='business' where org_id=$1", org.Id); err != nil {
		t.Fatal(err)
	}

	// day 0: the card fails → grace, retries scheduled.
	if _, err := w.subs.FailPayment(ctx, org.Id); err != nil {
		t.Fatalf("fail payment: %v", err)
	}
	_, sb := w.get(t, "/v1/orgs/"+org.Id+"/subscription", ownerCk)
	if !strings.Contains(sb, `"state":"grace"`) || !strings.Contains(sb, "next_retry_at") {
		t.Fatalf("day 0 should be grace with a retry listed: %s", sb)
	}

	// day 8: banner state = provisioning paused (the 7-day threshold passed);
	// retries still listed; dunning day shows 8.
	clock.Advance(8 * day)
	if _, err := w.subs.AdvanceLifecycle(ctx, org.Id); err != nil {
		t.Fatalf("advance day 8: %v", err)
	}
	_, sb = w.get(t, "/v1/orgs/"+org.Id+"/subscription", ownerCk)
	if !strings.Contains(sb, `"status":"provisioning_paused"`) || !strings.Contains(sb, `"day":8`) {
		t.Fatalf("day 8 should be provisioning_paused with day:8: %s", sb)
	}
	if !strings.Contains(sb, "next_retry_at") {
		t.Fatalf("retries must stay listed while paused: %s", sb)
	}
	// provisioning is REALLY paused: running services untouched, new ones... —
	// (provisioning-pause enforcement at the create gate is US-11.2 surface;
	// the state that drives it is asserted here.)

	// payment clears EVERYTHING instantly — no residual dunning state.
	if _, err := w.subs.Activate(ctx, org.Id); err != nil {
		t.Fatalf("activate: %v", err)
	}
	_, sb = w.get(t, "/v1/orgs/"+org.Id+"/subscription", ownerCk)
	if !strings.Contains(sb, `"status":"current"`) || strings.Contains(sb, "dunning") {
		t.Fatalf("payment must clear everything instantly: %s", sb)
	}

	// day-90 rule NEVER earlier: fail again, ride to suspended, then to day 89 —
	// the org, subscription, and data rows all still exist (nothing deletes for
	// billing reasons before day 90).
	if _, err := w.subs.FailPayment(ctx, org.Id); err != nil {
		t.Fatal(err)
	}
	clock.Advance(25 * day)
	if _, err := w.subs.AdvanceLifecycle(ctx, org.Id); err != nil { // → paused
		t.Fatal(err)
	}
	if _, err := w.subs.AdvanceLifecycle(ctx, org.Id); err != nil { // → suspended (day >= 21)
		t.Fatal(err)
	}
	_, sb = w.get(t, "/v1/orgs/"+org.Id+"/subscription", ownerCk)
	if !strings.Contains(sb, `"status":"suspended"`) {
		t.Fatalf("day 25 should be suspended (state kept): %s", sb)
	}
	clock.Advance(64 * day) // day 89 total on this track
	if _, err := w.subs.AdvanceLifecycle(ctx, org.Id); err != nil {
		t.Fatal(err)
	}
	var orgs, subs int
	_ = w.pool.QueryRow(ctx, "select count(*) from orgs where id=$1", org.Id).Scan(&orgs)
	_ = w.pool.QueryRow(ctx, "select count(*) from subscriptions where org_id=$1", org.Id).Scan(&subs)
	if orgs != 1 || subs != 1 {
		t.Fatalf("day-89: billing deletion fired EARLY (orgs=%d subs=%d) — the 90-day rule is never earlier", orgs, subs)
	}
}

// TestQAScenario4DowngradeBlock — qa.md #4: "Business→Pro with 12 members →
// 409 with the reasons, verbatim." (The canon names members + cells; cells are
// not yet data — members + projects are the evaluable dimensions today, and the
// cells reason is a recorded finding.) Also proves the CLEAN downgrade pends to
// the anchor under the warped clock and applies when it passes.
func TestQAScenario4DowngradeBlock(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	clock := newWarpedClock()
	w.subs.WithClock(clock.Now)
	w.svc.WithClock(clock.Now)

	ownerCk, ownerID := w.signupUser(t, "down-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"downco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	if _, err := w.pool.Exec(ctx, "update orgs set plan='business' where id=$1", org.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, "update subscriptions set plan='business' where org_id=$1", org.Id); err != nil {
		t.Fatal(err)
	}
	// 12 members total (11 + owner) and 4+ projects (> Pro's 3).
	for i := 0; i < 11; i++ {
		_, uid := w.signupUser(t, "down-m"+string(rune('a'+i))+"@example.com")
		if _, err := w.pool.Exec(ctx, "insert into members (id,org_id,user_id,role) values ($1,$2,$3,'developer')", "mem_down"+string(rune('a'+i)), org.Id, uid); err != nil {
			t.Fatal(err)
		}
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.Id)
	for _, name := range []string{"p1", "p2", "p3", "p4"} {
		if _, _, err := w.prov.CreateProject(ctx, orgRow, name, "", ownerID); err != nil {
			t.Fatalf("project %s: %v", name, err)
		}
	}

	// Business→Pro: 409 with BOTH reasons, verbatim grammar.
	r, db := w.post(t, "/v1/orgs/"+org.Id+"/subscription", `{"plan":"pro"}`, ownerCk)
	if r.StatusCode != 409 {
		t.Fatalf("blocked downgrade must 409: %d %s", r.StatusCode, db)
	}
	// the encoder escapes '>' as \u003e — unescape before the verbatim check.
	plain := strings.ReplaceAll(db, `\u003e`, ">")
	if !strings.Contains(plain, "12 members > Pro's 5 included seats") {
		t.Fatalf("members reason missing/not verbatim: %s", plain)
	}
	if !strings.Contains(plain, "4 projects > Pro's 3 project limit") {
		t.Fatalf("projects reason missing/not verbatim: %s", plain)
	}

	// Clean downgrade pends to the anchor and applies under warp.
	if _, err := w.pool.Exec(ctx, "delete from members where org_id=$1 and role='developer'", org.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, `delete from projects where org_id=$1 and name in ('p2','p3','p4')`, org.Id); err != nil {
		t.Fatal(err)
	}
	r, db = w.post(t, "/v1/orgs/"+org.Id+"/subscription", `{"plan":"pro"}`, ownerCk)
	if r.StatusCode != 200 {
		t.Fatalf("clean downgrade: %d %s", r.StatusCode, db)
	}
	var pending string
	_ = w.pool.QueryRow(ctx, "select coalesce(pending_plan,'') from subscriptions where org_id=$1", org.Id).Scan(&pending)
	if pending != "pro" {
		t.Fatalf("clean downgrade must PEND to the anchor, got pending=%q", pending)
	}
	// the plan is still business until the anchor passes (never immediate).
	var planNow string
	_ = w.pool.QueryRow(ctx, "select plan from orgs where id=$1", org.Id).Scan(&planNow)
	if planNow != "business" {
		t.Fatalf("downgrade applied immediately (must wait for anchor): %s", planNow)
	}
	clock.Advance(35 * day) // safely past any anchor day
	if _, err := w.subs.AdvanceLifecycle(ctx, org.Id); err != nil {
		t.Fatal(err)
	}
	_ = w.pool.QueryRow(ctx, "select plan from subscriptions where org_id=$1", org.Id).Scan(&planNow)
	if planNow != "pro" {
		t.Fatalf("pending downgrade did not apply at the anchor: %s", planNow)
	}
	_ = w.pool.QueryRow(ctx, "select plan from orgs where id=$1", org.Id).Scan(&planNow)
	if planNow != "pro" {
		t.Fatalf("orgs.plan diverged from the billing master after anchor apply: %s", planNow)
	}
}
