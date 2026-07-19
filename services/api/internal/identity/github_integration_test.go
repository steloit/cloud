package identity_test

// T4.1: the GitHub ingress against real Postgres — signature verification,
// idempotent delivery storage, installation lifecycle, push/PR spine events
// on linked repos, reinstall survival.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gh "github.com/steloit/cloud/services/api/internal/github"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

const ghSecret = "whsec_test"

func ghPost(t *testing.T, srv *httptest.Server, event, delivery string, payload any, sign bool) *http.Response {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/integrations/github/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-GitHub-Delivery", delivery)
	if sign {
		mac := hmac.New(sha256.New, []byte(ghSecret))
		mac.Write(body)
		req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestGithubWebhookIngress(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	// mount the ingress beside the world's mux? build a dedicated server
	q := store.New(w.pool)
	mux := http.NewServeMux()
	gh.NewHandler(q, w.rec, ghSecret).Mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// unsigned / wrongly signed → 401; unconfigured handler → 503
	if r := ghPost(t, srv, "push", "d0", map[string]any{}, false); r.StatusCode != 401 {
		t.Fatalf("unsigned: %d", r.StatusCode)
	}
	mux2 := http.NewServeMux()
	gh.NewHandler(q, w.rec, "").Mount(mux2)
	srv2 := httptest.NewServer(mux2)
	t.Cleanup(srv2.Close)
	if r := ghPost(t, srv2, "push", "dx", map[string]any{}, true); r.StatusCode != 503 {
		t.Fatalf("unconfigured: %d", r.StatusCode)
	}

	// org + service + repo link + installation
	_, ownerID := w.signupUser(t, "gh-owner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "ghco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.ID)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_ = prj
	// a web service via the estimate gate at store level: reuse InsertService directly
	svcRow, err := q.InsertService(ctx, store.InsertServiceParams{
		ID: ids.New("svc"), EnvID: env.ID, Name: "api", Product: "web",
		Shape: []byte(`{}`), ProvisioningSteps: []byte(`[]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertInstallation(ctx, store.UpsertInstallationParams{
		ID: ids.New("ghi"), OrgID: org.ID, InstallationID: 4242, AccountLogin: "ghco",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateRepoLink(ctx, store.CreateRepoLinkParams{
		ID: ids.New("rln"), OrgID: org.ID, ServiceID: svcRow.ID, Repo: "ghco/shop", Branch: "main",
	}); err != nil {
		t.Fatal(err)
	}

	// --- push on the linked branch: stored + spine event --------------------
	push := map[string]any{
		"ref": "refs/heads/main", "after": "abc1234",
		"repository":   map[string]any{"full_name": "ghco/shop"},
		"installation": map[string]any{"id": 4242},
	}
	if r := ghPost(t, srv, "push", "d1", push, true); r.StatusCode != 202 {
		t.Fatalf("push: %d", r.StatusCode)
	}
	var n int
	if err := w.pool.QueryRow(ctx, "select count(*) from github_deliveries where delivery_id='d1'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("delivery stored: %d %v", n, err)
	}
	if err := w.pool.QueryRow(ctx,
		`select count(*) from events where org_id=$1 and action='git.push' and detail->>'sha'='abc1234' and subject=$2`,
		org.ID, svcRow.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("push spine event: %d %v", n, err)
	}
	// redelivery: idempotent, no second event
	if r := ghPost(t, srv, "push", "d1", push, true); r.StatusCode != 200 {
		t.Fatalf("redelivery: %d", r.StatusCode)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='git.push'", org.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("redelivery duplicated the event: %d", n)
	}
	// push on a non-linked branch: stored, no event
	push["ref"] = "refs/heads/feature"
	if r := ghPost(t, srv, "push", "d2", push, true); r.StatusCode != 202 {
		t.Fatalf("branch push: %d", r.StatusCode)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='git.push'", org.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("unlinked branch marked: %d", n)
	}

	// --- PR opened: spine event for preview orchestration -------------------
	pr := map[string]any{
		"action": "opened", "number": 142,
		"pull_request": map[string]any{"head": map[string]any{"sha": "def5678", "ref": "feat"}},
		"repository":   map[string]any{"full_name": "ghco/shop"},
		"installation": map[string]any{"id": 4242},
	}
	if r := ghPost(t, srv, "pull_request", "d3", pr, true); r.StatusCode != 202 {
		t.Fatalf("pr: %d", r.StatusCode)
	}
	if err := w.pool.QueryRow(ctx,
		`select count(*) from events where org_id=$1 and action='git.pr_opened' and detail->>'number'='142'`,
		org.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("pr spine event: %d %v", n, err)
	}

	// --- uninstall marks; the LINK survives; reinstall restores flow --------
	if r := ghPost(t, srv, "installation", "d4", map[string]any{
		"action":       "deleted",
		"installation": map[string]any{"id": 4242, "account": map[string]any{"login": "ghco"}},
	}, true); r.StatusCode != 202 {
		t.Fatalf("uninstall: %d", r.StatusCode)
	}
	push["ref"] = "refs/heads/main"
	if r := ghPost(t, srv, "push", "d5", push, true); r.StatusCode != 202 {
		t.Fatalf("post-uninstall push: %d", r.StatusCode)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='git.push'", org.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("uninstalled installation still marks: %d", n)
	}
	// reinstall (same installation id via upsert) — the repo link was never touched
	if _, err := q.UpsertInstallation(ctx, store.UpsertInstallationParams{
		ID: ids.New("ghi"), OrgID: org.ID, InstallationID: 4242, AccountLogin: "ghco",
	}); err != nil {
		t.Fatal(err)
	}
	if r := ghPost(t, srv, "push", "d6", push, true); r.StatusCode != 202 {
		t.Fatalf("post-reinstall push: %d", r.StatusCode)
	}
	if err := w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action='git.push'", org.ID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("reinstall flow: %d events (want 2)", n)
	}
}
