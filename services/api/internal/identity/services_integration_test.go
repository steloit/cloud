package identity_test

// T3.3: services over live HTTP — the estimate gate enforced AT THE API
// LAYER (US-3.2's law), the guarded status machine, D22 update semantics.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/identity/store"
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
	// A pin on postgres is REFUSED (US-3.8, founder-ratified 2026-07-27):
	// postgres has no priced instance count, so the replicas a pin provisions
	// cannot be metered — and the platform must never provision capacity it
	// cannot bill correctly. Re-enabled when per-replica pricing is ratified
	// (US-3.10); the priceable path (web/worker) is covered by
	// TestManualOverrideRespectsTheCapAndExpires.
	resp, body = w.patch(t, "/v1/services/"+svc.Id, `{"override":{"instances":5,"reason":"load test"}}`, ownerCk)
	if resp.StatusCode != 422 || !strings.Contains(body, "override.instances") {
		t.Fatalf("a postgres pin must be refused as unpriceable: %d %s", resp.StatusCode, body)
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

// US-3.7: the estimate gate binds to the CONTRACTED CONFIGURATION, not to a
// price. Prices collide — postgres `dev`+78 GB and `standard` both come to
// 5800¢ — so before this, a caller could price one configuration and provision
// the other, in either direction, and the gate agreed because the number did.
func TestEstimateGateRefusesAPriceCollidingShape(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "collide@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"collideco"}`, ownerCk)
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

	estimateFor := func(shape string) string {
		t.Helper()
		resp, body := w.post(t, "/v1/estimates",
			`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":`+shape+`}]}`, ownerCk)
		if resp.StatusCode != 200 {
			t.Fatalf("createEstimate %s: %d %s", shape, resp.StatusCode, body)
		}
		var est struct {
			Id                string `json:"id"`
			MonthlyTotalCents int    `json:"monthly_total_cents"`
		}
		_ = json.Unmarshal([]byte(body), &est)
		return est.Id
	}

	// Both configurations price identically — that is the whole point.
	const devBig = `{"size":"dev","storage_gb":78}`
	const standard = `{"size":"standard"}`
	devEst := estimateFor(devBig)

	// Price a dev+78GB, try to provision a standard at the same price.
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"swapped","product":"postgres","estimate_id":"`+devEst+`","shape":`+standard+`}`, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("a price-colliding SUBSTITUTION was accepted (%d) — the estimate is a contract, not a number: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "does not cover this shape") || !strings.Contains(body, "remediation") {
		t.Fatalf("the refusal must name the reason and carry remediation: %s", body)
	}
	// The refusal must NOT have burned the one-shot estimate.
	var accepted int
	if err := w.pool.QueryRow(ctx,
		`SELECT count(*) FROM estimates WHERE id = $1 AND accepted_at IS NOT NULL`, devEst).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	if accepted != 0 {
		t.Fatal("a refused create burned the estimate — a mistyped shape must leave it usable")
	}
	// And nothing was provisioned.
	svcs, err := w.prov.ListServices(ctx, env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 0 {
		t.Fatalf("the refused create provisioned %d service(s)", len(svcs))
	}

	// The other direction is equally refused.
	stdEst := estimateFor(standard)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"swapped2","product":"postgres","estimate_id":"`+stdEst+`","shape":`+devBig+`}`, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("the reverse substitution was accepted (%d): %s", resp.StatusCode, body)
	}

	// The three substitutions a review reproduced live: all were 201 before,
	// each persisting the substituted value into the row AND into the desired
	// doc handed to the cell. They are UNPRICED fields — which is the point:
	// "any shape substitution impossible regardless of price equality".
	for _, tc := range []struct{ name, priced, created string }{
		{"version", `{"size":"standard","version":"16"}`, `{"size":"standard","version":"17"}`},
		{"pgmq", `{"size":"standard","pgmq":{"dlq":true}}`, `{"size":"standard","pgmq":{"dlq":false}}`},
		{"connections", `{"size":"standard","connections":{"max":50}}`, `{"size":"standard","connections":{"max":5000}}`},
	} {
		t.Run("unpriced field: "+tc.name, func(t *testing.T) {
			id := estimateFor(tc.priced)
			resp, body := w.post(t, "/v1/envs/"+env.ID+"/services",
				`{"name":"sub-`+tc.name+`","product":"postgres","estimate_id":"`+id+`","shape":`+tc.created+`}`, ownerCk)
			if resp.StatusCode != 409 {
				t.Fatalf("%s was substituted at the same price (%d): %s", tc.name, resp.StatusCode, body)
			}
		})
	}

	// A wrong-typed field must be refused rather than silently defaulted — it
	// would otherwise be priced as one configuration and stored as another.
	illTyped := estimateFor(`{"size":"dev","storage_gb":0}`)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"typed","product":"postgres","estimate_id":"`+illTyped+`","shape":{"size":"dev","storage_gb":"78"}}`, ownerCk)
	if resp.StatusCode != 422 {
		t.Fatalf("an ill-typed storage_gb was accepted (%d) — it prices as 0 GB and stores as \"78\": %s", resp.StatusCode, body)
	}

	// The HONEST create still works — and the same configuration spelled with
	// its defaults written out must not be refused.
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"honest","product":"postgres","estimate_id":"`+stdEst+`","shape":{"size":"standard","ha":false,"storage_gb":0}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("the contracted configuration, spelled with explicit defaults, was refused: %d %s", resp.StatusCode, body)
	}
	// What is STORED is the resolved configuration, with defaults explicit —
	// so the cell is handed what was contracted, not a partial map.
	var stored map[string]any
	if err := w.pool.QueryRow(ctx,
		`SELECT shape FROM services WHERE name = 'honest'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"size", "storage_gb", "ha", "version", "pgmq", "connections"} {
		if _, ok := stored[k]; !ok {
			t.Fatalf("the stored shape omits declared field %q: %v", k, stored)
		}
	}
}

// US-3.7: the price the customer was SHOWN is the price they pay. If the
// pricing table moves under a live estimate, the create must refuse rather than
// provision at a number the customer never saw.
//
// Driven by rewriting the stored line — which is exactly what a pricing deploy
// looks like from the gate's side: the estimate's recorded price no longer
// matches what the same configuration costs now.
func TestEstimateGateRefusesAShapeRepricedSinceTheEstimate(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "reprice@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"repriceco"}`, ownerCk)
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

	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", resp.StatusCode, body)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)

	// The table moved: the stored line now records a price that the current
	// engine no longer produces for this configuration.
	if _, err := w.pool.Exec(ctx,
		`UPDATE estimates SET lines = jsonb_set(lines::jsonb, '{0,monthly_cents}', '9900')::jsonb WHERE id = $1`,
		est.Id); err != nil {
		t.Fatal(err)
	}

	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 409 {
		t.Fatalf("a repriced shape provisioned at a price the customer never saw (%d): %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "repriced") || !strings.Contains(body, "remediation") {
		t.Fatalf("the refusal must name repricing and carry remediation: %s", body)
	}
	svcs, err := w.prov.ListServices(ctx, env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 0 {
		t.Fatalf("the repriced create provisioned %d service(s)", len(svcs))
	}
}

// US-3.7: an out-of-catalog intent must be refused as a FIELD ERROR before the
// one-shot estimate is burned. It previously reached the INSERT, violated the
// services.intent CHECK constraint, and surfaced as a 500 saying "retry" — with
// the estimate already consumed, so every retry returned 409 forever.
func TestOutOfCatalogIntentIsRefusedWithoutBurningTheEstimate(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "intent@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"intentco"}`, ownerCk)
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

	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", resp.StatusCode, body)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)

	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","intent":"bogus","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 422 {
		t.Fatalf("an out-of-catalog intent returned %d, want 422: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "intent") {
		t.Fatalf("the refusal must name the offending field: %s", body)
	}

	// The estimate must survive — the whole point of refusing before the burn.
	var burned bool
	if err := w.pool.QueryRow(ctx,
		`SELECT accepted_at IS NOT NULL FROM estimates WHERE id = $1`, est.Id).Scan(&burned); err != nil {
		t.Fatal(err)
	}
	if burned {
		t.Fatal("a rejected intent burned the one-shot estimate — the customer can never provision what they priced")
	}
	// ...and the SAME estimate still works with a valid intent.
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","intent":"database","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("the estimate was unusable after a rejected intent: %d %s", resp.StatusCode, body)
	}
}

// A stored shape that cannot be read must fail CLOSED — and must not condemn its
// siblings. Both arms were unreachable by any test: the loop used to abort on
// the first unreadable shape, so the same estimate refused or succeeded
// depending purely on array order.
func TestUnreadableStoredShapeFailsClosedWithoutCondemningSiblings(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "legacy@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"legacyco"}`, ownerCk)
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

	newEstimate := func() string {
		t.Helper()
		resp, body := w.post(t, "/v1/estimates",
			`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"a","shape":{"size":"dev","storage_gb":10}},`+
				`{"product":"postgres","name":"b","shape":{"size":"standard"}}]}`, ownerCk)
		if resp.StatusCode != 200 {
			t.Fatalf("createEstimate: %d %s", resp.StatusCode, body)
		}
		var est struct{ Id string }
		_ = json.Unmarshal([]byte(body), &est)
		return est.Id
	}

	// A legacy row: shape[0] carries a loosely-typed value the OLD helpers
	// accepted at estimate time and resolve() now refuses.
	id := newEstimate()
	if _, err := w.pool.Exec(ctx, `
		UPDATE estimates SET services = jsonb_set(services::jsonb, '{0,Shape,storage_gb}', '"78"')::jsonb
		WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	// The VALID sibling at index 1 must still be creatable — the unreadable
	// shape sits before it, which used to abort the whole loop.
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"b","product":"postgres","estimate_id":"`+id+`","shape":{"size":"standard"}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("a valid sibling shape was condemned by an unreadable one at a lower index: %d %s", resp.StatusCode, body)
	}

	// A shape that matches NOTHING readable, on an estimate with an unreadable
	// entry, must say what is true — the estimate is unusable, not "not covered".
	id2 := newEstimate()
	if _, err := w.pool.Exec(ctx, `
		UPDATE estimates SET services = jsonb_set(services::jsonb, '{0,Shape,storage_gb}', '"78"')::jsonb
		WHERE id = $1`, id2); err != nil {
		t.Fatal(err)
	}
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"c","product":"postgres","estimate_id":"`+id2+`","shape":{"size":"performance"}}`, ownerCk)
	if resp.StatusCode != 409 || !strings.Contains(body, "can no longer be used") {
		t.Fatalf("want 409 naming an unusable estimate, got %d %s", resp.StatusCode, body)
	}
	var burned bool
	if err := w.pool.QueryRow(ctx,
		`SELECT accepted_at IS NOT NULL FROM estimates WHERE id = $1`, id2).Scan(&burned); err != nil {
		t.Fatal(err)
	}
	if burned {
		t.Fatal("the refusal burned the estimate")
	}
}

// The desired doc the CELL renders from must carry the resolved configuration —
// that is the whole point of persisting it. Asserting only the `shape` column
// left the doc unpinned.
func TestDesiredDocCarriesTheResolvedConfiguration(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "desired@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"desiredco"}`, ownerCk)
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
	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"d","shape":{"size":"dev","storage_gb":10}}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", resp.StatusCode, body)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"d","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev","storage_gb":10}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createService: %d %s", resp.StatusCode, body)
	}

	var desired, shape map[string]any
	if err := w.pool.QueryRow(ctx,
		`SELECT desired, shape FROM services WHERE name = 'd'`).Scan(&desired, &shape); err != nil {
		t.Fatal(err)
	}
	dshape, _ := desired["shape"].(map[string]any)
	if dshape == nil {
		t.Fatalf("the desired doc carries no shape: %v", desired)
	}
	for k := range shape {
		if _, ok := dshape[k]; !ok {
			t.Fatalf("the desired doc omits %q, which the stored shape carries — the cell would build from a different configuration than the row records", k)
		}
	}
	if fmt.Sprint(dshape) != fmt.Sprint(shape) {
		t.Fatalf("the desired doc and the stored shape disagree:\n desired: %v\n stored:  %v", dshape, shape)
	}
}

// US-3.8: a manual instance-pin provisions REAL capacity, so it must clear the
// same hard cap a scale-up does — and it must actually expire.
//
// Before this: `params.Override` was set outside the reprice branch, so the pin
// bypassed the cap entirely; and `expires_at` was written but never read by
// anything, so the "24h auto-expiry" D22 requires — and the API's own error
// message promises — did not exist. Nine instances, billed for one, forever.
func TestManualOverrideRespectsTheCapAndExpires(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "pin@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"pinco"}`, ownerCk)
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
	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"web","name":"api","shape":{"size":"standard-1"}}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", resp.StatusCode, body)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"api","product":"web","estimate_id":"`+est.Id+`","shape":{"size":"standard-1"}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createService: %d %s", resp.StatusCode, body)
	}
	var svc struct {
		Id                   string `json:"id"`
		MonthlyEstimateCents int    `json:"monthly_estimate_cents"`
	}
	_ = json.Unmarshal([]byte(body), &svc)

	// Drive it to ready FIRST: a pin's whole use case is a running service, and
	// the billing span only exists once one is open. Pinning a `provisioning`
	// service exercises no billing path at all.
	row, err := w.prov.Transition(ctx, mustGetSvc(t, w, svc.Id), "ready", "system", "system", org.Id)
	if err != nil {
		t.Fatalf("transition to ready: %v", err)
	}
	_ = row

	// A cap just above the current run-rate: a 9× pin must not fit under it.
	if _, err := w.pool.Exec(ctx,
		`INSERT INTO budgets (org_id, limit_cents) VALUES ($1,$2)
		 ON CONFLICT (org_id) DO UPDATE SET limit_cents = $2`,
		org.Id, int64(svc.MonthlyEstimateCents)+100); err != nil {
		t.Fatal(err)
	}

	pin := func(n int) (*http.Response, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPatch, w.srv.URL+"/v1/services/"+svc.Id,
			strings.NewReader(`{"override":{"instances":`+fmt.Sprint(n)+`,"reason":"load test"}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", ownerCk)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		return r, string(b)
	}

	// --- the cap must refuse the pin -----------------------------------------
	resp, body = pin(9)
	if resp.StatusCode == 200 {
		t.Fatalf("a 9-instance pin cleared a hard cap that does not cover it — the cap is bypassable by anyone who can PATCH a service: %s", body)
	}

	// --- lift the cap; the pin now applies and reaches the cell ---------------
	if _, err := w.pool.Exec(ctx, `UPDATE budgets SET limit_cents = 100000000 WHERE org_id = $1`, org.Id); err != nil {
		t.Fatal(err)
	}
	resp, body = pin(9)
	if resp.StatusCode != 200 {
		t.Fatalf("pin under an ample cap: %d %s", resp.StatusCode, body)
	}
	var desired map[string]any
	var pinnedCents int64
	if err := w.pool.QueryRow(ctx,
		`SELECT desired, monthly_estimate_cents FROM services WHERE id=$1`, svc.Id).Scan(&desired, &pinnedCents); err != nil {
		t.Fatal(err)
	}
	if o, _ := desired["override"].(map[string]any); o == nil {
		t.Fatal("a live pin did not reach the desired doc")
	}
	// Founder ruling: pinned capacity is METERED, so the row must carry the
	// pinned rate — nine instances provisioned and one billed is the defect.
	// The EXACT pinned rate, not merely "more than base": a pricing bug that
	// bills 1¢ extra for nine instances is otherwise indistinguishable.
	wantPinned, err := estimates.PriceWithInstances(estimates.ShapeInput{
		Product: "web", Shape: map[string]any{"size": "standard-1"},
	}, 9)
	if err != nil {
		t.Fatal(err)
	}
	if pinnedCents != wantPinned.MonthlyCents {
		t.Fatalf("the pin priced at %d, the engine says %d", pinnedCents, wantPinned.MonthlyCents)
	}
	// And the INVOICE follows. Billing reads usage_events.rate_cents, snapshotted
	// when a span opens — so a pin must CLOSE the base span and OPEN one at the
	// pinned rate. Asserting monthly_estimate_cents alone tests the one column
	// no billing arithmetic reads.
	pinEdges := spanRates(t, w, svc.Id)
	if len(pinEdges) < 3 {
		t.Fatalf("a pin must re-cut the span (open@base, close@base, open@pinned); got %v", pinEdges)
	}
	if last := pinEdges[len(pinEdges)-1]; last.edge != "open" || last.rate != wantPinned.MonthlyCents {
		t.Fatalf("after the pin the open span bills at %s@%d, want open@%d — nine provisioned, one billed",
			last.edge, last.rate, wantPinned.MonthlyCents)
	}

	// --- EXPIRY: the pin must stop being rendered, and the cell must be told --
	if _, err := w.pool.Exec(ctx, `
		UPDATE services SET override = jsonb_set(override::jsonb, '{expires_at}',
			to_jsonb(to_char(now() - interval '1 hour', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')))::jsonb
		WHERE id = $1`, svc.Id); err != nil {
		t.Fatal(err)
	}
	var genBefore int64
	if err := w.pool.QueryRow(ctx, `SELECT generation FROM services WHERE id=$1`, svc.Id).Scan(&genBefore); err != nil {
		t.Fatal(err)
	}

	// Drive the REAL runner, not the raw query: every part of RunOverrideExpiry
	// — the price restore, the desired rebuild, the startup sweep, the ticker,
	// and its wiring in main.go — was removable with the suite green when only
	// the SQL was exercised.
	sweepCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		w.prov.RunOverrideExpiry(sweepCtx, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()
	// Wait for the END STATE, not for the column: the clear commits before the
	// span is re-cut, so polling on `override IS NULL` and reading the spans
	// immediately is a race that would flake in CI.
	deadline := time.After(5 * time.Second)
	for {
		var stillPinned bool
		if err := w.pool.QueryRow(ctx, `SELECT override IS NOT NULL FROM services WHERE id=$1`, svc.Id).Scan(&stillPinned); err != nil {
			t.Fatal(err)
		}
		edges := spanRates(t, w, svc.Id)
		settled := !stillPinned && len(edges) > 0 &&
			edges[len(edges)-1].edge == "open" &&
			edges[len(edges)-1].rate == int64(svc.MonthlyEstimateCents)
		if settled {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("the STARTUP sweep did not converge: pinned=%v spans=%v — a ticker does not fire immediately, so a service nobody edits keeps rendering and billing its pin",
				stillPinned, edges)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunOverrideExpiry did not return when its context was cancelled")
	}

	var overrideAfter []byte
	var genAfter, centsAfter int64
	var desiredAfter map[string]any
	if err := w.pool.QueryRow(ctx,
		`SELECT override, generation, monthly_estimate_cents, desired FROM services WHERE id=$1`,
		svc.Id).Scan(&overrideAfter, &genAfter, &centsAfter, &desiredAfter); err != nil {
		t.Fatal(err)
	}
	if overrideAfter != nil {
		t.Fatalf("the expired pin survived the sweep: %s", overrideAfter)
	}
	if genAfter <= genBefore {
		t.Fatalf("generation did not advance (%d → %d) — the cell would never re-poll, so it keeps rendering the pinned count", genBefore, genAfter)
	}
	if o, _ := desiredAfter["override"]; o != nil {
		t.Fatalf("the desired doc still carries the expired pin: %v", desiredAfter)
	}
	if centsAfter != int64(svc.MonthlyEstimateCents) {
		t.Fatalf("the base price was not restored (%d, want %d) — the customer keeps paying for capacity that was taken away",
			centsAfter, svc.MonthlyEstimateCents)
	}

	// And the INVOICE follows: the pinned span must be closed and a new one
	// opened at the base rate. `monthly_estimate_cents` alone reaches no
	// billing arithmetic.
	edges := spanRates(t, w, svc.Id)
	if len(edges) < 3 {
		t.Fatalf("expected open@base, close@pinned, open@base — got %v", edges)
	}
	last := edges[len(edges)-1]
	if last.edge != "open" || last.rate != int64(svc.MonthlyEstimateCents) {
		t.Fatalf("the final span is %s@%d; billing must resume at the base rate %d", last.edge, last.rate, svc.MonthlyEstimateCents)
	}
}

func mustGetSvc(t *testing.T, w *world, id string) store.Service {
	t.Helper()
	svc, err := store.New(w.pool).GetService(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

type spanRate struct {
	edge string
	rate int64
}

func spanRates(t *testing.T, w *world, serviceID string) []spanRate {
	t.Helper()
	rows, err := w.pool.Query(context.Background(),
		`SELECT edge, rate_cents FROM usage_events WHERE service_id=$1 ORDER BY at`, serviceID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []spanRate
	for rows.Next() {
		var e spanRate
		if err := rows.Scan(&e.edge, &e.rate); err != nil {
			t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

// A pin on a product whose catalog shape has no instance count cannot be
// metered, and the founder ruling is that pinned capacity IS metered — so it
// must be refused rather than provisioned unbilled.
func TestUnpriceablePinIsRefused(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "unpriced@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"unpricedco"}`, ownerCk)
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
	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev"}}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", resp.StatusCode, body)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev"}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createService: %d %s", resp.StatusCode, body)
	}
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)

	req, _ := http.NewRequest(http.MethodPatch, w.srv.URL+"/v1/services/"+svc.Id,
		strings.NewReader(`{"override":{"instances":9,"reason":"load test"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", ownerCk)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 422 {
		t.Fatalf("an unpriceable pin returned %d, want 422 — it would provision replicas nothing can bill: %s", r.StatusCode, string(b))
	}
	if !strings.Contains(string(b), "override.instances") {
		t.Fatalf("the refusal must name the field: %s", string(b))
	}
}

// A PATCH that carries no `override` key clears the pin — `UpdateServiceShape`
// sets the column unconditionally, so that is the shipped semantic and the only
// way a customer un-pins. The DESIRED DOC must agree.
//
// Before this, `desired` kept `svc.Override` while the column went NULL: the
// capacity kept rendering, the sweep could never match it (`override IS NOT
// NULL`), and a shape-only PATCH additionally dropped the price back to base —
// so un-pinning released neither the capacity nor the money, and the original
// defect was reachable in two calls.
func TestAnEditWithoutAnOverrideClearsThePinEverywhere(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "unpin@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"unpinco"}`, ownerCk)
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
	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"web","name":"api","shape":{"size":"standard-1"}}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", resp.StatusCode, body)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"api","product":"web","estimate_id":"`+est.Id+`","shape":{"size":"standard-1"}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createService: %d %s", resp.StatusCode, body)
	}
	var svc struct {
		Id                   string `json:"id"`
		MonthlyEstimateCents int    `json:"monthly_estimate_cents"`
	}
	_ = json.Unmarshal([]byte(body), &svc)
	if _, err := w.prov.Transition(ctx, mustGetSvc(t, w, svc.Id), "ready", "system", "system", org.Id); err != nil {
		t.Fatal(err)
	}

	resp, body = w.patch(t, "/v1/services/"+svc.Id, `{"override":{"instances":9,"reason":"load test"}}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("pin: %d %s", resp.StatusCode, body)
	}

	// A SCALING-only edit — no shape, no override. This is the shape of un-pin
	// that used to release the capacity while keeping the pinned PRICE forever:
	// the price was only recomputed inside the shape branch, and the row was
	// then unsweepable, so nothing ever restored it.
	resp, body = w.patch(t, "/v1/services/"+svc.Id, `{"scaling":{"min":1,"max":3}}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("scaling edit: %d %s", resp.StatusCode, body)
	}

	var override []byte
	var cents int64
	var desired map[string]any
	if err := w.pool.QueryRow(ctx,
		`SELECT override, monthly_estimate_cents, desired FROM services WHERE id=$1`,
		svc.Id).Scan(&override, &cents, &desired); err != nil {
		t.Fatal(err)
	}
	if override != nil {
		t.Fatalf("the override column survived an edit that carries none: %s", override)
	}
	if o := desired["override"]; o != nil {
		t.Fatalf("the pin is gone from the column but STILL IN THE DESIRED DOC (%v) — the capacity keeps running, unsweepable, and the price just dropped to base", o)
	}
	if cents != int64(svc.MonthlyEstimateCents) {
		t.Fatalf("price is %d, want the base %d", cents, svc.MonthlyEstimateCents)
	}
	// And the invoice followed the release.
	edges := spanRates(t, w, svc.Id)
	if last := edges[len(edges)-1]; last.edge != "open" || last.rate != int64(svc.MonthlyEstimateCents) {
		t.Fatalf("billing did not return to the base rate: final span %s@%d, want open@%d", last.edge, last.rate, svc.MonthlyEstimateCents)
	}
}
