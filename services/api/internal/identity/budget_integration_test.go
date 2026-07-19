package identity_test

// T11.6: the flagship hard spend cap against a real DB — set a budget, the
// overview reports it, an estimate that would cross the cap is refused at the
// accept gate with the arithmetic shown, raising the cap lets it through, and
// CSV exports stream the same rows.

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBudgetCapAndExports(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "cap-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"capco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	orgRow, err := w.svc.GetOrg(ctx, org.Id)
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}

	// --- set a very low cap ($1) and read it back on the overview ------------
	if r, b := w.put(t, "/v1/orgs/"+org.Id+"/billing/budget", `{"limit_cents":100,"alert_thresholds":[80]}`, ownerCk); r.StatusCode != 200 {
		t.Fatalf("setBudget: %d %s", r.StatusCode, b)
	}
	_, ob := w.get(t, "/v1/orgs/"+org.Id+"/billing/overview", ownerCk)
	if !strings.Contains(ob, `"limit_cents":100`) {
		t.Fatalf("overview missing the budget: %s", ob)
	}

	// --- an estimate that crosses the $1 cap is REFUSED at accept, with math --
	_, eb := w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(eb), &est)
	r, cb := w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if r.StatusCode != 402 || !strings.Contains(cb, "cap") {
		t.Fatalf("over-cap create should 402 (quota_exceeded family) naming the cap: %d %s", r.StatusCode, cb)
	}
	// US-11.7: the refusal carries the sanctioned catalog type + Payment Required.
	if !strings.Contains(cb, "quota_exceeded") {
		t.Fatalf("over-cap refusal must be the 402 quota_exceeded type: %s", cb)
	}
	// AC3: every cap hit lands on the events spine (auditable, not just a toast).
	var capEvents int
	_ = w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='billing.spend_cap_reached'", org.Id).Scan(&capEvents)
	if capEvents < 1 {
		t.Fatalf("over-cap refusal must emit a billing.spend_cap_reached spine event, found %d", capEvents)
	}
	// the event payload carries the arithmetic (projected == current + requested).
	var capDetail []byte
	_ = w.pool.QueryRow(ctx, "select detail from events where org_id=$1 and action='billing.spend_cap_reached' order by at desc limit 1", org.Id).Scan(&capDetail)
	var cd struct {
		CapCents       int64 `json:"cap_cents"`
		CurrentCents   int64 `json:"current_cents"`
		RequestedCents int64 `json:"requested_cents"`
		ProjectedCents int64 `json:"projected_cents"`
	}
	_ = json.Unmarshal(capDetail, &cd)
	if cd.CapCents != 100 || cd.ProjectedCents != cd.CurrentCents+cd.RequestedCents || cd.ProjectedCents <= cd.CapCents {
		t.Fatalf("cap event payload arithmetic wrong: %s", string(capDetail))
	}
	// blocked-not-warned: the service was NOT created.
	var svcCount int
	_ = w.pool.QueryRow(ctx, "select count(*) from services s join environments e on s.env_id=e.id join projects p on e.project_id=p.id where p.org_id=$1", org.Id).Scan(&svcCount)
	if svcCount != 0 {
		t.Fatalf("over-cap provision was NOT blocked — %d service(s) created despite the cap", svcCount)
	}
	// the math is shown (a dollar figure appears in the refusal)
	if !strings.Contains(cb, "$") {
		t.Fatalf("cap refusal did not show the arithmetic: %s", cb)
	}
	// the one-shot estimate was NOT burned (cap refused before accept) — reuse it.

	// --- raise the cap; the SAME estimate now provisions ---------------------
	if r, b := w.put(t, "/v1/orgs/"+org.Id+"/billing/budget", `{"limit_cents":100000000}`, ownerCk); r.StatusCode != 200 {
		t.Fatalf("raise budget: %d %s", r.StatusCode, b)
	}
	r, cb2 := w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("under-cap create should succeed: %d %s", r.StatusCode, cb2)
	}
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(cb2), &svc)

	// --- a scale-UP must respect the cap too (the create-only bypass) --------
	// tighten the cap to the current run-rate (forecast) — no headroom — then
	// try to scale the service up; it must be refused, not silently grow spend.
	_, ov := w.get(t, "/v1/orgs/"+org.Id+"/billing/overview", ownerCk)
	var overview struct {
		ForecastCents int `json:"forecast_cents"`
	}
	_ = json.Unmarshal([]byte(ov), &overview)
	if r, b := w.put(t, "/v1/orgs/"+org.Id+"/billing/budget", `{"limit_cents":`+strconv.Itoa(overview.ForecastCents)+`}`, ownerCk); r.StatusCode != 200 {
		t.Fatalf("tighten cap: %d %s", r.StatusCode, b)
	}
	sr, sb := w.patch(t, "/v1/services/"+svc.Id, `{"shape":{"size":"standard","storage_gb":50}}`, ownerCk)
	if sr.StatusCode != 402 || !strings.Contains(sb, "cap") {
		t.Fatalf("scale-up past the cap must 402 (create-only enforcement is a bypass): %d %s", sr.StatusCode, sb)
	}
	// the SCALE path emits the cap event too (not just create).
	var capEvents2 int
	_ = w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='billing.spend_cap_reached'", org.Id).Scan(&capEvents2)
	if capEvents2 < 2 {
		t.Fatalf("scale-up over the cap must also emit a spine event (create + scale = 2), found %d", capEvents2)
	}

	// --- budget changes are audited on the spine -----------------------------
	var audits int
	_ = w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='billing.budget_changed'", org.Id).Scan(&audits)
	if audits < 3 { // $1, raise, tighten
		t.Fatalf("budget changes not audited: %d billing.budget_changed events", audits)
	}

	// --- removing the cap (null) leaves it uncapped --------------------------
	if r, _ := w.put(t, "/v1/orgs/"+org.Id+"/billing/budget", `{"limit_cents":null}`, ownerCk); r.StatusCode != 200 {
		t.Fatal("remove cap")
	}

	// --- CSV exports stream the rows -----------------------------------------
	er, ub := w.get(t, "/v1/orgs/"+org.Id+"/billing/usage:export", ownerCk)
	if er.StatusCode != 200 || er.Header.Get("Content-Type") == "" || !strings.Contains(ub, "period,meter,used,rate_cents") {
		t.Fatalf("usage export: %d %q %s", er.StatusCode, er.Header.Get("Content-Type"), ub)
	}
	ir, ib := w.get(t, "/v1/orgs/"+org.Id+"/billing/invoices:export", ownerCk)
	if ir.StatusCode != 200 || !strings.Contains(ib, "id,period,status,total_cents,lines") {
		t.Fatalf("invoices export: %d %s", ir.StatusCode, ib)
	}
}

// TestBudgetCapBoundaryAndUncapped — pins the ±1¢ cap boundary (exactly-at-cap
// provisions; one cent under refuses) and that an uncapped org provisions
// freely (both enforceBudget return-nil branches: no budget row AND null limit).
func TestBudgetCapBoundaryAndUncapped(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "capb-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"capbco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	orgRow, _ := w.svc.GetOrg(ctx, org.Id)
	_, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}

	// UNCAPPED (no budget row set) → a paid provision succeeds freely.
	mkEstimate := func() string {
		_, eb := w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
		var est struct{ Id string }
		_ = json.Unmarshal([]byte(eb), &est)
		return est.Id
	}
	r, cb := w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"u1","product":"postgres","estimate_id":"`+mkEstimate()+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("uncapped (no budget row) provision must succeed: %d %s", r.StatusCode, cb)
	}
	// the monthly cost of what we just provisioned = the committed sum now.
	var committed int64
	_ = w.pool.QueryRow(ctx, "select coalesce(sum(monthly_estimate_cents),0) from services s join environments e on s.env_id=e.id join projects p on e.project_id=p.id where p.org_id=$1 and s.status <> 'deleting'", org.Id).Scan(&committed)

	// a second dev/10GB service costs the SAME monthly. Set the cap to EXACTLY
	// committed + planFee + nextMonthly → the boundary provision is allowed.
	var nextMonthly int64
	_, eb2 := w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"postgres","name":"u2","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	var est2 struct {
		Id                string
		MonthlyTotalCents int64 `json:"monthly_total_cents"`
	}
	_ = json.Unmarshal([]byte(eb2), &est2)
	nextMonthly = est2.MonthlyTotalCents
	// free plan fee is 0, so the cap = committed + nextMonthly is the exact edge.
	exact := committed + nextMonthly
	if r, b := w.put(t, "/v1/orgs/"+org.Id+"/billing/budget", `{"limit_cents":`+strconv.FormatInt(exact, 10)+`}`, ownerCk); r.StatusCode != 200 {
		t.Fatalf("set exact cap: %d %s", r.StatusCode, b)
	}
	r, cb = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"u2","product":"postgres","estimate_id":"`+est2.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("exactly-at-cap provision must be ALLOWED (projected == cap): %d %s", r.StatusCode, cb)
	}

	// now one cent UNDER the needed total → the next identical provision refuses.
	var committed2 int64
	_ = w.pool.QueryRow(ctx, "select coalesce(sum(monthly_estimate_cents),0) from services s join environments e on s.env_id=e.id join projects p on e.project_id=p.id where p.org_id=$1 and s.status <> 'deleting'", org.Id).Scan(&committed2)
	_, eb3 := w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"postgres","name":"u3","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	var est3 struct {
		Id                string
		MonthlyTotalCents int64 `json:"monthly_total_cents"`
	}
	_ = json.Unmarshal([]byte(eb3), &est3)
	if r, b := w.put(t, "/v1/orgs/"+org.Id+"/billing/budget", `{"limit_cents":`+strconv.FormatInt(committed2+est3.MonthlyTotalCents-1, 10)+`}`, ownerCk); r.StatusCode != 200 {
		t.Fatalf("set cap-1: %d %s", r.StatusCode, b)
	}
	r, cb = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"u3","product":"postgres","estimate_id":"`+est3.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if r.StatusCode != 402 {
		t.Fatalf("one cent under the total must 402 (boundary): %d %s", r.StatusCode, cb)
	}

	// null limit → uncapped again → provisions.
	if r, _ := w.put(t, "/v1/orgs/"+org.Id+"/billing/budget", `{"limit_cents":null}`, ownerCk); r.StatusCode != 200 {
		t.Fatal("null the cap")
	}
	r, cb = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"u3","product":"postgres","estimate_id":"`+est3.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("null cap must provision freely: %d %s", r.StatusCode, cb)
	}
}
