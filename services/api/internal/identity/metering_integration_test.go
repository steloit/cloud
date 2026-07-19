package identity_test

// T3.7: metering flows from the FIRST resource (D10). Span edges on billing
// transitions, fully tagged, append-only at the DB level. Failed
// provisioning never bills (C4/US-3.6's half that exists pre-driver).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMeteringSpans(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "met-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"metco"}`, ownerCk)
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

	// create via the estimate gate
	resp, body = w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createService: %d %s", resp.StatusCode, body)
	}
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)

	countUsage := func() int {
		var n int
		if err := w.pool.QueryRow(ctx, "select count(*) from usage_events where org_id=$1", org.Id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// provisioning: NOTHING billed yet (metering starts at ready)
	if n := countUsage(); n != 0 {
		t.Fatalf("billed before ready: %d rows", n)
	}

	// → ready: span OPENS, fully tagged, rate snapshotted
	row, orgID, err := w.prov.ServiceOrg(ctx, svc.Id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.prov.Transition(ctx, row, "ready", "system", "system", orgID); err != nil {
		t.Fatal(err)
	}
	var meter, edge, gotPrj, gotEnv, product string
	var rate int64
	if err := w.pool.QueryRow(ctx,
		"select meter, edge, project_id, env_id, product, rate_cents from usage_events where service_id=$1", svc.Id).
		Scan(&meter, &edge, &gotPrj, &gotEnv, &product, &rate); err != nil {
		t.Fatalf("span open row: %v", err)
	}
	if meter != "service_span" || edge != "open" || gotPrj != prj.ID || gotEnv != env.ID || product != "postgres" || rate != 2400 {
		t.Fatalf("open edge: %s %s %s %s %s %d", meter, edge, gotPrj, gotEnv, product, rate)
	}

	// ready → degraded → ready: no edges (still billing)
	row, _, _ = w.prov.ServiceOrg(ctx, svc.Id)
	if _, err := w.prov.Transition(ctx, row, "degraded", "system", "system", orgID); err != nil {
		t.Fatal(err)
	}
	row, _, _ = w.prov.ServiceOrg(ctx, svc.Id)
	if _, err := w.prov.Transition(ctx, row, "ready", "system", "system", orgID); err != nil {
		t.Fatal(err)
	}
	if n := countUsage(); n != 1 {
		t.Fatalf("degraded flapping emitted edges: %d rows", n)
	}

	// delete: span CLOSES
	resp, _ = w.del(t, "/v1/services/"+svc.Id, ownerCk)
	if resp.StatusCode != 202 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	var closes int
	if err := w.pool.QueryRow(ctx, "select count(*) from usage_events where service_id=$1 and edge='close'", svc.Id).Scan(&closes); err != nil || closes != 1 {
		t.Fatalf("span close: %d %v", closes, err)
	}

	// failed provisioning NEVER bills: a second service fails before ready
	resp, body = w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"worker","name":"w","shape":{}}]}`, ownerCk)
	var est2 struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est2)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"w","product":"worker","estimate_id":"`+est2.Id+`","shape":{}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("create w: %d %s", resp.StatusCode, body)
	}
	var w2 struct{ Id string }
	_ = json.Unmarshal([]byte(body), &w2)
	row2, _, _ := w.prov.ServiceOrg(ctx, w2.Id)
	if _, err := w.prov.Transition(ctx, row2, "failed", "system", "system", orgID); err != nil {
		t.Fatal(err)
	}
	var w2rows int
	if err := w.pool.QueryRow(ctx, "select count(*) from usage_events where service_id=$1", w2.Id).Scan(&w2rows); err != nil || w2rows != 0 {
		t.Fatalf("failed provisioning billed: %d rows", w2rows)
	}

	// append-only at the DB level
	if _, err := w.pool.Exec(ctx, "delete from usage_events where org_id=$1", org.Id); err == nil ||
		!strings.Contains(err.Error(), "append-only") {
		t.Fatalf("usage_events DELETE did not raise: %v", err)
	}
}
