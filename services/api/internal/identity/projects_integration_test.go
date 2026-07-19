package identity_test

// T3.2: projects + environments over live HTTP — implicit production env
// (ADR-037), plan gate (Free 1 / Pro 3), the closed EnvResolver seam,
// non-member 404s, and D8 (cell_id never surfaces).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProjectsAndEnvironments(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "prj-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"prjco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	// --- createProject: 201; the implicit production env is BORN ------------
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/projects", `{"name":"ecommerce"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createProject: %d %s", resp.StatusCode, body)
	}
	var prj struct {
		Id       string
		EnvCount int `json:"env_count"`
	}
	_ = json.Unmarshal([]byte(body), &prj)
	if !strings.HasPrefix(prj.Id, "prj_") || prj.EnvCount != 1 {
		t.Fatalf("project shape: %+v", prj)
	}
	if strings.Contains(body, "cell") {
		t.Fatalf("substrate leaked into the response (D8): %s", body)
	}
	resp, body = w.get(t, "/v1/projects/"+prj.Id+"/envs", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, `"name":"production"`) {
		t.Fatalf("implicit env missing: %d %s", resp.StatusCode, body)
	}
	var envList struct {
		Data []struct{ Id, Name, Region string }
	}
	_ = json.Unmarshal([]byte(body), &envList)
	if len(envList.Data) != 1 || envList.Data[0].Region != "aws/ap-south-1" {
		t.Fatalf("production env: %+v", envList.Data)
	}
	prodEnv := envList.Data[0].Id

	// --- the events seam is CLOSED: /envs/{env}/events through real rows ----
	resp, body = w.get(t, "/v1/envs/"+prodEnv+"/events", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, "project.created") {
		t.Fatalf("env events through real resolver: %d %s", resp.StatusCode, body)
	}

	// --- plan gate: Free allows 1 project → 402 naming pro ------------------
	resp, body = w.post(t, "/v1/orgs/"+org.Id+"/projects", `{"name":"second"}`, ownerCk)
	if resp.StatusCode != 402 || !strings.Contains(body, "pro") {
		t.Fatalf("plan gate: %d %s", resp.StatusCode, body)
	}
	if _, err := w.pool.Exec(ctx, "update orgs set plan='pro' where id=$1", org.Id); err != nil {
		t.Fatal(err)
	}
	resp, _ = w.post(t, "/v1/orgs/"+org.Id+"/projects", `{"name":"second"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("post-upgrade create: %d", resp.StatusCode)
	}
	// duplicate name in org → 409
	resp, _ = w.post(t, "/v1/orgs/"+org.Id+"/projects", `{"name":"ecommerce"}`, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("dup project name: %d", resp.StatusCode)
	}

	// --- get / list / rename ------------------------------------------------
	resp, body = w.get(t, "/v1/projects/"+prj.Id, ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, `"monthly_cost_cents":0`) {
		t.Fatalf("getProject: %d %s", resp.StatusCode, body)
	}
	resp, body = w.get(t, "/v1/orgs/"+org.Id+"/projects", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, "ecommerce") || !strings.Contains(body, "second") {
		t.Fatalf("listProjects: %d %s", resp.StatusCode, body)
	}
	resp, body = w.patch(t, "/v1/projects/"+prj.Id, `{"name":"shop"}`, ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, "shop") {
		t.Fatalf("rename: %d %s", resp.StatusCode, body)
	}

	// --- environments: create, dedupe, gated branch/clone -------------------
	resp, body = w.post(t, "/v1/projects/"+prj.Id+"/envs", `{"name":"staging","region":"aws/eu-west-1"}`, ownerCk)
	if resp.StatusCode != 201 || !strings.Contains(body, "aws/eu-west-1") {
		t.Fatalf("createEnvironment: %d %s", resp.StatusCode, body)
	}
	resp, _ = w.post(t, "/v1/projects/"+prj.Id+"/envs", `{"name":"staging"}`, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("dup env: %d", resp.StatusCode)
	}
	resp, body = w.post(t, "/v1/projects/"+prj.Id+"/envs", `{"name":"branchy","data":"branch"}`, ownerCk)
	if resp.StatusCode != 422 || !strings.Contains(body, "T3.4") {
		t.Fatalf("branch env must be refused loudly until the driver exists: %d %s", resp.StatusCode, body)
	}

	// --- delete: blocked by the EXPLICIT env, never by implicit production --
	resp, body = w.del(t, "/v1/projects/"+prj.Id, ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "staging") || strings.Contains(body, "production exists") {
		t.Fatalf("delete blockers: %d %s", resp.StatusCode, body)
	}
	// the 1-env project deletes fine (implicit production never blocks)
	var second struct{ Id string }
	resp, body = w.get(t, "/v1/orgs/"+org.Id+"/projects", ownerCk)
	var pl struct {
		Data []struct{ Id, Name string }
	}
	_ = json.Unmarshal([]byte(body), &pl)
	for _, p := range pl.Data {
		if p.Name == "second" {
			second.Id = p.Id
		}
	}
	resp, _ = w.del(t, "/v1/projects/"+second.Id, ownerCk)
	if resp.StatusCode != 202 {
		t.Fatalf("delete 1-env project: %d", resp.StatusCode)
	}
	resp, _ = w.del(t, "/v1/projects/"+second.Id, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("re-delete: %d", resp.StatusCode)
	}

	// --- non-members: 404 on every project-scoped path ----------------------
	strangerCk, _ := w.signupUser(t, "prj-stranger@example.com")
	for _, path := range []string{
		"/v1/projects/" + prj.Id,
		"/v1/projects/" + prj.Id + "/envs",
	} {
		resp, _ = w.get(t, path, strangerCk)
		if resp.StatusCode != 404 {
			t.Fatalf("stranger %s: %d (want 404)", path, resp.StatusCode)
		}
	}
	// developer CAN create envs (env.manage Y) but billing cannot
	_, billingID := w.signupUser(t, "prj-billing@example.com")
	if err := w.svc.AddMember(ctx, org.Id, billingID, "billing", ownerID); err != nil {
		t.Fatal(err)
	}
	billingCk, _ := w.loginCookie(t, "prj-billing@example.com")
	resp, body = w.post(t, "/v1/projects/"+prj.Id+"/envs", `{"name":"nope"}`, billingCk)
	if resp.StatusCode != 403 || !strings.Contains(body, "role:billing") {
		t.Fatalf("billing env create: %d %s", resp.StatusCode, body)
	}

	// template path is honest: 404 template until E9
	resp, _ = w.post(t, "/v1/orgs/"+org.Id+"/projects", `{"name":"tpl","template_id":"tpl_x"}`, ownerCk)
	if resp.StatusCode != 404 {
		t.Fatalf("template create: %d", resp.StatusCode)
	}
}
