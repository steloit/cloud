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
	"reflect"
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
	// (US-11.9); the priceable path (web/worker) is covered by
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

// A malformed shape on PATCH must be a 422 naming the field, exactly as it is
// on POST /v1/estimates — one class of error, one answer.
//
// The Price call that used to validate the merged shape was removed when
// pricing was unified; Resolve became the first validator and its error path
// returned the raw error, so a client typo became a 500 with an event id and
// "contact support". Three shapes, three ways to be wrong.
func TestAMalformedShapeOnPatchIsAFieldErrorNotA500(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "badshape@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"badshapeco"}`, ownerCk)
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
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)

	for _, tc := range []struct{ name, shape, field string }{
		{"unknown key", `{"bogus":1}`, "shape.bogus"},
		{"wrong type", `{"size":123}`, "shape.size"},
		{"fractional int", `{"instances":1.5}`, "shape.instances"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := w.patch(t, "/v1/services/"+svc.Id, `{"shape":`+tc.shape+`}`, ownerCk)
			if resp.StatusCode != 422 {
				t.Fatalf("%s returned %d, want 422 naming the field — a typo must not read as a server fault: %s",
					tc.shape, resp.StatusCode, body)
			}
			if !strings.Contains(body, tc.field) {
				t.Fatalf("the refusal must name %s: %s", tc.field, body)
			}
		})
	}
}

// A concurrent edit must lose loudly, not silently overwrite.
//
// `UpdateService` reads the service, prices from that read, and writes back. With
// no generation fence, an edit that lands in between is overwritten — and since
// US-3.8 writes the PRICE on every PATCH, that put three facts in disagreement:
// the column holding one shape, the cell rendering another from the stale
// desired doc, and the invoice charging a third rate that `repriceSpan` cannot
// detect, because both sides of its comparison come from the same stale read.
func TestAConcurrentEditIsRefusedNotSilentlyOverwritten(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "race@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"raceco"}`, ownerCk)
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
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)

	// B reads the service (what the handler does before pricing).
	stale := mustGetSvc(t, w, svc.Id)

	// A commits a scale-up in between.
	resp, body = w.patch(t, "/v1/services/"+svc.Id, `{"shape":{"instances":3}}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("A's scale-up: %d %s", resp.StatusCode, body)
	}
	var afterA struct {
		MonthlyEstimateCents int `json:"monthly_estimate_cents"`
	}
	_ = json.Unmarshal([]byte(body), &afterA)

	// B now writes from its stale read. It must be refused.
	_, err = w.prov.UpdateService(ctx, stale, org.Id, ownerID, nil, []byte(`{"min":1,"max":3}`), nil)
	if err == nil {
		t.Fatal("a write from a stale read succeeded — it overwrites the other edit's shape, desired doc AND price, leaving the column, the cell and the invoice each holding a different answer")
	}

	// A's edit survives, intact and self-consistent.
	var shape, desired map[string]any
	var cents int64
	if err := w.pool.QueryRow(ctx,
		`SELECT shape, desired, monthly_estimate_cents FROM services WHERE id=$1`,
		svc.Id).Scan(&shape, &desired, &cents); err != nil {
		t.Fatal(err)
	}
	if got, _ := shape["instances"].(float64); int(got) != 3 {
		t.Fatalf("A's contracted configuration was overwritten: %v", shape)
	}
	dshape, _ := desired["shape"].(map[string]any)
	if got, _ := dshape["instances"].(float64); int(got) != 3 {
		t.Fatalf("the cell would render %v, not A's 3 instances", dshape["instances"])
	}
	if cents != int64(afterA.MonthlyEstimateCents) {
		t.Fatalf("the price is %d, A was charged %d", cents, afterA.MonthlyEstimateCents)
	}
}

// The sweep must clear every shape of dead pin, and ONE malformed row must not
// abort the batch — a raising cast would stop expiry platform-wide with a log
// line as the only symptom.
//
// This restores coverage that was LOST while fixing something else: the earlier
// test aged the pin with `::text` (a space separator), which happened to exercise
// the malformed arm; correcting it to real RFC3339 moved it onto the cast arm and
// silently took the malformed arm's only coverage with it.
func TestTheSweepClearsEveryDeadPinShapeAndSurvivesAMalformedOne(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "sweep@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"sweepco"}`, ownerCk)
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

	mk := func(name, override string) string {
		t.Helper()
		resp, body := w.post(t, "/v1/estimates",
			`{"env":"`+env.ID+`","services":[{"product":"web","name":"`+name+`","shape":{"size":"standard-1"}}]}`, ownerCk)
		if resp.StatusCode != 200 {
			t.Fatalf("estimate %s: %d %s", name, resp.StatusCode, body)
		}
		var est struct{ Id string }
		_ = json.Unmarshal([]byte(body), &est)
		resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
			`{"name":"`+name+`","product":"web","estimate_id":"`+est.Id+`","shape":{"size":"standard-1"}}`, ownerCk)
		if resp.StatusCode != 201 {
			t.Fatalf("create %s: %d %s", name, resp.StatusCode, body)
		}
		var svc struct{ Id string }
		_ = json.Unmarshal([]byte(body), &svc)
		// Plant the pin directly: these shapes are ones the API would never write,
		// which is exactly why the sweep has to cope with them.
		if _, err := w.pool.Exec(ctx,
			`UPDATE services SET override = $2::jsonb WHERE id = $1`, svc.Id, override); err != nil {
			t.Fatal(err)
		}
		return svc.Id
	}

	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	// Each pin's REASON differs, so the value-level assertion below cannot pass
	// by matching some other row's detail.
	garbagePin := `{"instances":9,"reason":"garbage","expires_at":"garbage"}`
	// Passes a prefix regex but is not a valid timestamp — this is the shape that
	// used to abort the entire statement.
	castInvalidPin := `{"instances":9,"reason":"cast","expires_at":"2026-13-45T99:99:99Z"}`
	noExpiryPin := `{"instances":9,"reason":"noexpiry"}`
	// FUTURE-dated, but space-separated: Postgres parses it, Go's RFC3339 does
	// NOT. So the API refuses to honour the pin while a cast-only predicate
	// would call it unexpired and never sweep it — the pin sits forever,
	// unhonoured and unclearable. The two liveness implementations (Go and SQL)
	// must agree, and the regex arm is what makes them.
	goRejectsPin := `{"instances":9,"reason":"spaceform","expires_at":"` +
		time.Now().Add(time.Hour).UTC().Format("2006-01-02 15:04:05-07") + `"}`
	validPastPin := `{"instances":9,"reason":"validpast","expires_at":"` + past + `"}`

	garbage := mk("a-garbage", garbagePin)
	castInvalid := mk("b-castbad", castInvalidPin)
	noExpiry := mk("c-noexpiry", noExpiryPin)
	goRejects := mk("f-spaceform", goRejectsPin)
	validPast := mk("d-valid", validPastPin)
	future := mk("e-future", `{"instances":9,"reason":"future","expires_at":"`+
		time.Now().Add(time.Hour).UTC().Format(time.RFC3339)+`"}`)

	sweepCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		w.prov.RunOverrideExpiry(sweepCtx, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()
	deadline := time.After(5 * time.Second)
	for {
		var pinned int
		if err := w.pool.QueryRow(ctx,
			`SELECT count(*) FROM services WHERE override IS NOT NULL AND env_id = $1`, env.ID).Scan(&pinned); err != nil {
			t.Fatal(err)
		}
		if pinned == 1 { // only the future pin should remain
			break
		}
		select {
		case <-deadline:
			t.Fatalf("%d pins still set — a dead pin the sweep cannot match renders forever, and one malformed row must not abort the batch", pinned)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()
	<-done

	for _, id := range []string{garbage, castInvalid, noExpiry, validPast, goRejects} {
		var o []byte
		if err := w.pool.QueryRow(ctx, `SELECT override FROM services WHERE id=$1`, id).Scan(&o); err != nil {
			t.Fatal(err)
		}
		if o != nil {
			t.Fatalf("a dead pin survived the sweep: %s", o)
		}
	}
	var stillFuture []byte
	if err := w.pool.QueryRow(ctx, `SELECT override FROM services WHERE id=$1`, future).Scan(&stillFuture); err != nil {
		t.Fatal(err)
	}
	if stillFuture == nil {
		t.Fatal("the sweep cleared a pin that has not expired")
	}

	// Every clear reaches the SPINE, not only a log line. Applying a pin records
	// `service.updated` carrying the operator's reason; without a matching record
	// on release, the activity feed shows capacity pinned and never given back,
	// and the only account of the release is a log the customer cannot see.
	// `via='system'` because the clock asked for it, not a person.
	//
	// The DETAIL is compared by VALUE, not by key presence. A first version of
	// this assertion used `detail ? 'override_expired'`, which
	// `{"override_expired":null}` satisfies — the released pin and the reason
	// given for it were never actually compared, so the audit record could carry
	// nothing and still pass. `kind` and `actor` are asserted for the same
	// reason: ListOrgEvents filters on both, so a wrong kind removes the release
	// from the very view the application appears in, which IS the failure this
	// test's message names.
	for _, c := range []struct{ id, override string }{
		{garbage, garbagePin}, {castInvalid, castInvalidPin},
		{noExpiry, noExpiryPin}, {validPast, validPastPin}, {goRejects, goRejectsPin},
	} {
		rows, err := w.pool.Query(ctx,
			`SELECT kind, actor, detail->'override_expired' FROM events
			  WHERE subject=$1 AND action='service.updated' AND via='system'
			    AND detail ? 'override_expired'`, c.id)
		if err != nil {
			t.Fatal(err)
		}
		var got []struct {
			kind, actor string
			detail      []byte
		}
		for rows.Next() {
			var r struct {
				kind, actor string
				detail      []byte
			}
			if err := rows.Scan(&r.kind, &r.actor, &r.detail); err != nil {
				t.Fatal(err)
			}
			got = append(got, r)
		}
		rows.Close()
		if len(got) != 1 {
			t.Fatalf("clearing %s recorded %d spine events, want exactly 1 — an expiry the customer cannot see in their activity feed is capacity that silently vanished", c.id, len(got))
		}
		if got[0].kind != "scale" || got[0].actor != "system" {
			t.Fatalf("expiry event for %s is kind=%q actor=%q, want scale/system — ListOrgEvents filters on both, so a wrong kind hides the release from the same feed the pin's application appears in",
				c.id, got[0].kind, got[0].actor)
		}
		var gotPin, wantPin map[string]any
		if err := json.Unmarshal(got[0].detail, &gotPin); err != nil {
			t.Fatalf("expiry detail for %s is not the pin: %s", c.id, got[0].detail)
		}
		_ = json.Unmarshal([]byte(c.override), &wantPin)
		if !reflect.DeepEqual(gotPin, wantPin) {
			t.Fatalf("expiry event for %s carries %v, want the released pin %v — the reason is the entire audit value of a pin, and it has to survive the release",
				c.id, gotPin, wantPin)
		}
	}
	// ...and the pin that is still live recorded nothing.
	var futureEvents int
	if err := w.pool.QueryRow(ctx,
		`SELECT count(*) FROM events WHERE subject=$1 AND detail ? 'override_expired'`,
		future).Scan(&futureEvents); err != nil {
		t.Fatal(err)
	}
	if futureEvents != 0 {
		t.Fatalf("a live pin recorded %d expiry events", futureEvents)
	}
}

// US-3.6's invariant, reached through US-3.8's new code path: metering opens at
// `ready`, never before. A PATCH now writes the price on every edit, so without
// the IsBilling guard a reprice on a still-provisioning service would emit an
// OPEN span — and the rollup accrues an open span to the cutoff, billing a
// service that never ran.
func TestARepriceBeforeReadyEmitsNoSpan(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "prebill@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"prebillco"}`, ownerCk)
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
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)

	// Still provisioning. A scale-up reprices the row...
	resp, body = w.patch(t, "/v1/services/"+svc.Id, `{"shape":{"instances":3}}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("scale-up while provisioning: %d %s", resp.StatusCode, body)
	}
	if got := spanRates(t, w, svc.Id); len(got) != 0 {
		t.Fatalf("a reprice before ready emitted %v — D10: metering starts at ready, never before, and an open span accrues to the cutoff", got)
	}

	// ...and reaching ready opens exactly one span, at the repriced rate.
	if _, err := w.prov.Transition(ctx, mustGetSvc(t, w, svc.Id), "ready", "system", "system", org.Id); err != nil {
		t.Fatal(err)
	}
	edges := spanRates(t, w, svc.Id)
	if len(edges) != 1 || edges[0].edge != "open" {
		t.Fatalf("reaching ready produced %v, want exactly one open", edges)
	}
	var cents int64
	if err := w.pool.QueryRow(ctx, `SELECT monthly_estimate_cents FROM services WHERE id=$1`, svc.Id).Scan(&cents); err != nil {
		t.Fatal(err)
	}
	if edges[0].rate != cents {
		t.Fatalf("the span opened at %d but the row says %d", edges[0].rate, cents)
	}
}

// The desired doc never carries a dead pin — asserted through UpdateService,
// because that is where the guard lives.
//
// The first version of this test lived in `provisioning` and applied
// `overrideInstances` to its own input before calling `desiredDoc`. That is the
// production guard, copied into the test body: deleting the real one from
// services.go left the test green, and a mutation sweep found it still alive.
// A test that re-implements the line it is named for asserts only that the test
// author can write that line twice.
//
// So: drive the real entry point, read the real column. Deleting the guard at
// services.go must fail four of these five rows.
func TestTheDesiredDocNeverCarriesADeadPin(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "deadpin@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"deadpinco"}`, ownerCk)
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
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)

	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	future := time.Now().Add(time.Hour).Format(time.RFC3339)

	// `web` because it is the one product the catalog can price per instance
	// (founder ruling 2026-07-27: postgres/valkey pins are refused as
	// unpriceable), so the live row exercises the pinned path rather than a 422.
	for _, tc := range []struct {
		name     string
		override string
		wantPin  bool
	}{
		{"expired", `{"instances":3,"reason":"x","expires_at":"` + past + `"}`, false},
		{"no expiry at all", `{"instances":3,"reason":"x"}`, false},
		{"unparseable expiry", `{"instances":3,"reason":"x","expires_at":"soon"}`, false},
		{"zero instances", `{"instances":0,"reason":"x","expires_at":"` + future + `"}`, false},
		{"live", `{"instances":3,"reason":"x","expires_at":"` + future + `"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, err := w.prov.UpdateService(ctx, mustGetSvc(t, w, svc.Id),
				org.Id, ownerID, nil, nil, []byte(tc.override))
			if err != nil {
				t.Fatalf("UpdateService: %v", err)
			}
			var d map[string]any
			if err := json.Unmarshal(row.Desired, &d); err != nil {
				t.Fatal(err)
			}
			if _, present := d["override"]; present != tc.wantPin {
				t.Fatalf("override in desired = %v, want %v — the cell renders whatever is here, expiry or not: %s",
					present, tc.wantPin, row.Desired)
			}
			// The column itself keeps the raw pin either way: the sweep matches
			// on `override IS NOT NULL`, so stripping it here as well would make
			// a dead pin invisible to the only thing that clears it. Compared as
			// parsed JSON, never as bytes — jsonb reorders keys and re-spaces.
			var gotPin, wantPin map[string]any
			if err := json.Unmarshal(row.Override, &gotPin); err != nil {
				t.Fatalf("override column is not JSON: %s", row.Override)
			}
			_ = json.Unmarshal([]byte(tc.override), &wantPin)
			if !reflect.DeepEqual(gotPin, wantPin) {
				t.Fatalf("override column = %v, want the raw pin %v — the sweep lists on this column",
					gotPin, wantPin)
			}
		})
	}
}

// The TICKER arm, not just the startup sweep.
//
// `RunOverrideExpiry` sweeps once at boot and then on every tick. Deleting the
// tick arm survived a mutation sweep, because every existing test plants its
// pins before the goroutine starts and is therefore satisfied by the boot
// sweep alone. The consequence of that survivor is the whole feature dark in
// production with the suite green: any pin created more than a moment after
// boot never expires, which is precisely the "temporary pin that is permanent"
// this task exists to fix.
//
// So the pin here is planted only AFTER the boot sweep has been observed to
// finish — a sentinel row it clears — leaving a tick as the only mechanism that
// can clear the second one.
func TestAPinCreatedAfterBootStillExpires(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "ticker@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"tickerco"}`, ownerCk)
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
	mk := func(name string) string {
		t.Helper()
		resp, body := w.post(t, "/v1/estimates",
			`{"env":"`+env.ID+`","services":[{"product":"web","name":"`+name+`","shape":{"size":"standard-1"}}]}`, ownerCk)
		if resp.StatusCode != 200 {
			t.Fatalf("estimate %s: %d %s", name, resp.StatusCode, body)
		}
		var est struct{ Id string }
		_ = json.Unmarshal([]byte(body), &est)
		resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
			`{"name":"`+name+`","product":"web","estimate_id":"`+est.Id+`","shape":{"size":"standard-1"}}`, ownerCk)
		if resp.StatusCode != 201 {
			t.Fatalf("create %s: %d %s", name, resp.StatusCode, body)
		}
		var svc struct{ Id string }
		_ = json.Unmarshal([]byte(body), &svc)
		return svc.Id
	}
	pin := func(id string) {
		t.Helper()
		if _, err := w.pool.Exec(ctx, `UPDATE services SET override = $2::jsonb WHERE id = $1`,
			id, `{"instances":3,"reason":"x","expires_at":"`+
				time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)+`"}`); err != nil {
			t.Fatal(err)
		}
	}
	waitCleared := func(id, why string) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			var o []byte
			if err := w.pool.QueryRow(ctx, `SELECT override FROM services WHERE id=$1`, id).Scan(&o); err != nil {
				t.Fatal(err)
			}
			if o == nil {
				return
			}
			select {
			case <-deadline:
				t.Fatalf("%s: pin still set after 5s — %s", id, why)
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	sentinel := mk("sentinel")
	pin(sentinel)

	sweepCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		w.prov.RunOverrideExpiry(sweepCtx, 50*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()
	defer func() { cancel(); <-done }()

	// The boot sweep clears the sentinel. The load-bearing fact is not that its
	// row loop has finished — it may still be working through other rows — but
	// that it has already LISTED. `late` is created after this observation, so
	// it cannot be in the boot sweep's row set, and only a tick can reach it.
	waitCleared(sentinel, "the startup sweep did not run at all")

	// Everything from here can only be reached by a tick.
	late := mk("late")
	pin(late)
	waitCleared(late, "only the startup sweep runs: every pin created after boot renders forever, and nothing reports it")
}

// A pin and its release are BOTH visible in the activity feed the customer
// actually reads, carrying the operator's reason.
//
// Two gaps this closes, which are one class with two representations. The pin's
// reason reaching the spine on APPLY was new code on this branch with no test
// anywhere — replacing its whole detail with `{}` left the suite green, while
// the comment above it calls the reason "the whole audit value of an affordance
// that provisions capacity outside the normal estimate path". And the RELEASE
// side was asserted only at the DB, with no org fence and never through the
// endpoint — so "the row exists" was pinned and "the customer can see it" was
// not. `ListEvents` filters on `kind`, so the release's kind is load-bearing
// for it appearing in the same filtered view as the application.
func TestAPinAndItsReleaseAreBothVisibleInTheActivityFeed(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "feed@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"feedco"}`, ownerCk)
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
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)

	// --- a pin below one instance is REFUSED at the edge ---------------------
	//
	// It used to return 200 and do nothing visible: overrideInstances declines
	// it, no pin reaches the desired doc, the price stays at base — but the raw
	// pin was still written to the column, where it is non-NULL, unhonoured, and
	// UNSWEEPABLE, because the expires_at the handler stamps is future and
	// well-formed so every arm of ListExpiredOverrides calls it live. A 200
	// followed by 24h of a pin nothing will honour and nothing will clear. The
	// same value in shape.instances has always been a 422.
	for _, n := range []string{"0", "-5"} {
		req, _ := http.NewRequest(http.MethodPatch, w.srv.URL+"/v1/services/"+svc.Id,
			strings.NewReader(`{"override":{"instances":`+n+`,"reason":"x"}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", ownerCk)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != 422 || !strings.Contains(string(b), "override.instances") {
			t.Fatalf("instances=%s: %d %s — want 422 naming override.instances", n, r.StatusCode, string(b))
		}
		var stored []byte
		if err := w.pool.QueryRow(ctx, `SELECT override FROM services WHERE id=$1`, svc.Id).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored != nil {
			t.Fatalf("a refused pin was still written to the column: %s — it is non-NULL, unhonoured, and its stamped expiry keeps every sweep arm calling it live", stored)
		}
	}

	// --- APPLY: the reason reaches the feed ---------------------------------
	req, _ := http.NewRequest(http.MethodPatch, w.srv.URL+"/v1/services/"+svc.Id,
		strings.NewReader(`{"override":{"instances":3,"reason":"black friday"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", ownerCk)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatalf("pin: %d %s", r.StatusCode, string(b))
	}

	// Decode and select the ONE event being asserted about. `strings.Contains` on
	// the page body is not enough here and the reason is specific: `ListEvents`
	// resolves env→org and then pages the whole ORG (there is no env predicate),
	// the spine is append-only so the apply event stays in the page forever, and
	// the release carries the same reason string. So a body-level `Contains` for
	// the reason was satisfied by the apply row — the release assertion below
	// could never fail for the reason its message gave, and a test that cannot
	// fail for its stated reason will be trusted for it. It also let the apply
	// event carry the wrong subject, actor, via and action, all four at once.
	type feedEvent struct {
		Kind    string         `json:"kind"`
		Via     string         `json:"via"`
		Actor   string         `json:"actor"`
		Action  string         `json:"action"`
		Subject string         `json:"subject"`
		Detail  map[string]any `json:"detail"`
	}
	feed := func(detailKey string) []feedEvent {
		t.Helper()
		resp, body := w.get(t, "/v1/envs/"+env.ID+"/events?kind=scale", ownerCk)
		if resp.StatusCode != 200 {
			t.Fatalf("listEvents: %d %s", resp.StatusCode, body)
		}
		// `data`, per EventList in openapi.yaml — NOT `items`. A wrong key here
		// decodes to an empty slice with no error, which is a silent zero-hit
		// feed: the assertions below would then be measuring the decode, not the
		// spine. The len()==1 checks are what turn that into a failure.
		var page struct {
			Data []feedEvent `json:"data"`
		}
		if err := json.Unmarshal([]byte(body), &page); err != nil {
			t.Fatalf("decode feed: %v — %s", err, body)
		}
		if len(page.Data) == 0 {
			t.Fatalf("the activity feed is empty: %s", body)
		}
		var hits []feedEvent
		for _, e := range page.Data {
			if e.Subject == svc.Id {
				if _, ok := e.Detail[detailKey]; ok {
					hits = append(hits, e)
				}
			}
		}
		return hits
	}

	applied := feed("override")
	if len(applied) != 1 {
		t.Fatalf("the pin recorded %d events on this service carrying an override — an operator pinning capacity outside the estimate path with no recorded reason is the exact affordance the audit trail exists for", len(applied))
	}
	if applied[0].Via != "user" || applied[0].Actor != ownerID || applied[0].Action != "service.updated" {
		t.Fatalf("the pin's audit row is via=%q actor=%q action=%q, want user/%s/service.updated — an out-of-band capacity grab attributed to the system, or to nobody, is not an audit trail",
			applied[0].Via, applied[0].Actor, applied[0].Action, ownerID)
	}
	gotApplied, _ := applied[0].Detail["override"].(map[string]any)
	if gotApplied["instances"] != float64(3) || gotApplied["reason"] != "black friday" {
		t.Fatalf("the pin's audit row carries %v, want instances 3 / reason \"black friday\"", applied[0].Detail["override"])
	}
	// The stamped expiry is part of the record: D22's promise to the customer is
	// that the pin is TEMPORARY, so the audit row has to say when it ends.
	exp, err := time.Parse(time.RFC3339, fmt.Sprint(gotApplied["expires_at"]))
	if err != nil {
		t.Fatalf("the audit row's expires_at is not RFC3339 (%v) — the sweep parses it with exactly this, so an unparseable one is a pin nothing will ever clear", gotApplied["expires_at"])
	}
	if d := time.Until(exp); d < 23*time.Hour || d > 25*time.Hour {
		t.Fatalf("the pin expires in %s, want ~24h (D22)", d)
	}

	// --- RELEASE: the expiry reaches the SAME feed ---------------------------
	if _, err := w.pool.Exec(ctx,
		`UPDATE services SET override = jsonb_set(override, '{expires_at}', to_jsonb($2::text)) WHERE id = $1`,
		svc.Id, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	sweepCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		w.prov.RunOverrideExpiry(sweepCtx, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()
	deadline := time.After(5 * time.Second)
	for {
		var o []byte
		if err := w.pool.QueryRow(ctx, `SELECT override FROM services WHERE id=$1`, svc.Id).Scan(&o); err != nil {
			t.Fatal(err)
		}
		if o == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the pin never expired")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()
	<-done

	released := feed("override_expired")
	if len(released) != 1 {
		t.Fatalf("the release recorded %d events in the kind=scale feed the application appears in — the customer sees capacity pinned and never given back", len(released))
	}
	if released[0].Via != "system" || released[0].Actor != "system" {
		t.Fatalf("the release is via=%q actor=%q, want system/system — the clock released it, not a person",
			released[0].Via, released[0].Actor)
	}
	// The reason is asserted on the RELEASE event specifically. Asserted against
	// the page it would have passed on the apply event's copy of the same string.
	gotReleased, _ := released[0].Detail["override_expired"].(map[string]any)
	if gotReleased["reason"] != "black friday" || gotReleased["instances"] != float64(3) {
		t.Fatalf("the release carries %v, want the pin it released — the reason is the whole audit value and it has to survive the release", released[0].Detail["override_expired"])
	}

	// --- and a stranger cannot tell this env from one that does not exist -----
	//
	// 404, not 403: `contexts/api-conventions.md` makes membership and key
	// denials indistinguishable from a missing id, and only role denials an
	// honest 403. A 403 here tells a stranger that `env_…` is real.
	strangerCk, _ := w.signupUser(t, "feed-stranger@example.com")
	resp, _ = w.get(t, "/v1/envs/"+env.ID+"/events?kind=scale", strangerCk)
	if resp.StatusCode != 404 {
		t.Fatalf("a non-member got %d for a foreign env's activity feed, want 404 — anything else distinguishes a real env id from a fabricated one", resp.StatusCode)
	}
}

// A DELETING service's dead pin is teardown's problem, not the sweep's.
//
// `ListExpiredOverrides` fences on `status <> 'deleting'`, and unlike the two
// fences on ClearExpiredOverride — which are unreachable because generation
// always fires first — this one is observable, so it gets a test rather than a
// comment. Without it a deleting service carrying a dead pin is listed on EVERY
// tick, forever: expireOverride does the price, namespace and org lookups, the
// clear matches zero rows, and that comes back as pgx.ErrNoRows, which
// expireOverride reads as "the row moved under us" and returns nil for — with
// no log line at all. Permanent busy-work with no symptom is exactly the shape
// this task exists to remove.
func TestTheSweepLeavesADeletingServiceAlone(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "sweepdel@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"sweepdelco"}`, ownerCk)
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
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)

	pin := `{"instances":3,"reason":"x","expires_at":"` +
		time.Now().Add(-time.Hour).UTC().Format(time.RFC3339) + `"}`
	if _, err := w.pool.Exec(ctx,
		`UPDATE services SET override = $2::jsonb, status = 'deleting' WHERE id = $1`, svc.Id, pin); err != nil {
		t.Fatal(err)
	}
	// POSITIVE CONTROL. Without it this test is negative-only: a
	// ListExpiredOverrides that returned nothing at all — or errored into an
	// empty slice — would satisfy "the deleting row is absent" perfectly.
	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"web","name":"live","shape":{"size":"standard-1"}}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("createEstimate live: %d %s", resp.StatusCode, body)
	}
	var est2 struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est2)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"live","product":"web","estimate_id":"`+est2.Id+`","shape":{"size":"standard-1"}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createService live: %d %s", resp.StatusCode, body)
	}
	var live struct{ Id string }
	_ = json.Unmarshal([]byte(body), &live)
	if _, err := w.pool.Exec(ctx,
		`UPDATE services SET override = $2::jsonb WHERE id = $1`, live.Id, pin); err != nil {
		t.Fatal(err)
	}

	// Assert on the QUERY, not on the row's end state. Both fences exist —
	// ClearExpiredOverride carries `status <> 'deleting'` too — so deleting the
	// LIST's fence changes nothing observable about the row: the clear matches
	// zero rows, expireOverride reads that as pgx.ErrNoRows ("moved under us")
	// and returns nil. The damage is the endless re-listing, so that is what has
	// to be measured. Calling the generated query directly also means this test
	// cannot pass by re-implementing the predicate it is checking.
	listed, err := store.New(w.pool).ListExpiredOverrides(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sawLive := false
	for _, row := range listed {
		if row.ID == live.Id {
			sawLive = true
		}
		if row.ID == svc.Id {
			t.Fatal("a deleting service with a dead pin is listed as sweepable — it will be listed on every tick forever, doing the price, namespace and org lookups each time and swallowing the zero-row clear as 'moved under us', with no log line and no end")
		}
	}
	if !sawLive {
		t.Fatal("the identical dead pin on a NON-deleting service was not listed either — this test would pass against a query that returns nothing at all, which is not evidence that the deleting fence did anything")
	}

	// The earlier version of this test also ran a boot sweep and asserted the row
	// was unchanged. That half was worthless twice over: `cancel()` fired
	// immediately after starting the goroutine, so the sweep usually died on a
	// cancelled context before querying anything; and even when it ran,
	// ClearExpiredOverride carries the same fence, so the row would be unchanged
	// with the LIST fence deleted too. Dropped — the query assertion above is the
	// real owner, and a second assertion that cannot fail only makes the first
	// one look better supported than it is.
}

// Exactly-at-expiry is DEAD in SQL, matching Go's half-open window.
//
// I had recorded `<=` vs `<` as an unkillable boundary on the grounds that
// `now()` moves on before any assertion can run. That is true of the SWEEP,
// which runs its SELECT in its own transaction — but not inside ONE
// transaction, where now() is fixed for the whole statement sequence. So the
// boundary is reachable after all, and it is worth reaching: this is the exact
// point at which the SQL predicate and `overrideInstances`' half-open window
// are claimed to agree, and TestOverrideLiveness already pins the Go side.
// (QA, 2026-07-27 — the disagreement was correct and the test is theirs.)
func TestAPinExpiringAtExactlyNowIsSweptNotStranded(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "boundary@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"boundaryco"}`, ownerCk)
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
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)

	// A second service, for the just-alive side of the boundary.
	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"web","name":"live","shape":{"size":"standard-1"}}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("createEstimate live: %d %s", resp.StatusCode, body)
	}
	var est2 struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est2)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"live","product":"web","estimate_id":"`+est2.Id+`","shape":{"size":"standard-1"}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createService live: %d %s", resp.StatusCode, body)
	}
	var live struct{ Id string }
	_ = json.Unmarshal([]byte(body), &live)

	tx, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Written FROM now() inside this transaction, in the RFC3339 form the regex
	// arm accepts — so expires_at is bit-for-bit the value the predicate
	// compares against, not "about now".
	if _, err := tx.Exec(ctx, `UPDATE services SET override = jsonb_build_object(
	        'instances', 9, 'reason', 'boundary',
	        'expires_at', to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'))
	      WHERE id = $1`, svc.Id); err != nil {
		t.Fatal(err)
	}
	// Same transaction ⇒ same now(). pgx.Tx satisfies store.DBTX, so this runs
	// the real query text rather than a copy of the predicate.
	rows, err := store.New(tx).ListExpiredOverrides(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.ID == svc.Id {
			found = true
		}
	}
	// The OTHER side of the window. Without this the boundary is pinned from one
	// direction only, and `<= now()` widened to `<= now() + interval '1 minute'`
	// survives — the sweep clearing pins the API is still honouring, capacity
	// vanishing early, which is the same disagreement in the opposite direction.
	// The other tests miss it because their live pin is an hour out.
	if _, err := tx.Exec(ctx, `UPDATE services SET override = jsonb_build_object(
	        'instances', 9, 'reason', 'just-alive',
	        'expires_at', to_char((now() + interval '1 microsecond') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'))
	      WHERE id = $1`, live.Id); err != nil {
		t.Fatal(err)
	}
	rows, err = store.New(tx).ListExpiredOverrides(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == live.Id {
			t.Fatal("a pin expiring one microsecond in the FUTURE was swept — the sweep is clearing capacity the API is still honouring, and the customer loses instances before the pin they were promised runs out")
		}
	}

	if !found {
		t.Fatal("a pin expiring at exactly now() was not swept — SQL would be treating the window as open at the instant Go treats it as closed, so the API refuses to honour the pin while the sweep declines to clear it: stranded, in the one place the two implementations are supposed to agree")
	}
}
