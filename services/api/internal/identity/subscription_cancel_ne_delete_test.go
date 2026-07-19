package identity_test

// US-11.5 — B12: Cancel ≠ Delete. Cancelling a subscription changes the PLAN
// (ends at the anchor), never the RESOURCES: services keep running and keep
// metering. The way back is one click ("Resume" before the anchor uncancels;
// "Restart" after re-subscribes via changePlan) — as clean as the way out.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCancelDoesNotStopServicesOrMetering(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "c12-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"c12co"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	// a paid subscription to cancel
	if _, err := w.pool.Exec(ctx, "update subscriptions set plan='business' where org_id=$1", org.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, "update orgs set plan='business' where id=$1", org.Id); err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.Id)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_ = prj
	resp, body = w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createService: %d %s", resp.StatusCode, body)
	}
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)
	// bring the service to ready → its metering span OPENS
	row, orgID, _ := w.prov.ServiceOrg(ctx, svc.Id)
	if _, err := w.prov.Transition(ctx, row, "ready", "system", "system", orgID); err != nil {
		t.Fatal(err)
	}
	spanRows := func() int {
		var n int
		_ = w.pool.QueryRow(ctx, "select count(*) from usage_events where service_id=$1", svc.Id).Scan(&n)
		return n
	}
	if spanRows() != 1 { // one open edge
		t.Fatalf("expected the open span edge, got %d", spanRows())
	}

	// CANCEL the subscription.
	if _, err := w.subs.Cancel(ctx, org.Id, "too-expensive"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// B12 invariant: the SERVICE is untouched (still ready) and its metering span
	// is still OPEN — cancel emitted NO billing-close edge.
	var svcStatus string
	_ = w.pool.QueryRow(ctx, "select status from services where id=$1", svc.Id).Scan(&svcStatus)
	if svcStatus != "ready" {
		t.Fatalf("cancel changed the service status to %q — Cancel ≠ Delete, resources keep running", svcStatus)
	}
	if spanRows() != 1 {
		t.Fatalf("cancel emitted a billing edge (%d rows) — metering must keep running after cancel", spanRows())
	}

	// the API echoes the wind-down contract: resources_unaffected, plan ends at
	// the anchor.
	_, sb := w.get(t, "/v1/orgs/"+org.Id+"/subscription", ownerCk)
	if !strings.Contains(sb, `"status":"cancelled_at_anchor"`) || !strings.Contains(sb, `"resources_unaffected":true`) {
		t.Fatalf("wind-down contract not echoed: %s", sb)
	}

	// metering STILL accrues after cancel: a later close (real teardown) is the
	// only thing that stops it — prove the span can still close cleanly.
	row, _, _ = w.prov.ServiceOrg(ctx, svc.Id)
	if _, err := w.prov.Transition(ctx, row, "deleting", "system", "system", orgID); err != nil {
		t.Fatal(err)
	}
	if spanRows() != 2 { // open + close
		t.Fatalf("the span should close on real teardown (not on cancel): %d rows", spanRows())
	}
}

func TestResumeBeforeAnchorAndRestartAfter(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	clock := newWarpedClock()
	w.subs.WithClock(clock.Now)
	w.svc.WithClock(clock.Now)

	ownerCk, _ := w.signupUser(t, "resume-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"resumeco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	if _, err := w.pool.Exec(ctx, "update subscriptions set plan='pro' where org_id=$1", org.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, "update orgs set plan='pro' where id=$1", org.Id); err != nil {
		t.Fatal(err)
	}

	// cancel → cancelled_at_anchor
	if _, err := w.subs.Cancel(ctx, org.Id, ""); err != nil {
		t.Fatal(err)
	}
	// "Resume" BEFORE the anchor: one click uncancels — status current, plan
	// restored, wind-down cleared (no plan_ends_at in the response).
	r, b := w.post(t, "/v1/orgs/"+org.Id+"/subscription/reactivate", ``, ownerCk)
	if r.StatusCode != 200 {
		t.Fatalf("resume (reactivate): %d %s", r.StatusCode, b)
	}
	if !strings.Contains(b, `"status":"current"`) || !strings.Contains(b, `"plan":"pro"`) || strings.Contains(b, "wind_down") {
		t.Fatalf("resume must restore current/pro and clear wind-down: %s", b)
	}
	var planEnds bool
	_ = w.pool.QueryRow(ctx, "select plan_ends_at is not null from subscriptions where org_id=$1", org.Id).Scan(&planEnds)
	if planEnds {
		t.Fatal("resume did not clear plan_ends_at")
	}

	// Now cancel again and let the anchor PASS → the plan ends, org drops to free.
	if _, err := w.subs.Cancel(ctx, org.Id, ""); err != nil {
		t.Fatal(err)
	}
	clock.Advance(40 * day)
	if _, err := w.subs.AdvanceLifecycle(ctx, org.Id); err != nil {
		t.Fatal(err)
	}
	var st, pl string
	_ = w.pool.QueryRow(ctx, "select status, plan from subscriptions where org_id=$1", org.Id).Scan(&st, &pl)
	if st != "current" || pl != "free" {
		t.Fatalf("after the anchor the plan should have ended to free: status=%s plan=%s", st, pl)
	}
	// "Resume" no longer applies (already current/free) → 409; use "Restart".
	r, _ = w.post(t, "/v1/orgs/"+org.Id+"/subscription/reactivate", ``, ownerCk)
	if r.StatusCode != 409 {
		t.Fatalf("reactivate after the anchor must 409 (restart via changePlan), got %d", r.StatusCode)
	}
	// "Restart" = re-subscribe via changePlan (upgrade free→pro, immediate).
	r, b = w.post(t, "/v1/orgs/"+org.Id+"/subscription", `{"plan":"pro"}`, ownerCk)
	if r.StatusCode != 200 || !strings.Contains(b, `"plan":"pro"`) {
		t.Fatalf("restart (changePlan free→pro): %d %s", r.StatusCode, b)
	}
}
