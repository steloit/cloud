package identity_test

// T3.1: the estimate engine behind the contract — canon shapes priced over
// live HTTP, env fencing, and the one-shot Accept gate (consumed by T3.3).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

func TestEstimates(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "est-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"estco"}`, ownerCk)
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

	// --- canon shapes over HTTP: db-reports = $24, jobs = $21 ---------------
	resp, body = w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[
		{"product":"postgres","name":"db-reports","shape":{"size":"dev","storage_gb":10}},
		{"product":"postgres","name":"jobs","intent":"jobs","shape":{"size":"dev","storage_gb":4}}
	]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", resp.StatusCode, body)
	}
	var est struct {
		Id                string `json:"id"`
		MonthlyTotalCents int    `json:"monthly_total_cents"`
		Lines             []struct {
			Name         string `json:"name"`
			Intent       string `json:"intent"`
			MonthlyCents int    `json:"monthly_cents"`
			Basis        string `json:"basis"`
		} `json:"lines"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal([]byte(body), &est); err != nil {
		t.Fatal(err)
	}
	if est.MonthlyTotalCents != 4500 || len(est.Lines) != 2 || !strings.HasPrefix(est.Id, "est_") {
		t.Fatalf("estimate shape: %+v", est)
	}
	if est.Lines[0].MonthlyCents != 2400 || est.Lines[1].MonthlyCents != 2100 || est.Lines[1].Intent != "jobs" {
		t.Fatalf("lines: %+v", est.Lines)
	}
	if est.Lines[0].Basis != "fixed" || est.ExpiresAt == "" {
		t.Fatalf("basis/expiry: %+v", est)
	}

	// intent defaulting: postgres → database (S11)
	if est.Lines[0].Intent != "database" {
		t.Fatalf("default intent: %+v", est.Lines[0])
	}

	// --- validation: unknown size names the field; gpu names the surface ----
	resp, body = w.post(t, "/v1/estimates", `{"services":[{"product":"postgres","shape":{"size":"mega"}}]}`, ownerCk)
	if resp.StatusCode != 422 || !strings.Contains(body, "shape.size") {
		t.Fatalf("bad size: %d %s", resp.StatusCode, body)
	}
	resp, _ = w.post(t, "/v1/estimates", `{"services":[]}`, ownerCk)
	if resp.StatusCode != 422 {
		t.Fatalf("empty services: %d", resp.StatusCode)
	}

	// --- fencing: anonymous 401; foreign env 404; template 404 --------------
	resp, _ = w.post(t, "/v1/estimates", `{"services":[{"product":"web"}]}`, "")
	if resp.StatusCode != 401 {
		t.Fatalf("anonymous estimate: %d", resp.StatusCode)
	}
	strangerCk, _ := w.signupUser(t, "est-stranger@example.com")
	resp, _ = w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"web"}]}`, strangerCk)
	if resp.StatusCode != 404 {
		t.Fatalf("foreign env: %d", resp.StatusCode)
	}
	resp, _ = w.post(t, "/v1/estimates", `{"template_id":"tpl_x"}`, ownerCk)
	if resp.StatusCode != 404 {
		t.Fatalf("template estimate: %d", resp.StatusCode)
	}

	// --- Accept: one-shot, env-fenced, liveness-checked (the T3.3 gate) -----
	estSvc := estimates.NewService(store.New(w.pool))
	if _, _, err := estSvc.Accept(ctx, est.Id, env.ID); err != nil {
		t.Fatalf("first accept: %v", err)
	}
	if _, _, err := estSvc.Accept(ctx, est.Id, env.ID); err == nil {
		t.Fatal("second accept must fail — estimates are one-shot")
	}
	// wrong env
	created2, err := estSvc.Create(ctx, org.Id, env.ID, []estimates.ShapeInput{{Product: "web"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := estSvc.Accept(ctx, created2.Row.ID, "env_other"); err == nil {
		t.Fatal("cross-env accept must fail")
	}
	// expired
	if _, err := w.pool.Exec(ctx, "update estimates set expires_at = now() - interval '1 minute' where id=$1", created2.Row.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := estSvc.Accept(ctx, created2.Row.ID, env.ID); err == nil {
		t.Fatal("expired accept must fail")
	}
	// an env-less pricing preview can never be accepted
	preview, err := estSvc.Create(ctx, "", "", []estimates.ShapeInput{{Product: "worker"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := estSvc.Accept(ctx, preview.Row.ID, env.ID); err == nil {
		t.Fatal("env-less preview accepted")
	}
}
