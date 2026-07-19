package identity_test

// US-11.5 — B12: Cancel ≠ Delete. Cancelling a subscription changes the PLAN
// (ends at the anchor), never the RESOURCES: services keep running and keep
// metering. The way back is one click ("Resume" before the anchor uncancels;
// "Restart" after re-subscribes via changePlan) — as clean as the way out.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/metering"
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

// TestReactivateFromDunningRefused — reactivate only uncancels a
// cancelled_at_anchor sub; it must NOT resurrect a dunning (grace/suspended)
// org to current, which would silently lift enforcement without payment.
func TestReactivateFromDunningRefused(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, _ := w.signupUser(t, "react-dun-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"rdunco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	if _, err := w.pool.Exec(ctx, "update subscriptions set plan='pro' where org_id=$1", org.Id); err != nil {
		t.Fatal(err)
	}
	// enter dunning (grace)
	if _, err := w.subs.FailPayment(ctx, org.Id); err != nil {
		t.Fatal(err)
	}
	// reactivate must 409 and NOT touch the dunning state.
	r, _ := w.post(t, "/v1/orgs/"+org.Id+"/subscription/reactivate", ``, ownerCk)
	if r.StatusCode != 409 {
		t.Fatalf("reactivate from grace must 409 (no dunning resurrection), got %d", r.StatusCode)
	}
	var st string
	var dun bool
	_ = w.pool.QueryRow(ctx, "select status, dunning_started_at is not null from subscriptions where org_id=$1", org.Id).Scan(&st, &dun)
	if st != "grace" || !dun {
		t.Fatalf("reactivate mutated the dunning track: status=%s dunning=%v", st, dun)
	}
}

// TestCancelledOrgStillBilledAtRollup — the MONEY-layer proof of "keeps
// metering": a cancelled org with an open span still bills at rollup (usage is
// driven by spans, not subscription status).
func TestCancelledOrgStillBilledAtRollup(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	_, ownerID := w.signupUser(t, "bill-cancel-owner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "billcancelco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, "update subscriptions set plan='pro' where org_id=$1", org.ID); err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.ID)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	period := metering.Period(t0)
	// an OPEN span (running service), planted at t0.
	if _, err := w.pool.Exec(ctx,
		`insert into usage_events (id, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
		 values ('use_bc', $1, $2, $3, 'svc_bc', 'service_span', 'open', 'postgres', 2400, $4)`,
		org.ID, prj.ID, env.ID, t0); err != nil {
		t.Fatal(err)
	}
	// CANCEL the subscription (after the span opened).
	if _, err := w.subs.Cancel(ctx, org.ID, ""); err != nil {
		t.Fatal(err)
	}
	// roll up 2h later — the cancelled org STILL accrues billable seconds.
	em := metering.NewEmitter(store.New(w.pool))
	if err := em.Rollup(ctx, org.ID, period, t0.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var used int64
	if err := w.pool.QueryRow(ctx,
		`select used from quota_usage where org_id=$1 and meter='service_span_seconds' and period=$2`,
		org.ID, period).Scan(&used); err != nil {
		t.Fatalf("no rollup row for the cancelled org — metering stopped at cancel: %v", err)
	}
	if used <= 0 {
		t.Fatalf("cancelled org billed 0 seconds — Cancel ≠ Delete requires metering to keep running, got %d", used)
	}
}

// TestReactivateAuthzFencing — reactivate is a mutation: read_only tokens 403,
// foreign non-members 404 (no org-existence leak).
func TestReactivateAuthzFencing(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, _ := w.signupUser(t, "react-authz-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"razco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	if _, err := w.pool.Exec(ctx, "update subscriptions set plan='pro' where org_id=$1", org.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.subs.Cancel(ctx, org.Id, ""); err != nil {
		t.Fatal(err)
	}
	// read_only token → 403
	r, tb := w.post(t, "/v1/me/tokens", `{"name":"ro","scope":"read_only"}`, ownerCk)
	var tk struct{ Token string }
	_ = json.Unmarshal([]byte(tb), &tk)
	req, _ := http.NewRequest(http.MethodPost, w.srv.URL+"/v1/orgs/"+org.Id+"/subscription/reactivate", nil)
	req.Header.Set("Authorization", "Bearer "+tk.Token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatalf("read_only token reactivate must 403, got %d", res.StatusCode)
	}
	// foreign non-member → 403 (the whole billing surface uses requireOrg, which
	// denies non-members uniformly — a non-existent org also 403s, so no
	// existence oracle; consistent with cancel/changePlan).
	strangerCk, _ := w.signupUser(t, "react-stranger@example.com")
	r, _ = w.post(t, "/v1/orgs/"+org.Id+"/subscription/reactivate", ``, strangerCk)
	if r.StatusCode != 403 {
		t.Fatalf("non-member reactivate must 403 (billing-surface convention), got %d", r.StatusCode)
	}
}
