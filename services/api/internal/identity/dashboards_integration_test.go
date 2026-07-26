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

// TestDashboardForkAndPrebuilt — a prebuilt dashboard (generated view) is
// read-only: it cannot be edited or deleted, only FORKED into a personal
// editable copy (spec §2c / Design-Spec §244). Widgets copy with the fork.
func TestDashboardForkAndPrebuilt(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "dsh-fork-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"forkco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	// seed a PREBUILT dashboard (generated view — system row, prebuilt=true) with
	// a widget, org-visible so the member can read+fork it.
	if _, err := w.pool.Exec(ctx,
		`insert into dashboards (id,org_id,name,scope,visibility,owner_id,prebuilt) values ('dsh_pb',$1,'PostgreSQL Health','org','org',$2,true)`,
		org.Id, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx,
		`insert into dashboard_widgets (id,dashboard_id,source,query,viz) values ('wdg_pb','dsh_pb','metrics','pg_up','stat')`); err != nil {
		t.Fatal(err)
	}

	// a prebuilt cannot be edited or deleted — must fork.
	r, b := w.patch(t, "/v1/dashboards/dsh_pb", `{"name":"hacked"}`, ownerCk)
	if r.StatusCode != 409 {
		t.Fatalf("editing a prebuilt must 409 (fork instead), got %d %s", r.StatusCode, b)
	}
	r, _ = w.del(t, "/v1/dashboards/dsh_pb", ownerCk)
	if r.StatusCode != 409 {
		t.Fatalf("deleting a prebuilt must 409, got %d", r.StatusCode)
	}
	r, b = w.post(t, "/v1/dashboards/dsh_pb/widgets", `{"source":"logs","query":"x","viz":"list"}`, ownerCk)
	if r.StatusCode != 409 {
		t.Fatalf("adding a widget to a prebuilt must 409, got %d %s", r.StatusCode, b)
	}

	// FORK → a personal editable copy owned by the caller, widget copied.
	r, b = w.post(t, "/v1/dashboards/dsh_pb/fork", ``, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("fork: %d %s", r.StatusCode, b)
	}
	var fork struct {
		Id, Visibility string
		Prebuilt       bool
	}
	_ = json.Unmarshal([]byte(b), &fork)
	if fork.Visibility != "personal" || fork.Prebuilt {
		t.Fatalf("fork must be a personal, non-prebuilt copy: %s", b)
	}
	// the fork renders the copied widget and IS editable (rename works).
	r, b = w.get(t, "/v1/dashboards/"+fork.Id, ownerCk)
	if r.StatusCode != 200 || !strings.Contains(b, "pg_up") {
		t.Fatalf("fork should carry the copied widget: %d %s", r.StatusCode, b)
	}
	r, b = w.patch(t, "/v1/dashboards/"+fork.Id, `{"name":"my pg"}`, ownerCk)
	if r.StatusCode != 200 || !strings.Contains(b, "my pg") {
		t.Fatalf("fork must be editable: %d %s", r.StatusCode, b)
	}
	// the prebuilt source is untouched (still there, still prebuilt).
	var stillPrebuilt bool
	_ = w.pool.QueryRow(ctx, "select prebuilt from dashboards where id='dsh_pb'").Scan(&stillPrebuilt)
	if !stillPrebuilt {
		t.Fatal("fork mutated the prebuilt source")
	}
}

// TestDashboardDuplicateAndDeleteWidget — duplicate copies a custom dashboard
// into a personal one; deleteWidget removes a widget (editor-gated).
func TestDashboardDuplicateAndDeleteWidget(t *testing.T) {
	w := newWorld(t, time.Hour)
	ownerCk, _ := w.signupUser(t, "dsh-dup-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"dupco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	r, b := w.post(t, "/v1/orgs/"+org.Id+"/dashboards", `{"name":"ops","scope":"org","visibility":"org"}`, ownerCk)
	var dsh struct{ Id string }
	_ = json.Unmarshal([]byte(b), &dsh)
	r, b = w.post(t, "/v1/dashboards/"+dsh.Id+"/widgets", `{"source":"metrics","query":"rate(x)","viz":"line"}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("addWidget: %d %s", r.StatusCode, b)
	}
	var wdg struct{ Id string }
	_ = json.Unmarshal([]byte(b), &wdg)

	// duplicate → personal copy with the widget
	r, b = w.post(t, "/v1/dashboards/"+dsh.Id+"/duplicate", ``, ownerCk)
	if r.StatusCode != 201 || !strings.Contains(b, "rate(x)") {
		t.Fatalf("duplicate should copy widgets: %d %s", r.StatusCode, b)
	}
	var dup struct{ Id, Visibility string }
	_ = json.Unmarshal([]byte(b), &dup)
	if dup.Visibility != "personal" {
		t.Fatalf("duplicate must be personal: %s", b)
	}

	// deleteWidget on the original (editor-gated)
	r, _ = w.del(t, "/v1/dashboards/"+dsh.Id+"/widgets/"+wdg.Id, ownerCk)
	if r.StatusCode != 204 {
		t.Fatalf("deleteWidget: %d", r.StatusCode)
	}
	r, b = w.get(t, "/v1/dashboards/"+dsh.Id, ownerCk)
	if strings.Contains(b, "rate(x)") {
		t.Fatalf("widget should be gone from the original: %s", b)
	}
	// the duplicate keeps ITS copy (independent)
	r, b = w.get(t, "/v1/dashboards/"+dup.Id, ownerCk)
	if !strings.Contains(b, "rate(x)") {
		t.Fatalf("duplicate must keep its own widget after original's delete: %s", b)
	}
	// cross-dashboard widget delete is 404 (widget not on this dashboard)
	r, _ = w.del(t, "/v1/dashboards/"+dup.Id+"/widgets/"+wdg.Id, ownerCk)
	if r.StatusCode != 404 {
		t.Fatalf("deleting a widget not on this dashboard must 404, got %d", r.StatusCode)
	}
}

// TestDashboardForkAuthzFencing — fork/duplicate are WRITES: a read_only token
// is refused, a non-member/foreign source 404s BEFORE any copy, and a fork by a
// member (not the owner) is owned by that member (review + QA gaps).
func TestDashboardForkAuthzFencing(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "dsh-fa-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"faco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	memberCk, memberID := w.signupUser(t, "dsh-fa-member@example.com")
	if _, err := w.pool.Exec(ctx,
		"insert into members (id,org_id,user_id,role) values ('mbr_fa',$1,$2,'developer')", org.Id, memberID); err != nil {
		t.Fatal(err)
	}
	// an org-visible dashboard + a personal one, both owned by ownerA
	r, b := w.post(t, "/v1/orgs/"+org.Id+"/dashboards", `{"name":"shared","scope":"org","visibility":"org"}`, ownerCk)
	var shared struct{ Id string }
	_ = json.Unmarshal([]byte(b), &shared)
	r, b = w.post(t, "/v1/orgs/"+org.Id+"/dashboards", `{"name":"mine","scope":"org","visibility":"personal"}`, ownerCk)
	var personal struct{ Id string }
	_ = json.Unmarshal([]byte(b), &personal)

	// read_only token (owner's) cannot fork — it's a write.
	r, tb := w.post(t, "/v1/me/tokens", `{"name":"ro","scope":"read_only"}`, ownerCk)
	var tk struct{ Token string }
	_ = json.Unmarshal([]byte(tb), &tk)
	req, _ := http.NewRequest(http.MethodPost, w.srv.URL+"/v1/dashboards/"+shared.Id+"/fork", nil)
	req.Header.Set("Authorization", "Bearer "+tk.Token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatalf("read_only token fork must 403, got %d", res.StatusCode)
	}

	// a member CANNOT fork the owner's PERSONAL dashboard (invisible → 404, no copy)
	before := 0
	_ = w.pool.QueryRow(ctx, "select count(*) from dashboards where owner_id=$1", memberID).Scan(&before)
	r, _ = w.post(t, "/v1/dashboards/"+personal.Id+"/fork", ``, memberCk)
	if r.StatusCode != 404 {
		t.Fatalf("member fork of owner's personal dashboard must 404, got %d", r.StatusCode)
	}
	after := 0
	_ = w.pool.QueryRow(ctx, "select count(*) from dashboards where owner_id=$1", memberID).Scan(&after)
	if after != before {
		t.Fatal("a forbidden fork created a copy anyway")
	}

	// a member CAN fork the ORG-visible dashboard → the copy is owned by the MEMBER
	r, b = w.post(t, "/v1/dashboards/"+shared.Id+"/fork", ``, memberCk)
	if r.StatusCode != 201 {
		t.Fatalf("member fork of org dashboard: %d %s", r.StatusCode, b)
	}
	var fork struct{ Id string }
	_ = json.Unmarshal([]byte(b), &fork)
	var forkOwner string
	_ = w.pool.QueryRow(ctx, "select owner_id from dashboards where id=$1", fork.Id).Scan(&forkOwner)
	if forkOwner != memberID {
		t.Fatalf("fork must be owned by the forking member, got owner=%s (ownerA=%s)", forkOwner, ownerID)
	}
}

// TestForkPreservesProjectScope — a project-scoped source forks to a
// project-scoped personal copy; the born-filter travels (a member without that
// project access can't read the copy).
func TestForkPreservesProjectScope(t *testing.T) {
	w := newWorld(t, time.Hour)
	ownerCk, _ := w.signupUser(t, "dsh-ps-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"psco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	r, pb := w.post(t, "/v1/orgs/"+org.Id+"/projects", `{"name":"papp"}`, ownerCk)
	var proj struct{ Id string }
	_ = json.Unmarshal([]byte(pb), &proj)
	// a project-scoped, org-visible dashboard
	r, b := w.post(t, "/v1/orgs/"+org.Id+"/dashboards",
		`{"name":"proj view","scope":"project:`+proj.Id+`","visibility":"org"}`, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("create project-scoped: %d %s", r.StatusCode, b)
	}
	var src struct{ Id string }
	_ = json.Unmarshal([]byte(b), &src)
	// fork it → the copy keeps the project scope
	r, b = w.post(t, "/v1/dashboards/"+src.Id+"/fork", ``, ownerCk)
	if r.StatusCode != 201 {
		t.Fatalf("fork: %d %s", r.StatusCode, b)
	}
	var fork struct{ Id, Scope string }
	_ = json.Unmarshal([]byte(b), &fork)
	if fork.Scope != "project:"+proj.Id {
		t.Fatalf("fork must preserve the project scope (born-filter travels), got scope=%q", fork.Scope)
	}
}

// TestDeleteWidgetOnPrebuiltAndAudit — the 4th mutation (deleteWidget) is also
// blocked on a prebuilt; and a shared-dashboard widget delete is audited (F7).
func TestDeleteWidgetOnPrebuiltAndAudit(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "dsh-dwp-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"dwpco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	// prebuilt with a widget: deleteWidget must 409, widget survives.
	if _, err := w.pool.Exec(ctx,
		`insert into dashboards (id,org_id,name,scope,visibility,owner_id,prebuilt) values ('dsh_dwp',$1,'PG','org','org',$2,true)`, org.Id, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx,
		`insert into dashboard_widgets (id,dashboard_id,source,query,viz) values ('wdg_dwp','dsh_dwp','metrics','x','stat')`); err != nil {
		t.Fatal(err)
	}
	r, _ := w.del(t, "/v1/dashboards/dsh_dwp/widgets/wdg_dwp", ownerCk)
	if r.StatusCode != 409 {
		t.Fatalf("deleteWidget on a prebuilt must 409, got %d", r.StatusCode)
	}
	var stillThere int
	_ = w.pool.QueryRow(ctx, "select count(*) from dashboard_widgets where id='wdg_dwp'").Scan(&stillThere)
	if stillThere != 1 {
		t.Fatal("prebuilt widget was deleted despite the 409")
	}

	// a real shared dashboard: deleteWidget audits (F7).
	r, b := w.post(t, "/v1/orgs/"+org.Id+"/dashboards", `{"name":"ops","scope":"org","visibility":"org"}`, ownerCk)
	var dsh struct{ Id string }
	_ = json.Unmarshal([]byte(b), &dsh)
	r, b = w.post(t, "/v1/dashboards/"+dsh.Id+"/widgets", `{"source":"metrics","query":"y","viz":"line"}`, ownerCk)
	var wdg struct{ Id string }
	_ = json.Unmarshal([]byte(b), &wdg)
	r, _ = w.del(t, "/v1/dashboards/"+dsh.Id+"/widgets/"+wdg.Id, ownerCk)
	if r.StatusCode != 204 {
		t.Fatalf("deleteWidget: %d", r.StatusCode)
	}
	var audited int
	_ = w.pool.QueryRow(ctx, "select count(*) from events where subject=$1 and action='dashboard.updated'", dsh.Id).Scan(&audited)
	if audited < 1 {
		t.Fatalf("shared-dashboard widget delete must be audited (F7), found %d", audited)
	}
}
