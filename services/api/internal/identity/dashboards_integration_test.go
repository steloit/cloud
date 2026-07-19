package identity_test

// T12.6 — dashboards (DB8): SCOPE (org | project:prj_…) ⊥ VISIBILITY
// (personal | org | restricted). The two axes are enforced independently:
// visibility decides who SEES the object; project scope is "born filtered" —
// create and read require access to that project.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDashboardsScopeVisibilityOrthogonal(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, _ := w.signupUser(t, "dsh-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"dshco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	// a second member (developer) in the same org
	memberCk, memberID := w.signupUser(t, "dsh-member@example.com")
	if _, err := w.pool.Exec(ctx,
		"insert into members (id,org_id,user_id,role) values ('mbr_dsh',$1,$2,'developer')", org.Id, memberID); err != nil {
		t.Fatal(err)
	}

	// --- VISIBILITY axis: personal is owner-only, org is all-members ---------
	r, b := w.post(t, "/v1/orgs/"+org.Id+"/dashboards",
		`{"name":"my private","scope":"org","visibility":"personal"}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("create personal: %d %s", r.StatusCode, b)
	}
	var personal struct{ Id string }
	_ = json.Unmarshal([]byte(b), &personal)

	r, b = w.post(t, "/v1/orgs/"+org.Id+"/dashboards",
		`{"name":"team fleet","scope":"org","visibility":"org"}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("create org-visible: %d %s", r.StatusCode, b)
	}
	var shared struct{ Id string }
	_ = json.Unmarshal([]byte(b), &shared)

	// the member sees the org dashboard but NOT the owner's personal one
	r, b = w.get(t, "/v1/orgs/"+org.Id+"/dashboards", memberCk)
	if r.StatusCode != 200 || !strings.Contains(b, shared.Id) {
		t.Fatalf("member should see the org dashboard: %d %s", r.StatusCode, b)
	}
	if strings.Contains(b, personal.Id) {
		t.Fatalf("member must NOT see the owner's personal dashboard: %s", b)
	}
	// direct GET of the personal dashboard by the member → 404 (invisible)
	r, _ = w.get(t, "/v1/dashboards/"+personal.Id, memberCk)
	if r.StatusCode != 404 {
		t.Fatalf("member GET of personal dashboard must 404, got %d", r.StatusCode)
	}

	// --- SCOPE axis: project-scoped is born filtered ------------------------
	// owner makes a project the member is NOT granted access to... but in this
	// org model every member sees every project (memberOrg), so to prove "born
	// filtered" we use a project in a DIFFERENT org the member isn't in.
	otherCk, _ := w.signupUser(t, "dsh-other@example.com")
	r, ob := w.post(t, "/v1/orgs", `{"name":"otherco"}`, otherCk)
	if r.StatusCode != 201 {
		t.Fatalf("other org: %d %s", r.StatusCode, ob)
	}
	var other struct{ Id string }
	_ = json.Unmarshal([]byte(ob), &other)
	r, pb := w.post(t, "/v1/orgs/"+other.Id+"/projects", `{"name":"secret"}`, otherCk)
	if r.StatusCode != 201 {
		t.Fatalf("other project: %d %s", r.StatusCode, pb)
	}
	var proj struct{ Id string }
	_ = json.Unmarshal([]byte(pb), &proj)

	// our org's owner cannot create a dashboard scoped to another org's project
	r, b = w.post(t, "/v1/orgs/"+org.Id+"/dashboards",
		`{"name":"cross","scope":"project:`+proj.Id+`","visibility":"org"}`, ownerCk)
	if r.StatusCode == 201 {
		t.Fatalf("cross-org project scope must be rejected, got 201: %s", b)
	}
	// the other org's owner CAN (born filtered to a project they access)
	r, b = w.post(t, "/v1/orgs/"+other.Id+"/dashboards",
		`{"name":"proj view","scope":"project:`+proj.Id+`","visibility":"org"}`, otherCk)
	if r.StatusCode != 201 {
		t.Fatalf("in-org project scope should succeed: %d %s", r.StatusCode, b)
	}
}

func TestDashboardCRUDAndWidgets(t *testing.T) {
	w := newWorld(t, time.Hour)
	ownerCk, _ := w.signupUser(t, "dsh-crud@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"crudco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	r, b := w.post(t, "/v1/orgs/"+org.Id+"/dashboards",
		`{"name":"ops","scope":"org","visibility":"org"}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("create: %d %s", r.StatusCode, b)
	}
	var dsh struct{ Id string }
	_ = json.Unmarshal([]byte(b), &dsh)

	// add a widget
	r, b = w.post(t, "/v1/dashboards/"+dsh.Id+"/widgets",
		`{"source":"metrics","query":"rate(http_requests)","viz":"line","pos":{"x":0,"y":0,"w":6,"h":4}}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("addWidget: %d %s", r.StatusCode, b)
	}
	// GET renders the widget
	r, b = w.get(t, "/v1/dashboards/"+dsh.Id, ownerCk)
	if r.StatusCode != 200 || !strings.Contains(b, "rate(http_requests)") {
		t.Fatalf("get with widget: %d %s", r.StatusCode, b)
	}
	// rename via PATCH
	r, b = w.patch(t, "/v1/dashboards/"+dsh.Id, `{"name":"ops v2"}`, ownerCk)
	if r.StatusCode != 200 || !strings.Contains(b, "ops v2") {
		t.Fatalf("patch rename: %d %s", r.StatusCode, b)
	}
	// F7/F6: un-sharing (org→personal) — the most governance-sensitive edit —
	// must still be audited (audit gates on pre- OR post-state shared).
	r, b = w.patch(t, "/v1/dashboards/"+dsh.Id, `{"visibility":"personal"}`, ownerCk)
	if r.StatusCode != 200 {
		t.Fatalf("un-share PATCH: %d %s", r.StatusCode, b)
	}
	var audited int
	_ = w.pool.QueryRow(context.Background(),
		"select count(*) from events where subject=$1 and action='dashboard.updated'", dsh.Id).Scan(&audited)
	if audited < 1 {
		t.Fatalf("un-share (org→personal) must be audited, found %d dashboard.updated events", audited)
	}
	// delete
	r, _ = w.del(t, "/v1/dashboards/"+dsh.Id, ownerCk)
	if r.StatusCode != 204 {
		t.Fatalf("delete: %d", r.StatusCode)
	}
	r, _ = w.get(t, "/v1/dashboards/"+dsh.Id, ownerCk)
	if r.StatusCode != 404 {
		t.Fatalf("get after delete must 404, got %d", r.StatusCode)
	}
}

func TestDashboardBillingRoleCannotShare(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, _ := w.signupUser(t, "dsh-b-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"billco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	billingCk, billingID := w.signupUser(t, "dsh-billing@example.com")
	if _, err := w.pool.Exec(ctx,
		"insert into members (id,org_id,user_id,role) values ('mbr_bill',$1,$2,'billing')", org.Id, billingID); err != nil {
		t.Fatal(err)
	}
	// billing can create a PERSONAL dashboard (dashboard.create = Y)
	r, b := w.post(t, "/v1/orgs/"+org.Id+"/dashboards",
		`{"name":"mine","scope":"org","visibility":"personal"}`, billingCk)
	if r.StatusCode != 201 {
		t.Fatalf("billing personal create should pass: %d %s", r.StatusCode, b)
	}
	// but NOT an org-shared one (dashboard.share_org = N for billing)
	r, b = w.post(t, "/v1/orgs/"+org.Id+"/dashboards",
		`{"name":"shared","scope":"org","visibility":"org"}`, billingCk)
	if r.StatusCode != 403 {
		t.Fatalf("billing share must be 403, got %d %s", r.StatusCode, b)
	}
}

// TestDashboardReadOnlyTokenCannotMutate — an owner's read_only personal token
// must be refused on every edit path, including the owner branch (review H1).
func TestDashboardReadOnlyTokenCannotMutate(t *testing.T) {
	w := newWorld(t, time.Hour)
	ownerCk, _ := w.signupUser(t, "dsh-tok-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"tokco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	r, b := w.post(t, "/v1/orgs/"+org.Id+"/dashboards",
		`{"name":"mine","scope":"org","visibility":"personal"}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("create: %d %s", r.StatusCode, b)
	}
	var dsh struct{ Id string }
	_ = json.Unmarshal([]byte(b), &dsh)
	// mint a read_only personal token for the SAME owner
	r, tb := w.post(t, "/v1/me/tokens", `{"name":"ro","scope":"read_only"}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("mint token: %d %s", r.StatusCode, tb)
	}
	var tk struct{ Token string }
	_ = json.Unmarshal([]byte(tb), &tk)
	// DELETE via the read_only token → 403, never a silent success
	req, _ := http.NewRequest(http.MethodDelete, w.srv.URL+"/v1/dashboards/"+dsh.Id, nil)
	req.Header.Set("Authorization", "Bearer "+tk.Token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatalf("read_only token DELETE must 403, got %d", res.StatusCode)
	}
}

// TestDashboardCrossOrgIDOR — a member of org A cannot GET or DELETE a
// dashboard in org B (bare dash id, no org in path).
func TestDashboardCrossOrgIDOR(t *testing.T) {
	w := newWorld(t, time.Hour)
	aCk, _ := w.signupUser(t, "dsh-a@example.com")
	ra, ba := w.post(t, "/v1/orgs", `{"name":"orgA"}`, aCk)
	if ra.StatusCode != 201 {
		t.Fatalf("orgA: %d %s", ra.StatusCode, ba)
	}
	var orgA struct{ Id string }
	_ = json.Unmarshal([]byte(ba), &orgA)
	r, db := w.post(t, "/v1/orgs/"+orgA.Id+"/dashboards",
		`{"name":"a-shared","scope":"org","visibility":"org"}`, aCk)
	if r.StatusCode != 201 {
		t.Fatalf("create A dashboard: %d %s", r.StatusCode, db)
	}
	var dsh struct{ Id string }
	_ = json.Unmarshal([]byte(db), &dsh)

	// a totally separate user/org
	bCk, _ := w.signupUser(t, "dsh-b@example.com")
	rb, bb := w.post(t, "/v1/orgs", `{"name":"orgB"}`, bCk)
	if rb.StatusCode != 201 {
		t.Fatalf("orgB: %d %s", rb.StatusCode, bb)
	}
	// B GETs A's dashboard by id → 404 (no cross-org read, no existence oracle)
	r, _ = w.get(t, "/v1/dashboards/"+dsh.Id, bCk)
	if r.StatusCode != 404 {
		t.Fatalf("cross-org GET must 404, got %d", r.StatusCode)
	}
	// B DELETEs A's dashboard → 404
	r, _ = w.del(t, "/v1/dashboards/"+dsh.Id, bCk)
	if r.StatusCode != 404 {
		t.Fatalf("cross-org DELETE must 404, got %d", r.StatusCode)
	}
}

// TestDashboardInvalidWidgetEnum — an out-of-set source/viz is a clean 422,
// never a raw DB CHECK 500 (review).
func TestDashboardInvalidWidgetEnum(t *testing.T) {
	w := newWorld(t, time.Hour)
	ownerCk, _ := w.signupUser(t, "dsh-enum@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"enumco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	r, b := w.post(t, "/v1/orgs/"+org.Id+"/dashboards",
		`{"name":"d","scope":"org","visibility":"org"}`, ownerCk)
	var dsh struct{ Id string }
	_ = json.Unmarshal([]byte(b), &dsh)
	r, b = w.post(t, "/v1/dashboards/"+dsh.Id+"/widgets",
		`{"source":"telepathy","query":"x","viz":"line"}`, ownerCk)
	if r.StatusCode != 422 {
		t.Fatalf("invalid widget source must be 422 (not 500), got %d %s", r.StatusCode, b)
	}
}

// TestDashboardRaiseToOrgNeedsShareGrant — transitioning a personal dashboard
// to org via PATCH requires dashboard.share_org (review: the raise gate must
// fire on the transition, and billing lacks the grant).
func TestDashboardRaiseToOrgNeedsShareGrant(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, _ := w.signupUser(t, "dsh-raise-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"raiseco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	billingCk, billingID := w.signupUser(t, "dsh-raise-billing@example.com")
	if _, err := w.pool.Exec(ctx,
		"insert into members (id,org_id,user_id,role) values ('mbr_raise',$1,$2,'billing')", org.Id, billingID); err != nil {
		t.Fatal(err)
	}
	// billing creates a personal dashboard (allowed)
	r, b := w.post(t, "/v1/orgs/"+org.Id+"/dashboards",
		`{"name":"mine","scope":"org","visibility":"personal"}`, billingCk)
	if r.StatusCode != 201 {
		t.Fatalf("billing personal create: %d %s", r.StatusCode, b)
	}
	var dsh struct{ Id string }
	_ = json.Unmarshal([]byte(b), &dsh)
	// billing PATCHes it to org → 403 (no share_org grant)
	r, b = w.patch(t, "/v1/dashboards/"+dsh.Id, `{"visibility":"org"}`, billingCk)
	if r.StatusCode != 403 {
		t.Fatalf("raise-to-org without share_org must 403, got %d %s", r.StatusCode, b)
	}
}
