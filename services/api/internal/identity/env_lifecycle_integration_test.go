package identity_test

// T4.7: environment rename + teardown over live HTTP — the implicit env
// never deletes (identity by FLAG, so it survives its own rename), blockers
// named, ADR-037 semantics end-to-end.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEnvironmentLifecycle(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "envlc-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"envlc"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	orgRow, err := w.svc.GetOrg(ctx, org.Id)
	if err != nil {
		t.Fatal(err)
	}
	prj, prod, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	staging, err := w.prov.CreateEnvironment(ctx, prj, "staging", "", false, false, ownerID)
	if err != nil {
		t.Fatal(err)
	}

	// --- rename: the ADR-037 escape hatch, production included -------------
	resp, body = w.patch(t, "/v1/envs/"+prod.ID, `{"name":"live"}`, ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, `"name":"live"`) {
		t.Fatalf("rename production: %d %s", resp.StatusCode, body)
	}
	// duplicate name → 409
	resp, _ = w.patch(t, "/v1/envs/"+staging.ID, `{"name":"live"}`, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("dup rename: %d", resp.StatusCode)
	}
	var n int
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='env.renamed'", org.Id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rename event: %d %v", n, err)
	}

	// --- the implicit env NEVER deletes — even under its new name ----------
	resp, body = w.del(t, "/v1/envs/"+prod.ID, ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "project deletion") {
		t.Fatalf("implicit delete: %d %s", resp.StatusCode, body)
	}

	// --- explicit env with services: every blocker NAMED --------------------
	respE, bodyE := w.post(t, "/v1/estimates", `{"env":"`+staging.ID+`","services":[{"product":"worker","name":"w1","shape":{}}]}`, ownerCk)
	if respE.StatusCode != 200 {
		t.Fatalf("estimate: %d %s", respE.StatusCode, bodyE)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(bodyE), &est)
	respE, bodyE = w.post(t, "/v1/envs/"+staging.ID+"/services",
		`{"name":"w1","product":"worker","estimate_id":"`+est.Id+`","shape":{}}`, ownerCk)
	if respE.StatusCode != 201 {
		t.Fatalf("create w1: %d %s", respE.StatusCode, bodyE)
	}
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(bodyE), &svc)

	resp, body = w.del(t, "/v1/envs/"+staging.ID, ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "w1") {
		t.Fatalf("blockers named: %d %s", resp.StatusCode, body)
	}
	// delete the service (→ deleting), then teardown schedules
	if r, _ := w.del(t, "/v1/services/"+svc.Id, ownerCk); r.StatusCode != 202 {
		t.Fatal("service delete")
	}
	resp, _ = w.del(t, "/v1/envs/"+staging.ID, ownerCk)
	if resp.StatusCode != 202 {
		t.Fatalf("env teardown: %d", resp.StatusCode)
	}
	var scheduled bool
	if err := w.pool.QueryRow(ctx, "select deletion_scheduled_at is not null from environments where id=$1", staging.ID).Scan(&scheduled); err != nil || !scheduled {
		t.Fatalf("teardown not scheduled: %v %v", scheduled, err)
	}
	// idempotence: second teardown → 409
	resp, _ = w.del(t, "/v1/envs/"+staging.ID, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("re-teardown: %d", resp.StatusCode)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='env.deletion_scheduled'", org.Id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("teardown event: %d %v", n, err)
	}

	// --- project deletion still exempts the implicit env under its new name -
	// (the "shop" project has envs live(implicit) + staging(scheduled): both
	// exempt → deletion schedules)
	resp, body = w.del(t, "/v1/projects/"+prj.ID, ownerCk)
	if resp.StatusCode != 202 {
		t.Fatalf("project delete with renamed implicit env: %d %s", resp.StatusCode, body)
	}

	// --- RBAC: billing can't manage envs ------------------------------------
	_, billingID := w.signupUser(t, "envlc-billing@example.com")
	if err := w.svc.AddMember(ctx, org.Id, billingID, "billing", ownerID); err != nil {
		t.Fatal(err)
	}
	billingCk, _ := w.loginCookie(t, "envlc-billing@example.com")
	resp, body = w.patch(t, "/v1/envs/"+prod.ID, `{"name":"nope"}`, billingCk)
	if resp.StatusCode != 403 || !strings.Contains(body, "role:billing") {
		t.Fatalf("billing rename: %d %s", resp.StatusCode, body)
	}
}
