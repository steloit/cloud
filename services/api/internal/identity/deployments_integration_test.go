package identity_test

// T4.3: deployment records over live HTTP — immutable history, per-env
// numbering, spine markers (US-4.4), rollback-creates-a-new-record.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDeployments(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "dep-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"depco"}`, ownerCk)
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

	mkService := func(name, product, shape string) string {
		resp, body := w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"`+product+`","name":"`+name+`","shape":`+shape+`}]}`, ownerCk)
		var est struct{ Id string }
		_ = json.Unmarshal([]byte(body), &est)
		resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
			`{"name":"`+name+`","product":"`+product+`","estimate_id":"`+est.Id+`","shape":`+shape+`}`, ownerCk)
		if resp.StatusCode != 201 {
			t.Fatalf("create %s: %d %s", name, resp.StatusCode, body)
		}
		var svc struct{ Id string }
		_ = json.Unmarshal([]byte(body), &svc)
		return svc.Id
	}
	api := mkService("api", "web", `{"size":"standard-1","instances":1}`)
	db := mkService("db", "postgres", `{"size":"dev"}`)

	// databases don't take deployments
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/deployments", `{"service":"`+db+`","git_sha":"aaa1111"}`, ownerCk)
	if resp.StatusCode != 422 || !strings.Contains(body, "compute") {
		t.Fatalf("db deployment: %d %s", resp.StatusCode, body)
	}

	// --- create: numbered from 1, queued, spine marker with number + sha ----
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/deployments", `{"service":"`+api+`","git_sha":"aaa1111"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createDeployment: %d %s", resp.StatusCode, body)
	}
	var d1 struct {
		Id, State string
		Number    int
	}
	_ = json.Unmarshal([]byte(body), &d1)
	if d1.Number != 1 || d1.State != "queued" {
		t.Fatalf("d1: %+v", d1)
	}
	var n int
	if err := w.pool.QueryRow(ctx,
		`select count(*) from events where org_id=$1 and kind='deploy' and detail->>'number'='1' and detail->>'sha'='aaa1111'`,
		org.Id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("deploy marker: %d %v", n, err)
	}

	// second deployment numbers 2
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/deployments", `{"service":"`+api+`","git_sha":"bbb2222"}`, ownerCk)
	var d2 struct{ Id string; Number int }
	_ = json.Unmarshal([]byte(body), &d2)
	if d2.Number != 2 {
		t.Fatalf("d2 number: %d", d2.Number)
	}

	// --- immutability at the DB level ---------------------------------------
	if _, err := w.pool.Exec(ctx, "delete from deployments where id=$1", d1.Id); err == nil ||
		!strings.Contains(err.Error(), "immutable") {
		t.Fatalf("DELETE did not raise: %v", err)
	}
	if _, err := w.pool.Exec(ctx, "update deployments set git_sha='hacked' where id=$1", d1.Id); err == nil ||
		!strings.Contains(err.Error(), "identity columns") {
		t.Fatalf("identity UPDATE did not raise: %v", err)
	}
	// lifecycle columns may advance (the pipeline's path)
	if _, err := w.pool.Exec(ctx, "update deployments set state='building' where id=$1 and state='queued'", d1.Id); err != nil {
		t.Fatalf("lifecycle update blocked: %v", err)
	}

	// --- rollback: needs an earlier SUCCESSFUL deployment -------------------
	resp, body = w.post(t, "/v1/deployments/"+d2.Id+"/rollback", "", ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "no earlier successful") {
		t.Fatalf("rollback without history: %d %s", resp.StatusCode, body)
	}
	// drive d1 live through the machine, then d2 live
	for _, step := range [][2]string{{"building", "migrating"}, {"migrating", "canary"}, {"canary", "verifying"}, {"verifying", "live"}} {
		if _, err := w.pool.Exec(ctx, "update deployments set state=$2 where id=$1 and state=$3", d1.Id, step[1], step[0]); err != nil {
			t.Fatal(err)
		}
	}
	resp, body = w.post(t, "/v1/deployments/"+d2.Id+"/rollback", "", ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("rollback: %d %s", resp.StatusCode, body)
	}
	var rbFull struct {
		Id     string `json:"id"`
		GitSha string `json:"git_sha"`
		Number int    `json:"number"`
	}
	_ = json.Unmarshal([]byte(body), &rbFull)
	if rbFull.Number != 3 || rbFull.GitSha != "aaa1111" {
		t.Fatalf("rollback record: %+v", rbFull)
	}
	var rollbackOf string
	if err := w.pool.QueryRow(ctx, "select rollback_of from deployments where id=$1", rbFull.Id).Scan(&rollbackOf); err != nil || rollbackOf != d2.Id {
		t.Fatalf("rollback_of: %s %v", rollbackOf, err)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='deploy.rolled_back'", org.Id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rollback event: %d %v", n, err)
	}

	// history: newest first, all three records, nothing rewritten
	resp, body = w.get(t, "/v1/envs/"+env.ID+"/deployments", ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("list: %d", resp.StatusCode)
	}
	var list struct {
		Data []struct {
			Number int
			GitSha string `json:"git_sha"`
		}
	}
	_ = json.Unmarshal([]byte(body), &list)
	if len(list.Data) != 3 || list.Data[0].Number != 3 || list.Data[2].GitSha != "aaa1111" {
		t.Fatalf("history: %+v", list.Data)
	}
}
