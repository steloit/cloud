package identity_test

// T12.1: the policy authoring surface over HTTP — CRUD, versioning, dry-run
// impact preview, warn-first enforcement vocabulary, and the audit-event link.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPolicyAuthoringLifecycle(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ck, uid := w.signupUser(t, "gov@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "govco", uid)
	if err != nil {
		t.Fatal(err)
	}
	base := "/v1/orgs/" + org.ID + "/policies"
	body := `{"key":"allowed-regions","description":"EU only","rule":{"regions":["eu-west-1"]}}`

	// --- dry-run: impact preview, NOTHING persisted ------------------------
	resp, out := w.post(t, base+"?dry_run=true", body, ck)
	if resp.StatusCode != 200 {
		t.Fatalf("dry-run: %d %s", resp.StatusCode, out)
	}
	var impact struct {
		MembersAffected *int `json:"members_affected"`
	}
	_ = json.Unmarshal([]byte(out), &impact)
	if impact.MembersAffected == nil || *impact.MembersAffected != 1 {
		t.Fatalf("dry-run members_affected: %s", out)
	}
	var n int
	_ = w.pool.QueryRow(ctx, "select count(*) from policies where org_id=$1", org.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("dry-run persisted a policy (%d)", n)
	}

	// --- create: 201, version 1, warn-first default, audit linked ----------
	resp, out = w.post(t, base, body, ck)
	if resp.StatusCode != 201 {
		t.Fatalf("create: %d %s", resp.StatusCode, out)
	}
	var pol struct {
		Id              string `json:"id"`
		Version         int    `json:"version"`
		Enforcement     string `json:"enforcement"`
		LastChangeEvent string `json:"last_change_event"`
		ViolationCount  *int   `json:"violation_count_30d"`
	}
	_ = json.Unmarshal([]byte(out), &pol)
	if pol.Version != 1 || pol.Enforcement != "warn" {
		t.Fatalf("create shape: %s", out)
	}
	if !strings.HasPrefix(pol.LastChangeEvent, "evt_") {
		t.Fatalf("last_change_event not linked: %s", out)
	}
	if pol.ViolationCount == nil || *pol.ViolationCount != 0 {
		t.Fatalf("violation_count_30d should be 0: %s", out)
	}
	// the policy.created event is on the spine, subject = the policy id
	var evtAction string
	if err := w.pool.QueryRow(ctx, "select action from events where id=$1", pol.LastChangeEvent).Scan(&evtAction); err != nil {
		t.Fatalf("audit event not found: %v", err)
	}
	if evtAction != "policy.created" {
		t.Fatalf("audit action = %q", evtAction)
	}

	// --- duplicate same-key same-scope: 409 checklist, not a wall ----------
	resp, out = w.post(t, base, body, ck)
	if resp.StatusCode != 409 {
		t.Fatalf("duplicate: %d %s", resp.StatusCode, out)
	}
	if !strings.Contains(out, "remediation") {
		t.Fatalf("409 missing remediation: %s", out)
	}

	// --- get + list --------------------------------------------------------
	resp, out = w.get(t, "/v1/policies/"+pol.Id, ck)
	if resp.StatusCode != 200 {
		t.Fatalf("get: %d %s", resp.StatusCode, out)
	}
	resp, out = w.get(t, base, ck)
	if resp.StatusCode != 200 || !strings.Contains(out, pol.Id) {
		t.Fatalf("list: %d %s", resp.StatusCode, out)
	}

	// --- update: version bumps, PRIOR version retained, new audit link -----
	resp, out = w.patch(t, "/v1/policies/"+pol.Id, `{"enforcement":"enforce"}`, ck)
	if resp.StatusCode != 200 {
		t.Fatalf("update: %d %s", resp.StatusCode, out)
	}
	var upd struct {
		Version         int    `json:"version"`
		Enforcement     string `json:"enforcement"`
		LastChangeEvent string `json:"last_change_event"`
	}
	_ = json.Unmarshal([]byte(out), &upd)
	if upd.Version != 2 || upd.Enforcement != "enforce" {
		t.Fatalf("update shape: %s", out)
	}
	if upd.LastChangeEvent == pol.LastChangeEvent || !strings.HasPrefix(upd.LastChangeEvent, "evt_") {
		t.Fatalf("update did not relink the audit event: %s", out)
	}
	// both versions retained in history
	var versions int
	_ = w.pool.QueryRow(ctx, "select count(*) from policy_versions where policy_id=$1", pol.Id).Scan(&versions)
	if versions != 2 {
		t.Fatalf("policy_versions retained %d, want 2", versions)
	}

	// --- enforcement vocabulary is per-key (warn|enforce for rule policies) -
	resp, out = w.post(t, base, `{"key":"compute-ceiling","rule":{"max":4},"enforcement":"enabled"}`, ck)
	if resp.StatusCode != 422 {
		t.Fatalf("bad enforcement should be 422: %d %s", resp.StatusCode, out)
	}
	// ai-assistant DOES accept the enabled|opt_in|disabled triad
	resp, out = w.post(t, base, `{"key":"ai-assistant","rule":{},"enforcement":"disabled"}`, ck)
	if resp.StatusCode != 201 {
		t.Fatalf("ai-assistant disabled should create: %d %s", resp.StatusCode, out)
	}

	// --- cross-org isolation: a foreign policy id is a 404, never a 403 -----
	otherCk, _ := w.signupUser(t, "outsider@example.com")
	resp, _ = w.get(t, "/v1/policies/"+pol.Id, otherCk)
	if resp.StatusCode != 404 {
		t.Fatalf("foreign policy leaked (want 404): %d", resp.StatusCode)
	}
	resp, _ = w.patch(t, "/v1/policies/"+pol.Id, `{"enforcement":"warn"}`, otherCk)
	if resp.StatusCode != 404 {
		t.Fatalf("foreign policy patch leaked (want 404): %d", resp.StatusCode)
	}
}

// A member who lacks policy.manage (developer role) is denied 403 — a real
// permission denial, distinct from the non-member 404 above.
func TestPolicyManageRequired(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	ownerCk, ownerID := w.signupUser(t, "owner2@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "govco2", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	devCk, devID := w.signupUser(t, "dev2@example.com")
	if err := w.svc.AddMember(ctx, org.ID, devID, "developer", ownerID); err != nil {
		t.Fatal(err)
	}

	base := "/v1/orgs/" + org.ID + "/policies"
	body := `{"key":"allowed-regions","rule":{"regions":["eu-west-1"]}}`

	// owner may create
	resp, out := w.post(t, base, body, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("owner create: %d %s", resp.StatusCode, out)
	}
	// developer (member, no policy.manage) is denied 403
	resp, out = w.post(t, base, `{"key":"compute-ceiling","rule":{"max":4}}`, devCk)
	if resp.StatusCode != 403 {
		t.Fatalf("developer create should be 403: %d %s", resp.StatusCode, out)
	}
	if !strings.Contains(out, "policy.manage") {
		t.Fatalf("403 should name the missing permission: %s", out)
	}
}
