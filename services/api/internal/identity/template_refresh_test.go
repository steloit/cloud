package identity_test

// T12.5 — refresh-as-new-version (T2): POST /templates/{tpl}/refresh re-captures
// from the live source, MINTS a new version (version+1), and PRESERVES the
// informational used_by_count (instantiations are copies, never linked). A
// refresh whose source no longer exists is refused loudly (the template stays
// usable as-is).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTemplateRefreshVersioning(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "tpl-refresh-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"tprco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	if _, err := w.pool.Exec(ctx, "update orgs set plan='business' where id=$1", org.Id); err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.Id)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_ = prj

	mk := func(name, product, shape string) {
		r, b := w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"`+product+`","name":"`+name+`","shape":`+shape+`}]}`, ownerCk)
		if r.StatusCode != 200 {
			t.Fatalf("estimate %s: %d %s", name, r.StatusCode, b)
		}
		var est struct{ Id string }
		_ = json.Unmarshal([]byte(b), &est)
		r, b = w.post(t, "/v1/envs/"+env.ID+"/services",
			`{"name":"`+name+`","product":"`+product+`","estimate_id":"`+est.Id+`","shape":`+shape+`}`, ownerCk)
		if r.StatusCode != 201 {
			t.Fatalf("create %s: %d %s", name, r.StatusCode, b)
		}
	}
	// source env starts with just a db (no excluded bindings → no required inputs).
	mk("db", "postgres", `{"size":"dev","storage_gb":10}`)

	// --- capture → version 1 (capture takes service IDs) ---------------------
	var dbID string
	_ = w.pool.QueryRow(ctx, "select id from services where env_id=$1 and name='db'", env.ID).Scan(&dbID)
	r, tb := w.post(t, "/v1/orgs/"+org.Id+"/templates",
		`{"name":"data-stack","source":{"project":"`+prj.ID+`","env":"`+env.ID+`"},"services":["`+dbID+`"]}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("capture: %d %s", r.StatusCode, tb)
	}
	var tpl struct {
		Id                   string `json:"id"`
		Version              int    `json:"version"`
		UsedByCount          int    `json:"used_by_count"`
		MonthlyEstimateCents int    `json:"monthly_estimate_cents"`
	}
	_ = json.Unmarshal([]byte(tb), &tpl)
	if tpl.Version != 1 {
		t.Fatalf("captured template must be version 1, got %d", tpl.Version)
	}

	// --- instantiate → used_by_count bumps to 1 ------------------------------
	r, ib := w.post(t, "/v1/orgs/"+org.Id+"/projects",
		`{"name":"data-copy","template_id":"`+tpl.Id+`"}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("instantiate: %d %s", r.StatusCode, ib)
	}
	var usedBefore int
	_ = w.pool.QueryRow(ctx, "select used_by_count from templates where id=$1", tpl.Id).Scan(&usedBefore)
	if usedBefore != 1 {
		t.Fatalf("instantiation must bump used_by_count to 1, got %d", usedBefore)
	}

	// --- change the source (add a cache), then REFRESH -----------------------
	mk("cache", "valkey", `{"memory_mb":1024}`)
	r, rb := w.post(t, "/v1/templates/"+tpl.Id+"/refresh", ``, ownerCk)
	if r.StatusCode != 200 {
		t.Fatalf("refresh: %d %s", r.StatusCode, rb)
	}
	var refreshed struct {
		Version              int `json:"version"`
		UsedByCount          int `json:"used_by_count"`
		MonthlyEstimateCents int `json:"monthly_estimate_cents"`
	}
	_ = json.Unmarshal([]byte(rb), &refreshed)
	// version MINTED (v1 → v2)
	if refreshed.Version != 2 {
		t.Fatalf("refresh must mint a new version (2), got %d", refreshed.Version)
	}
	// used_by_count PRESERVED across the refresh
	if refreshed.UsedByCount != 1 {
		t.Fatalf("refresh must preserve used_by_count (1), got %d", refreshed.UsedByCount)
	}
	// contents re-captured from the live source — the newly-added service appears.
	// NOTE (recorded finding): refresh re-captures the WHOLE source env
	// (captureFrom(env, nil)), NOT the original service selection — the template
	// persists source_project/source_env but not the captured ids, so an added
	// (or previously-excluded) service reappears. This PINS that broadening;
	// whether refresh should honor the original selection is a T2 design question.
	if !strings.Contains(rb, "cache") {
		t.Fatalf("refresh must re-capture the full source env (cache added → appears): %s", rb)
	}

	// --- refresh with the source GONE → refused loudly, template intact ------
	if _, err := w.pool.Exec(ctx, "delete from projects where id=$1", prj.ID); err != nil {
		t.Fatal(err)
	}
	r, gb := w.post(t, "/v1/templates/"+tpl.Id+"/refresh", ``, ownerCk)
	if r.StatusCode != 409 || !strings.Contains(gb, "no longer exists") {
		t.Fatalf("refresh with a deleted source must 409 (stays usable as-is): %d %s", r.StatusCode, gb)
	}
	// the template is untouched — still version 2, still used_by_count 1.
	var v, u int
	_ = w.pool.QueryRow(ctx, "select version, used_by_count from templates where id=$1", tpl.Id).Scan(&v, &u)
	if v != 2 || u != 1 {
		t.Fatalf("a failed refresh mutated the template: version=%d used_by_count=%d (want 2, 1)", v, u)
	}
}

// TestRefreshRequiresManage — refresh mutates the template, so it needs
// template.manage; a member without it gets denied and a non-member 404s.
func TestRefreshRequiresManage(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "tpl-rz-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"trzco"}`, ownerCk)
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
	r, b := w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(b), &est)
	r, _ = w.post(t, "/v1/envs/"+env.ID+"/services", `{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("create svc: %d", r.StatusCode)
	}
	var dbID, prjID string
	_ = w.pool.QueryRow(ctx, "select id from services where env_id=$1 and name='db'", env.ID).Scan(&dbID)
	_ = w.pool.QueryRow(ctx, "select project_id from environments where id=$1", env.ID).Scan(&prjID)
	r, tb := w.post(t, "/v1/orgs/"+org.Id+"/templates",
		`{"name":"t","source":{"project":"`+prjID+`","env":"`+env.ID+`"},"services":["`+dbID+`"]}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("capture: %d %s", r.StatusCode, tb)
	}
	var tpl struct{ Id string }
	_ = json.Unmarshal([]byte(tb), &tpl)

	// a developer member (template.consume, NOT template.manage) is denied.
	devCk, devID := w.signupUser(t, "tpl-rz-dev@example.com")
	if _, err := w.pool.Exec(ctx, "insert into members (id,org_id,user_id,role) values ('mbr_rz',$1,$2,'developer')", org.Id, devID); err != nil {
		t.Fatal(err)
	}
	r, _ = w.post(t, "/v1/templates/"+tpl.Id+"/refresh", ``, devCk)
	if r.StatusCode == 200 {
		t.Fatal("a developer without template.manage refreshed the template")
	}
	// a total non-member → 404 (no existence oracle).
	strangerCk, _ := w.signupUser(t, "tpl-rz-stranger@example.com")
	r, _ = w.post(t, "/v1/templates/"+tpl.Id+"/refresh", ``, strangerCk)
	if r.StatusCode != 404 {
		t.Fatalf("non-member refresh must 404, got %d", r.StatusCode)
	}
}
