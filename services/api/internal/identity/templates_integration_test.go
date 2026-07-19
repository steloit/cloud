package identity_test

// T12.4 / QA scenario 5: capture with secrets+bindings in the source →
// contents carry ZERO secret material (DB-level check proven live); excluded
// binding → required_input; estimate identical at save/list/consume; deleting
// (and editing) a used template leaves instantiations untouched.

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/store"
)

func TestTemplates(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "tpl-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"tplco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	orgRow, err := w.svc.GetOrg(ctx, org.Id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, "update orgs set plan='business' where id=$1", org.Id); err != nil {
		t.Fatal(err) // room for the instantiated project
	}
	orgRow, _ = w.svc.GetOrg(ctx, org.Id)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}

	// --- source env: api (web) + db (postgres) + cache EXCLUDED + a secret ----
	mk := func(name, product, shape string) string {
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
		var svc struct{ Id string }
		_ = json.Unmarshal([]byte(b), &svc)
		return svc.Id
	}
	apiID := mk("api", "web", `{"size":"standard-1","instances":1}`)
	dbID := mk("db", "postgres", `{"size":"dev","storage_gb":10}`)
	cacheID := mk("cache", "valkey", `{"memory_gb":1}`)
	// internal binding api->db (captured); excluded binding api->cache.
	apiSvc, err := store.New(w.pool).GetService(ctx, apiID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.prov.CreateBinding(ctx, apiSvc, dbID, "read_write", org.Id, env.ID, prj.ID, ownerID); err != nil {
		t.Fatalf("bind api->db: %v", err)
	}
	if _, _, err := w.prov.CreateBinding(ctx, apiSvc, cacheID, "read_only", org.Id, env.ID, prj.ID, ownerID); err != nil {
		t.Fatalf("bind api->cache: %v", err)
	}
	// a real secret in the source env (must never reach any template).
	if _, err := w.pool.Exec(ctx,
		`insert into secrets (id, org_id, project_id, env_id, name, version, ciphertext, nonce, wrapped_dek, dek_nonce, kek_id, created_by)
		 values ('sec_leakme', $1, $2, $3, 'DATABASE_URL', 1, 'supersecret'::bytea, 'n'::bytea, 'd'::bytea, 'dn'::bytea, 'test-v1', $4)`,
		org.Id, prj.ID, env.ID, ownerID); err != nil {
		t.Fatal(err)
	}

	// --- capture api+db (cache excluded) --------------------------------------
	r, tb := w.post(t, "/v1/orgs/"+org.Id+"/templates",
		`{"name":"shop-stack","source":{"project":"`+prj.ID+`","env":"`+env.ID+`"},"services":["`+apiID+`","`+dbID+`"]}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("capture: %d %s", r.StatusCode, tb)
	}
	var tpl struct {
		Id                   string         `json:"id"`
		MonthlyEstimateCents int            `json:"monthly_estimate_cents"`
		Contents             map[string]any `json:"contents"`
		RequiredInputs       []struct {
			Name string `json:"name"`
		} `json:"required_inputs"`
	}
	_ = json.Unmarshal([]byte(tb), &tpl)

	// QA #5: zero secret material — in the response AND at the DB level.
	if strings.Contains(tb, "sec_") || strings.Contains(tb, "supersecret") || strings.Contains(tb, "secret_ref") || strings.Contains(tb, "ciphertext") {
		t.Fatalf("template response carries secret material: %s", tb)
	}
	var dbContents string
	_ = w.pool.QueryRow(ctx, "select contents::text from templates where id=$1", tpl.Id).Scan(&dbContents)
	if strings.Contains(dbContents, "sec_") || strings.Contains(dbContents, "supersecret") {
		t.Fatalf("stored contents carry secret material: %s", dbContents)
	}
	// the DB-level guard is REAL: sneaking secret material in is rejected by the CHECK.
	if _, err := w.pool.Exec(ctx, `update templates set contents = '{"services":[{"name":"x","secret_ref":"sec_evil"}]}'::jsonb where id=$1`, tpl.Id); err == nil {
		t.Fatal("DB CHECK failed to block secret material in contents")
	}
	// excluded binding surfaced as a required input, never silently dropped.
	if len(tpl.RequiredInputs) != 1 {
		t.Fatalf("expected 1 required_input (api->cache excluded), got %+v", tpl.RequiredInputs)
	}
	if tpl.MonthlyEstimateCents <= 0 {
		t.Fatal("no birth estimate")
	}

	// --- estimate identical at save / list / consume --------------------------
	_, lb := w.get(t, "/v1/orgs/"+org.Id+"/templates", ownerCk)
	if !strings.Contains(lb, `"monthly_estimate_cents":`+strconv.Itoa(tpl.MonthlyEstimateCents)) {
		t.Fatalf("list estimate differs from birth: %s", lb)
	}
	r, eb := w.post(t, "/v1/estimates", `{"template_id":"`+tpl.Id+`"}`, ownerCk)
	if r.StatusCode != 200 {
		t.Fatalf("estimate-at-consume: %d %s", r.StatusCode, eb)
	}
	var est struct {
		MonthlyTotalCents int `json:"monthly_total_cents"`
	}
	_ = json.Unmarshal([]byte(eb), &est)
	if est.MonthlyTotalCents != tpl.MonthlyEstimateCents {
		t.Fatalf("consume estimate %d != birth %d", est.MonthlyTotalCents, tpl.MonthlyEstimateCents)
	}

	// --- consume: instantiate; missing required_inputs must 422 listing gaps --
	r, ib := w.post(t, "/v1/orgs/"+org.Id+"/projects", `{"name":"shop2","template_id":"`+tpl.Id+`"}`, ownerCk)
	if r.StatusCode != 422 || !strings.Contains(ib, "required_inputs.") {
		t.Fatalf("missing required_inputs should 422 with the gap named: %d %s", r.StatusCode, ib)
	}
	inputName := tpl.RequiredInputs[0].Name
	r, ib = w.post(t, "/v1/orgs/"+org.Id+"/projects",
		`{"name":"shop3","template_id":"`+tpl.Id+`","required_inputs":{"`+inputName+`":"redis://consumer-supplied:6379"}}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("instantiate: %d %s", r.StatusCode, ib)
	}
	var newPrj struct{ Id string }
	_ = json.Unmarshal([]byte(ib), &newPrj)
	var svcCount int
	_ = w.pool.QueryRow(ctx,
		`select count(*) from services s join environments e on e.id=s.env_id where e.project_id=$1`,
		newPrj.Id).Scan(&svcCount)
	if svcCount != 2 {
		t.Fatalf("instantiation created %d services, want 2 (api+db copies)", svcCount)
	}
	// the required input re-minted as a sealed secret in the NEW env (ADR-021),
	// named for the input — never a link back to the excluded service.
	var secretCount int
	_ = w.pool.QueryRow(ctx,
		`select count(*) from secrets s join environments e on e.id=s.env_id where e.project_id=$1 and s.name=$2`,
		newPrj.Id, inputName).Scan(&secretCount)
	if secretCount != 1 {
		t.Fatalf("required input did not land as a sealed secret in the new env: %d", secretCount)
	}
	_ = cacheID

	// --- frozen copy: edit + delete the template; instantiations untouched ----
	if r, _ := w.patch(t, "/v1/templates/"+tpl.Id, `{"name":"renamed"}`, ownerCk); r.StatusCode != 200 {
		t.Fatal("update template")
	}
	if r, _ := w.del(t, "/v1/templates/"+tpl.Id, ownerCk); r.StatusCode != 204 {
		t.Fatal("delete template")
	}
	var after int
	_ = w.pool.QueryRow(ctx,
		`select count(*) from services s join environments e on e.id=s.env_id where e.project_id=$1`,
		newPrj.Id).Scan(&after)
	if after != 2 {
		t.Fatalf("deleting the recipe touched the meals: %d services remain", after)
	}
}
