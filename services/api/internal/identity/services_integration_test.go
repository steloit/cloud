package identity_test

// T3.3: services over live HTTP — the estimate gate enforced AT THE API
// LAYER (US-3.2's law), the guarded status machine, D22 update semantics.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestServices(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "svc-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"svcco"}`, ownerCk)
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

	// --- the estimate gate: nothing provisions without an accepted estimate -
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db-reports","product":"postgres","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 422 || !strings.Contains(body, "estimate_id") {
		t.Fatalf("no-estimate create must 422 naming estimate_id: %d %s", resp.StatusCode, body)
	}
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db-reports","product":"postgres","estimate_id":"est_bogus","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "estimate") {
		t.Fatalf("bogus estimate: %d %s", resp.StatusCode, body)
	}

	// estimate the exact shape, then create
	resp, body = w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db-reports","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("estimate: %d %s", resp.StatusCode, body)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)

	// an estimate for a DIFFERENT shape must not cover this create — and the
	// mismatch must NOT burn the one-shot estimate (coverage pre-check)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db-reports","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"standard","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "does not cover") {
		t.Fatalf("shape-mismatch create: %d %s", resp.StatusCode, body)
	}
	// the SAME estimate still works for the shape it actually priced
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db-reports","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createService: %d %s", resp.StatusCode, body)
	}
	var svc struct {
		Id, Status           string
		MonthlyEstimateCents int `json:"monthly_estimate_cents"`
		Intent               string
		ProvisioningSteps    []struct{ Step, Status string } `json:"provisioning_steps"`
	}
	_ = json.Unmarshal([]byte(body), &svc)
	if svc.Status != "provisioning" || svc.MonthlyEstimateCents != 2400 || svc.Intent != "database" {
		t.Fatalf("service shape: %+v", svc)
	}
	if len(svc.ProvisioningSteps) != 5 || svc.ProvisioningSteps[0].Step != "allocate" || svc.ProvisioningSteps[0].Status != "active" {
		t.Fatalf("C4 timeline: %+v", svc.ProvisioningSteps)
	}

	// a used estimate cannot be reused (one-shot)
	resp, _ = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db-two","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("estimate reuse: %d", resp.StatusCode)
	}

	// UNIQUE(env_id, name)
	resp, body = w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db-reports","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	var est2 struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est2)
	resp, _ = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db-reports","product":"postgres","estimate_id":"`+est2.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("dup name: %d", resp.StatusCode)
	}

	// --- status machine over real rows --------------------------------------
	dbSvc, orgID, err := w.prov.ServiceOrg(ctx, svc.Id)
	if err != nil {
		t.Fatal(err)
	}
	// illegal: provisioning → suspended
	if _, err := w.prov.Transition(ctx, dbSvc, "suspended", "system", "system", orgID); err == nil {
		t.Fatal("illegal transition accepted")
	}
	// legal: provisioning → ready; steps flip done; event lands
	readied, err := w.prov.Transition(ctx, dbSvc, "ready", "system", "system", orgID)
	if err != nil {
		t.Fatalf("provisioning→ready: %v", err)
	}
	if readied.Status != "ready" || !strings.Contains(string(readied.ProvisioningSteps), `"done"`) {
		t.Fatalf("ready row: %s %s", readied.Status, readied.ProvisioningSteps)
	}
	var n int
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='service.ready'", orgID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("ready event: %d %v", n, err)
	}
	// stale-row transition: the SQL FROM-guard catches concurrent movement
	if _, err := w.prov.Transition(ctx, dbSvc, "ready", "system", "system", orgID); err == nil {
		t.Fatal("stale transition accepted")
	}

	// --- update: shape change reprices; override requires reason (D22) ------
	resp, body = w.patch(t, "/v1/services/"+svc.Id, `{"shape":{"storage_gb":20}}`, ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, `"monthly_estimate_cents":2900`) {
		t.Fatalf("shape update reprice (19+20*0.5=29): %d %s", resp.StatusCode, body)
	}
	resp, body = w.patch(t, "/v1/services/"+svc.Id, `{"override":{"instances":5,"reason":""}}`, ownerCk)
	if resp.StatusCode != 422 || !strings.Contains(body, "override.reason") {
		t.Fatalf("override without reason: %d %s", resp.StatusCode, body)
	}
	resp, _ = w.patch(t, "/v1/services/"+svc.Id, `{"override":{"instances":5,"reason":"load test"}}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("override: %d", resp.StatusCode)
	}
	var overrideJSON string
	if err := w.pool.QueryRow(ctx, "select override->>'expires_at' from services where id=$1", svc.Id).Scan(&overrideJSON); err != nil || overrideJSON == "" {
		t.Fatalf("override auto-expiry not recorded: %q %v", overrideJSON, err)
	}

	// --- list + non-member 404 ----------------------------------------------
	resp, body = w.get(t, "/v1/envs/"+env.ID+"/services", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, "db-reports") {
		t.Fatalf("listServices: %d %s", resp.StatusCode, body)
	}
	strangerCk, _ := w.signupUser(t, "svc-stranger@example.com")
	resp, _ = w.get(t, "/v1/services/"+svc.Id, strangerCk)
	if resp.StatusCode != 404 {
		t.Fatalf("stranger getService: %d", resp.StatusCode)
	}

	// --- delete: 202 → deleting; re-delete 409 ------------------------------
	resp, _ = w.del(t, "/v1/services/"+svc.Id, ownerCk)
	if resp.StatusCode != 202 {
		t.Fatalf("deleteService: %d", resp.StatusCode)
	}
	var status string
	if err := w.pool.QueryRow(ctx, "select status from services where id=$1", svc.Id).Scan(&status); err != nil || status != "deleting" {
		t.Fatalf("status after delete: %s %v", status, err)
	}
	resp, _ = w.del(t, "/v1/services/"+svc.Id, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("re-delete: %d", resp.StatusCode)
	}
}
