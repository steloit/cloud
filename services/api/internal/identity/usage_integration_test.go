package identity_test

// US-6.2: the per-org usage report over live HTTP — reconciles with raw
// meter events on read (recompute), billing.view gated, visible from alpha
// day one.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/metering"
)

func TestUsageReport(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "usage-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"usageco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	orgRow, _ := w.svc.GetOrg(ctx, org.Id)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_ = prj

	// no usage yet → empty meters, but a well-formed report
	month := metering.Period(time.Now())
	resp, body = w.get(t, "/v1/orgs/"+org.Id+"/billing/usage", ownerCk)
	if resp.StatusCode != 200 || !strings.Contains(body, `"month":"`+month+`"`) {
		t.Fatalf("empty report: %d %s", resp.StatusCode, body)
	}

	// create + ready a service so a span opens (this month)
	respE, bodyE := w.post(t, "/v1/estimates", `{"env":"`+env.ID+`","services":[{"product":"worker","name":"w","shape":{}}]}`, ownerCk)
	if respE.StatusCode != 200 {
		t.Fatalf("estimate: %d %s", respE.StatusCode, bodyE)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(bodyE), &est)
	respE, bodyE = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"w","product":"worker","estimate_id":"`+est.Id+`","shape":{}}`, ownerCk)
	if respE.StatusCode != 201 {
		t.Fatalf("create: %d %s", respE.StatusCode, bodyE)
	}
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(bodyE), &svc)
	row, orgID, _ := w.prov.ServiceOrg(ctx, svc.Id)
	if _, err := w.prov.Transition(ctx, row, "ready", "system", "system", orgID); err != nil {
		t.Fatal(err)
	}
	// backdate the open edge so the span has measurable seconds this month
	if _, err := w.pool.Exec(ctx,
		"update usage_events set at = now() - interval '1 hour' where org_id=$1 and edge='open'", org.Id); err != nil {
		t.Fatal(err)
	}

	// the report RECOMPUTES on read: the span now shows measurable seconds
	resp, body = w.get(t, "/v1/orgs/"+org.Id+"/billing/usage", ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("report: %d %s", resp.StatusCode, body)
	}
	var rep struct {
		Month  string
		Meters []struct {
			Meter string
			Used  float64
		}
	}
	_ = json.Unmarshal([]byte(body), &rep)
	found := false
	for _, m := range rep.Meters {
		if m.Meter == "service_span_seconds" && m.Used >= 3000 {
			found = true
		}
	}
	if !found {
		t.Fatalf("span meter not reconciled from raw events: %+v", rep.Meters)
	}
	// reconciles with the rollup table directly (report == stored derivation)
	var used int64
	if err := w.pool.QueryRow(ctx,
		"select used from quota_usage where org_id=$1 and meter='service_span_seconds' and period=$2",
		org.Id, month).Scan(&used); err != nil || used < 3000 {
		t.Fatalf("rollup row: %d %v", used, err)
	}

	// billing.view gate: a developer is denied naming the role
	_, devID := w.signupUser(t, "usage-dev@example.com")
	if err := w.svc.AddMember(ctx, org.Id, devID, "developer", ownerID); err != nil {
		t.Fatal(err)
	}
	devCk, _ := w.loginCookie(t, "usage-dev@example.com")
	resp, body = w.get(t, "/v1/orgs/"+org.Id+"/billing/usage", devCk)
	if resp.StatusCode != 403 || !strings.Contains(body, "role:developer") {
		t.Fatalf("developer usage: %d %s", resp.StatusCode, body)
	}
	// billing role CAN view
	_, billID := w.signupUser(t, "usage-bill@example.com")
	if err := w.svc.AddMember(ctx, org.Id, billID, "billing", ownerID); err != nil {
		t.Fatal(err)
	}
	billCk, _ := w.loginCookie(t, "usage-bill@example.com")
	resp, _ = w.get(t, "/v1/orgs/"+org.Id+"/billing/usage", billCk)
	if resp.StatusCode != 200 {
		t.Fatalf("billing usage: %d", resp.StatusCode)
	}
}
