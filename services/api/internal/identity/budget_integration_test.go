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
	if r.StatusCode != 409 || !strings.Contains(cb, "cap") {
		t.Fatalf("over-cap create should 409 naming the cap: %d %s", r.StatusCode, cb)
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
	if sr.StatusCode != 409 || !strings.Contains(sb, "cap") {
		t.Fatalf("scale-up past the cap must 409 (create-only enforcement is a bypass): %d %s", sr.StatusCode, sb)
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
