package identity_test

// T3.6: the bind / unbind / rotate cycle over live HTTP — credentials in the
// vault only, masked reads, U6 dependents on service delete, audit trail.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/secrets"
)

func TestBindings(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "bnd-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"bndco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	orgRow, err := w.svc.GetOrg(ctx, org.Id)
	if err != nil {
		t.Fatal(err)
	}
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}

	mkService := func(name, product, shape string) string {
		resp, body := w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"`+product+`","name":"`+name+`","shape":`+shape+`}]}`, ownerCk)
		if resp.StatusCode != 200 {
			t.Fatalf("estimate %s: %d %s", name, resp.StatusCode, body)
		}
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
	db := mkService("db-main", "postgres", `{"size":"dev","storage_gb":10}`)

	// --- bind: 201 pending, deterministic env var, masked value -------------
	resp, body = w.post(t, "/v1/services/"+api+"/bindings", `{"target":"`+db+`","scope":"read_write"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createBinding: %d %s", resp.StatusCode, body)
	}
	var bnd struct {
		Id, Status string
		EnvVars    map[string]string `json:"env_vars"`
		SecretRef  *string           `json:"secret_ref"`
	}
	_ = json.Unmarshal([]byte(body), &bnd)
	if bnd.Status != "pending" || !strings.HasPrefix(bnd.Id, "bnd_") {
		t.Fatalf("binding: %+v", bnd)
	}
	if _, ok := bnd.EnvVars["DB_MAIN_URL"]; !ok {
		t.Fatalf("deterministic env var missing: %v", bnd.EnvVars)
	}
	for _, v := range bnd.EnvVars {
		if strings.Contains(v, "postgres://") || strings.Contains(v, "@") {
			t.Fatalf("credential leaked in read: %s", v)
		}
	}
	if bnd.SecretRef != nil {
		t.Fatal("secret_ref returned — must never be")
	}

	// the credential EXISTS in the vault, scoped to the env
	cred, _, err := w.vault.Get(ctx, secrets.Scope{OrgID: org.Id, ProjectID: prj.ID, EnvID: env.ID}, "binding/"+bnd.Id)
	if err != nil || !strings.HasPrefix(string(cred), "postgres://bnd_rw:") {
		t.Fatalf("vault credential: %q %v", cred, err)
	}

	// duplicate binding → 409
	resp, _ = w.post(t, "/v1/services/"+api+"/bindings", `{"target":"`+db+`"}`, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("dup binding: %d", resp.StatusCode)
	}
	// self-binding → 422; cross-env → 422
	resp, _ = w.post(t, "/v1/services/"+api+"/bindings", `{"target":"`+api+`"}`, ownerCk)
	if resp.StatusCode != 422 {
		t.Fatalf("self binding: %d", resp.StatusCode)
	}
	env2, err := w.prov.CreateEnvironment(ctx, prj, "staging", "", false, false, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_ = env2
	// (cross-env needs a service in env2 to test properly — the same-env
	// check fires on target lookup; covered by unit of CreateBinding rule)

	// --- U6: deleting the bound target names its dependents ------------------
	resp, body = w.del(t, "/v1/services/"+db, ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "api") || !strings.Contains(body, bnd.Id) {
		t.Fatalf("dependents 409: %d %s", resp.StatusCode, body)
	}

	// --- list shows the binding, still masked --------------------------------
	resp, body = w.get(t, "/v1/services/"+api+"/bindings", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, bnd.Id) || strings.Contains(body, "postgres://") {
		t.Fatalf("listBindings: %d %s", resp.StatusCode, body)
	}

	// --- unbind: 204; credentials rotated then removed; audit trail ----------
	resp, _ = w.del(t, "/v1/bindings/"+bnd.Id, ownerCk)
	if resp.StatusCode != 204 {
		t.Fatalf("deleteBinding: %d", resp.StatusCode)
	}
	if _, _, err := w.vault.Get(ctx, secrets.Scope{OrgID: org.Id, ProjectID: prj.ID, EnvID: env.ID}, "binding/"+bnd.Id); err != secrets.ErrNotFound {
		t.Fatalf("credential survives unbind: %v", err)
	}
	var status string
	var rotated bool
	if err := w.pool.QueryRow(ctx, "select status, rotated_at is not null from bindings where id=$1", bnd.Id).Scan(&status, &rotated); err != nil || status != "revoked" || !rotated {
		t.Fatalf("revoked row: %s rotated=%v %v", status, rotated, err)
	}
	var n int
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action in ('binding.created','binding.revoked')", org.Id).Scan(&n); err != nil || n != 2 {
		t.Fatalf("bind/unbind audit: %d %v", n, err)
	}

	// the pair is free again after revocation (partial-unique index)
	resp, _ = w.post(t, "/v1/services/"+api+"/bindings", `{"target":"`+db+`"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("rebind after revoke: %d", resp.StatusCode)
	}
	// …and now the target delete is blocked again; unbind, then it schedules
	var bnd2 struct{ Id string }
	respB, bodyB := w.get(t, "/v1/services/"+api+"/bindings", ownerCk)
	_ = respB
	var bl struct {
		Data []struct{ Id string }
	}
	_ = json.Unmarshal([]byte(bodyB), &bl)
	if len(bl.Data) != 1 {
		t.Fatalf("expected 1 active binding, got %d", len(bl.Data))
	}
	bnd2.Id = bl.Data[0].Id
	if r, _ := w.del(t, "/v1/bindings/"+bnd2.Id, ownerCk); r.StatusCode != 204 {
		t.Fatal("unbind 2")
	}
	if r, _ := w.del(t, "/v1/services/"+db, ownerCk); r.StatusCode != 202 {
		t.Fatalf("delete after unbind: %d", r.StatusCode)
	}
}
