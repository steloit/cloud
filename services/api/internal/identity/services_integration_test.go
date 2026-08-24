package identity_test

// T3.3: services over live HTTP — the estimate gate enforced AT THE API
// LAYER (US-3.2's law), the guarded status machine, D22 update semantics.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/money"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
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
	if pinnedCents != wantPinned.MonthlyCents.Int64() {
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
	if last := pinEdges[len(pinEdges)-1]; last.edge != "open" || last.rate != wantPinned.MonthlyCents.Int64() {
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
// `ListExpiredOverrides` fences on `status <> 'deleting'`, and it is observable,
// so it gets a test rather than a comment. Without it a deleting service
// carrying a dead pin is listed on EVERY
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

// The pin's numeric bounds, at the edge and in the pricing engine.
//
// Three separate jobs, and the test asserts the LAYERING as much as the values:
// the handler refuses a nonsensical count, the engine refuses a count it cannot
// price exactly, and the hard spend cap refuses a count that prices fine but
// costs too much. Collapsing any of those into another is how the overflow
// below went unnoticed.
func TestAPinsInstanceCountIsBoundedAtEveryLayer(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "bounds@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"boundsco"}`, ownerCk)
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

	pin := func(payload string) (*http.Response, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPatch, w.srv.URL+"/v1/services/"+svc.Id, strings.NewReader(payload))
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
		r, b := pin(`{"override":{"instances":` + n + `,"reason":"x"}}`)
		if r.StatusCode != 422 || !strings.Contains(b, "override.instances") {
			t.Fatalf("instances=%s: %d %s — want 422 naming override.instances", n, r.StatusCode, b)
		}
		var stored []byte
		if err := w.pool.QueryRow(ctx, `SELECT override FROM services WHERE id=$1`, svc.Id).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored != nil {
			t.Fatalf("a refused pin was still written to the column: %s — it is non-NULL, unhonoured, and its stamped expiry keeps every sweep arm calling it live", stored)
		}
	}

	// --- a pin too large to PRICE is refused, and never wraps ----------------
	//
	// This is the one that mattered most. Before the engine's upper bound,
	// override.instances = MaxInt64 returned HTTP 200 and wrote
	// monthly_estimate_cents = -700: the multiply wrapped, so the hard spend cap
	// was BYPASSED (UpdateService only enforces on `delta > 0`, and a wrapped
	// price is a decrease), ADR-025's integer-cents rule was violated, and
	// repriceSpan would open a billing span at a negative rate. The whole task's
	// premise is "never provision what cannot be billed correctly", and this was
	// the input that broke it on a single authenticated PATCH.
	for _, n := range []string{"9223372036854775807"} {
		r, b := pin(`{"override":{"instances":` + n + `,"reason":"probe"}}`)
		if r.StatusCode != 422 || !strings.Contains(b, "override.instances") {
			t.Fatalf("instances=%s: %d %s — want 422 naming override.instances", n, r.StatusCode, b)
		}
		var cents int64
		var stored []byte
		if err := w.pool.QueryRow(ctx,
			`SELECT monthly_estimate_cents, override FROM services WHERE id=$1`, svc.Id).Scan(&cents, &stored); err != nil {
			t.Fatal(err)
		}
		if cents < 0 {
			t.Fatalf("instances=%s produced monthly_estimate_cents=%d — money is integer cents (ADR-025) and a wrapped price also skips the hard cap, because UpdateService only enforces on an increase", n, cents)
		}
		if stored != nil {
			t.Fatalf("a refused pin reached the column: %s", stored)
		}
	}

	// A merely EXPENSIVE pin is a different job. 1000 instances prices exactly
	// and is nowhere near the engine's representability ceiling, so the engine
	// has no business refusing it — picking a maximum instance count is a
	// commercial decision and the engine does not make those. The HARD SPEND CAP
	// is what refuses it, and this asserts the layering rather than assuming it.
	// Before the overflow fix this path was unreachable for large pins, because a
	// wrapped price is a DECREASE and UpdateService only enforces on an increase.
	// (An earlier version used 2^40 here. Once the ceiling was re-derived from
	// what the billing rollup can carry it dropped to ~3.4e12 cents, and 2^40
	// instances became the ENGINE's business — so the example stopped testing the
	// layer it named. A test whose example drifts out from under its claim is the
	// same failure as a comment that does.)
	var base int64
	if err := w.pool.QueryRow(ctx, `SELECT monthly_estimate_cents FROM services WHERE id=$1`, svc.Id).Scan(&base); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx,
		`INSERT INTO budgets (org_id, limit_cents) VALUES ($1,$2)
		 ON CONFLICT (org_id) DO UPDATE SET limit_cents = $2`, org.Id, base+100); err != nil {
		t.Fatal(err)
	}
	// t.Cleanup, not a trailing DELETE: a t.Fatalf between the insert and the
	// delete would otherwise leave a live cap for whatever ran next.
	t.Cleanup(func() {
		_, _ = w.pool.Exec(context.Background(), `DELETE FROM budgets WHERE org_id=$1`, org.Id)
	})
	rBig, bBig := pin(`{"override":{"instances":1000,"reason":"expensive"}}`)
	// 402 specifically, from the CAP — not merely "not 200". Asserting only
	// non-200 does not say which layer refused, which is the one thing this
	// section exists to show: adding a commercial ceiling to the pricing engine
	// (the decision that file explicitly refuses to make) would satisfy a
	// non-200 assertion and silently move the boundary.
	if rBig.StatusCode != 402 || !strings.Contains(bBig, "quota") {
		t.Fatalf("a 1000-instance pin should be refused by the hard spend cap (402 quota_exceeded), got %d %s — if the pricing engine refused it instead, the engine has acquired a commercial ceiling it is not allowed to have", rBig.StatusCode, bBig)
	}
	var capEvents int
	if err := w.pool.QueryRow(ctx,
		`SELECT count(*) FROM events WHERE org_id=$1 AND action='billing.spend_cap_reached'`, org.Id).Scan(&capEvents); err != nil {
		t.Fatal(err)
	}
	if capEvents == 0 {
		t.Fatal("the cap refused the pin but recorded nothing on the spine — 'the cap is real' has to be auditable, not just a response code")
	}
	var afterCents int64
	if err := w.pool.QueryRow(ctx, `SELECT monthly_estimate_cents FROM services WHERE id=$1`, svc.Id).Scan(&afterCents); err != nil {
		t.Fatal(err)
	}
	if afterCents != base {
		t.Fatalf("a capped-out pin still moved the price: %d → %d", base, afterCents)
	}
	if _, err := w.pool.Exec(ctx, `DELETE FROM budgets WHERE org_id=$1`, org.Id); err != nil {
		t.Fatal(err)
	}

	// --- one instance is the LEGAL FLOOR, and it is honoured ------------------
	//
	// Without this the guard is pinned from below only: `< 1` can be tightened to
	// `< 2` and the whole suite stays green, silently refusing a legal pin.
	r0, b0 := pin(`{"override":{"instances":1,"reason":"floor"}}`)
	if r0.StatusCode != 200 {
		t.Fatalf("a one-instance pin is legal and must be honoured: %d %s", r0.StatusCode, b0)
	}
	var floorDesired []byte
	if err := w.pool.QueryRow(ctx, `SELECT desired FROM services WHERE id=$1`, svc.Id).Scan(&floorDesired); err != nil {
		t.Fatal(err)
	}
	var fd map[string]any
	_ = json.Unmarshal(floorDesired, &fd)
	ov, _ := fd["override"].(map[string]any)
	if ov["instances"] != float64(1) {
		t.Fatalf("a one-instance pin did not reach the cell: %s", floorDesired)
	}

}

// The activity feed hides itself from every principal WITHOUT standing — on
// BOTH transports.
//
// `listEvents` is x-streamable: the same path is served by the strict JSON
// handler and, when the client sends `Accept: text/event-stream`, by
// `events.Streamer`. The 404-for-no-standing conversion was added to the JSON
// half only, so one request header restored the existence oracle it was added
// to close — and the JSON half's assertion made the endpoint look fenced. An
// unknown env already 404s on both, so a 403 on either is a positive signal
// that this env id is real.
//
// FOUR principals, because the conversion has four outcomes and an earlier
// version of this comment claimed coverage the body did not have — the same
// failure as the false claim it was written next to. Anonymous (no credentials
// at all, the cheapest oracle and the one the SSE ordering bug exposed);
// non-member `membership:` → 404; org key scoped elsewhere `key:` → 404; and a
// member who merely lacks observe.read `role:` → 403. The last is the arm the
// conversion must NOT swallow: without it the whole check can be replaced with
// "every denial is a 404" and everything else here still passes.
func TestTheActivityFeedHidesItselfFromPrincipalsWithoutStanding(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "fence-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"fenceco"}`, ownerCk)
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
	path := "/v1/envs/" + env.ID + "/events?kind=scale"

	get := func(t *testing.T, cookie, accept string) int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, w.srv.URL+path, nil)
		req.Header.Set("Cookie", cookie)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		r.Body.Close()
		return r.StatusCode
	}

	strangerCk, _ := w.signupUser(t, "fence-stranger@example.com")
	// The control: an env that does not exist 404s for the owner on both
	// transports. That is what a no-standing denial has to be indistinguishable
	// FROM, so without it "404" proves nothing.
	for _, accept := range []string{"", "text/event-stream"} {
		req, _ := http.NewRequest(http.MethodGet, w.srv.URL+"/v1/envs/env_doesnotexist/events?kind=scale", nil)
		req.Header.Set("Cookie", ownerCk)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode != 404 {
			t.Fatalf("an unknown env returns %d (accept=%q); the fence below is only meaningful if this is 404", r.StatusCode, accept)
		}
	}

	// ANONYMOUS first, because it is the cheapest oracle and the one this test
	// originally missed: SSE looked up the env BEFORE checking the principal, so
	// with no credentials a fabricated env answered 404 and a real one 401. Both
	// transports must answer the same thing to a caller with nothing.
	for _, accept := range []string{"", "text/event-stream"} {
		var codes []int
		for _, envPath := range []string{env.ID, "env_doesnotexist"} {
			req, _ := http.NewRequest(http.MethodGet, w.srv.URL+"/v1/envs/"+envPath+"/events?kind=scale", nil)
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			r, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, r.Body)
			r.Body.Close()
			codes = append(codes, r.StatusCode)
		}
		if codes[0] != codes[1] {
			t.Fatalf("anonymous (accept=%q): real env → %d, fabricated env → %d — with no credentials at all, the difference IS the oracle", accept, codes[0], codes[1])
		}
	}

	// A NON-MEMBER: 404 on both transports.
	if got := get(t, strangerCk, ""); got != 404 {
		t.Fatalf("JSON: a non-member got %d, want 404 — anything else separates a real env id from a fabricated one", got)
	}
	if got := get(t, strangerCk, "text/event-stream"); got != 404 {
		t.Fatalf("SSE: a non-member got %d, want 404 — the JSON half is fenced and this one is not, so one request header is the oracle", got)
	}

	// An ORG KEY SCOPED ELSEWHERE: also no standing, also 404 on both. This is
	// the `key:` arm, and dropping it from the conversion used to survive the
	// entire suite on both transports while a doc comment said it was covered.
	otherCk, otherOwner := w.signupUser(t, "fence-otherorg@example.com")
	_ = otherOwner
	resp, body = w.post(t, "/v1/orgs", `{"name":"otherfenceco"}`, otherCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg(other): %d %s", resp.StatusCode, body)
	}
	var otherOrg struct{ Id string }
	_ = json.Unmarshal([]byte(body), &otherOrg)
	rk, kb := w.post(t, "/v1/orgs/"+otherOrg.Id+"/api-keys",
		`{"name":"obs","scope":"full","permissions":["observe.read"]}`, otherCk)
	if rk.StatusCode != 201 {
		t.Fatalf("createApiKey: %d %s", rk.StatusCode, kb)
	}
	var key struct{ Token string }
	_ = json.Unmarshal([]byte(kb), &key)
	for _, accept := range []string{"", "text/event-stream"} {
		req, _ := http.NewRequest(http.MethodGet, w.srv.URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+key.Token)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if r.StatusCode != 404 {
			t.Fatalf("a key scoped to another org got %d (accept=%q), want 404 — a key:… denial is no standing, exactly like a non-member", r.StatusCode, accept)
		}
	}

	// A MEMBER WHO MERELY LACKS THE PERMISSION: an honest 403, on both
	// transports. This is the arm the conversion must NOT swallow — without it,
	// the whole prefix check can be replaced with "every denial is a 404" and
	// every other assertion here still passes. `billing` is the role without
	// observe.read (rbac/matrix.csv).
	memberCk, memberID := w.signupUser(t, "fence-billing@example.com")
	if err := w.svc.AddMember(ctx, org.Id, memberID, "billing", ownerID); err != nil {
		t.Fatal(err)
	}
	if got := get(t, memberCk, ""); got != 403 {
		t.Fatalf("JSON: a member lacking observe.read got %d, want 403 — turning this into a 404 would hide the org from its own members and make the remediation unreachable", got)
	}
	if got := get(t, memberCk, "text/event-stream"); got != 403 {
		t.Fatalf("SSE: a member lacking observe.read got %d, want 403", got)
	}
}

// A row poisoned by the PRE-O16 overflow must make the spend cap fail CLOSED.
//
// O16's headline claim, and it was entirely unpinned: its QA review mutated five
// separate fail-closed arms — the `committed` and `limit` re-validations, the
// not-representable branch, UpdateService's prior-price arm, and the
// ErrStoredAmountUnrepresentable → 409 map — and ALL FIVE survived the suite.
// The behaviour was correct; nothing forced it to stay correct.
//
// Not hypothetical: before O16, `postgres {storage_gb: 1e18}` returned 200 with
// monthly_total_cents = -5340232221128652948 and that value was PERSISTED.
// enforceBudget projects against SumOrgMonthlyEstimate, so one such row left the
// org's committed run-rate at ~-5.3e18 and its cap bypassed for every later
// create. After O16 the same row must REFUSE — with a problem+json carrying a
// remediation, because "contact support" with no message is not a remediation.
func TestAPoisonedStoredPriceMakesTheSpendCapFailClosed(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "failclosed@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"failclosedco"}`, ownerCk)
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

	create := func(name string) (int, string) {
		r, b := w.post(t, "/v1/estimates",
			`{"env":"`+env.ID+`","services":[{"product":"web","name":"`+name+`","shape":{"size":"standard-1"}}]}`, ownerCk)
		if r.StatusCode != 200 {
			t.Fatalf("createEstimate: %d %s", r.StatusCode, b)
		}
		var est struct{ Id string }
		_ = json.Unmarshal([]byte(b), &est)
		r2, b2 := w.post(t, "/v1/envs/"+env.ID+"/services",
			`{"name":"`+name+`","product":"web","estimate_id":"`+est.Id+`","shape":{"size":"standard-1"}}`, ownerCk)
		return r2.StatusCode, b2
	}

	code, b := create("first")
	if code != 201 {
		t.Fatalf("the first service must create normally: %d %s", code, b)
	}
	var one struct{ Id string }
	_ = json.Unmarshal([]byte(b), &one)

	// Poison it with exactly the value main produced, then give the org a cap so
	// enforceBudget runs at all (no budget row means uncapped).
	if _, err := w.pool.Exec(ctx,
		`UPDATE services SET monthly_estimate_cents = $2 WHERE id = $1`, one.Id, int64(-5340232221128652948)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx,
		`INSERT INTO budgets (org_id, limit_cents) VALUES ($1,$2)
		 ON CONFLICT (org_id) DO UPDATE SET limit_cents = $2`, org.Id, int64(10000)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = w.pool.Exec(context.Background(), `DELETE FROM budgets WHERE org_id=$1`, org.Id)
	})

	code, b = create("second")
	if code != 409 {
		t.Fatalf("a create against an org holding an unrepresentable stored price returned %d %s — want 409. "+
			"Before O16 this returned 201, because the negative committed total made every projection look far under the cap; "+
			"a permissive answer here IS the spend-cap bypass", code, b)
	}
	if !strings.Contains(b, `"remediation"`) {
		t.Fatalf("the 409 carries no remediation: %s — every problem+json must (AGENTS.md hard rule)", b)
	}
	var refused int
	if err := w.pool.QueryRow(ctx,
		`SELECT count(*) FROM services WHERE env_id=$1 AND name='second'`, env.ID).Scan(&refused); err != nil {
		t.Fatal(err)
	}
	if refused != 0 {
		t.Fatal("the refused create still inserted a row — a fail-closed cap that provisions anyway is not fail-closed")
	}

	// --- the other three arms this branch adds or rewrites ------------------
	//
	// O26's review found that pinning only the `committed` re-validation left
	// FOUR of the five fail-closed arms surviving mutation — including two this
	// branch introduces. Each is driven here, because the behaviour was correct
	// and nothing was holding it there.

	// (a) UpdateService's PRIOR-PRICE arm. A poisoned stored price must not
	// become the baseline an increase is measured against. Without the guard,
	// money.FromInt's error is ignored and the delta is computed from a garbage
	// prior — the wrapped row silently sets the reference point.
	r, b2 := w.patch(t, "/v1/services/"+one.Id,
		`{"shape":{"size":"standard-1","instances":4}}`, ownerCk)
	if r.StatusCode != 409 {
		t.Fatalf("PATCHing a service whose stored price is unrepresentable returned %d %s — want 409; "+
			"a permissive answer prices the change against a garbage baseline", r.StatusCode, b2)
	}
	if !strings.Contains(b2, `"remediation"`) {
		t.Fatalf("the PATCH 409 carries no remediation: %s", b2)
	}
	// WHICH arm answered is the assertion, not merely that something did.
	// Removing the prior-price guard still yields a 409 here — enforceBudget's
	// `committed` re-validation catches the same poisoned row one step later —
	// so a status-code-only check cannot tell the two apart and the guard
	// survives mutation. The messages differ: this arm names THIS SERVICE's
	// stored estimate, the other names the ORGANIZATION's committed spend.
	if !strings.Contains(b2, "this service's stored monthly estimate") {
		t.Fatalf("the PATCH 409 did not come from the prior-price guard: %s — "+
			"if it says \"committed monthly spend\" then enforceBudget caught it instead, "+
			"which means the guard that stops a garbage baseline is not the thing holding", b2)
	}

	// (b) The NOT-REPRESENTABLE branch of enforceBudget. Dropping
	// `notRepresentable ||` makes an overflowing projection compare Zero > limit
	// = false, i.e. FAIL OPEN — the exact defect O16 exists to close. Drive it
	// with a committed total at the ceiling so the projection cannot be summed.
	if _, err := w.pool.Exec(ctx,
		`UPDATE services SET monthly_estimate_cents = $2 WHERE id = $1`, one.Id, money.MaxMonthly); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx,
		`UPDATE budgets SET limit_cents = $2 WHERE org_id = $1`, org.Id, money.MaxMonthly); err != nil {
		t.Fatal(err)
	}
	code, b = create("third")
	if code != 402 {
		t.Fatalf("a create whose projection cannot be represented returned %d %s — want 402. "+
			"Overflow must be treated as OVER cap: a number we cannot represent is not one we can prove is affordable, "+
			"and without that arm the projection compares Zero > limit and FAILS OPEN", code, b)
	}
	// `not_representable` lives in the SPINE payload, never in the API body, so
	// asserting on it here would be a condition describing something it does not
	// own — the shape of the dead switch arm this branch's first round removed.
	if !strings.Contains(b, "cannot be evaluated") {
		t.Fatalf("the 402 does not say the projection was not computable: %s", b)
	}
	// The spine payload is the audit record, and the omit-not-null fix is only
	// real if something reads it. Decode into a map and assert ABSENCE — `null`
	// would decode to 0 in a typed reader, which is the "row of zeros" the fix
	// exists to avoid, so zero-ness is not the check.
	var capDetail []byte
	if err := w.pool.QueryRow(ctx,
		`SELECT detail FROM events WHERE org_id=$1 AND action='billing.spend_cap_reached'
		 ORDER BY at DESC LIMIT 1`, org.Id).Scan(&capDetail); err != nil {
		t.Fatal(err)
	}
	var capFields map[string]any
	if err := json.Unmarshal(capDetail, &capFields); err != nil {
		t.Fatalf("the spine payload is not valid JSON: %s", capDetail)
	}
	if _, present := capFields["projected_cents"]; present {
		t.Fatalf("projected_cents is present on the not-representable branch: %s — "+
			"it must be OMITTED, not null: a consumer typed int64 reads null as 0, "+
			"indistinguishable from a real zero", capDetail)
	}
	if _, present := capFields["current_cents"]; present {
		t.Fatalf("current_cents is present on the not-representable branch: %s — "+
			"absence must uniformly mean not computable", capDetail)
	}
	if capFields["reason"] != "not_representable" {
		t.Fatalf("the spine payload does not carry the reason: %s", capDetail)
	}

	// (c) An out-of-range stored BUDGET (`lErr`). `limit` is a stored value like
	// the others; leaving it unchecked meant an out-of-range limit_cents was
	// evaluated silently while an out-of-range committed failed closed.
	if _, err := w.pool.Exec(ctx,
		`UPDATE services SET monthly_estimate_cents = 100 WHERE id = $1`, one.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx,
		`UPDATE budgets SET limit_cents = $2 WHERE org_id = $1`, org.Id, int64(-1)); err != nil {
		t.Fatal(err)
	}
	code, b = create("fourth")
	if code != 409 {
		t.Fatalf("a create against an org whose stored budget is out of range returned %d %s — want 409; "+
			"the same input class must not be treated two ways", code, b)
	}

	// (d) A poisoned ESTIMATE, not a poisoned service. Cents.UnmarshalJSON
	// refuses an out-of-range stored `lines` amount, and that must surface as a
	// 409 telling the caller to re-price — not as a bare 500 they can only
	// retry into. Nothing else in the suite writes such a row, so without this
	// the ErrStoredAmountUnrepresentable map is unreachable and its removal
	// survives every test.
	if _, err := w.pool.Exec(ctx,
		`UPDATE budgets SET limit_cents = 100000000 WHERE org_id = $1`, org.Id); err != nil {
		t.Fatal(err)
	}
	r5, b5 := w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"web","name":"poisoned","shape":{"size":"standard-1"}}]}`, ownerCk)
	if r5.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", r5.StatusCode, b5)
	}
	var poisonedEst struct{ Id string }
	_ = json.Unmarshal([]byte(b5), &poisonedEst)
	if _, err := w.pool.Exec(ctx,
		`UPDATE estimates SET lines = $2::jsonb WHERE id = $1`, poisonedEst.Id,
		`[{"name":"poisoned","product":"web","intent":"app","monthly_cents":-5340232221128652948,"basis":"fixed","egress_note":null}]`); err != nil {
		t.Fatal(err)
	}
	r6, b6 := w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"poisoned","product":"web","estimate_id":"`+poisonedEst.Id+`","shape":{"size":"standard-1"}}`, ownerCk)
	if r6.StatusCode != 409 {
		t.Fatalf("creating from an estimate whose stored lines hold an unrepresentable amount returned %d %s — want 409. "+
			"A bare 500 tells the caller nothing and they can only retry into it", r6.StatusCode, b6)
	}
	if !strings.Contains(b6, "representable") {
		t.Fatalf("the 409 does not name the cause: %s", b6)
	}

	// The OTHER half of the conditional. money.Cents refuses both directions, and
	// case (d) above only drives ErrNegative — reducing the guard to
	// `errors.Is(err, money.ErrNegative)` alone survived it. A POSITIVE
	// out-of-range value is not hypothetical: O16's own evidence records
	// `PriceAll of two such lines -> +7766279631452245720`, which is exactly the
	// shape a legacy row can hold, and it would have fallen to the plain 500.
	r9, b9 := w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"web","name":"toobig","shape":{"size":"standard-1"}}]}`, ownerCk)
	if r9.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", r9.StatusCode, b9)
	}
	var tooBigEst struct{ Id string }
	_ = json.Unmarshal([]byte(b9), &tooBigEst)
	if _, err := w.pool.Exec(ctx,
		`UPDATE estimates SET lines = $2::jsonb WHERE id = $1`, tooBigEst.Id,
		`[{"name":"toobig","product":"web","intent":"app","monthly_cents":7766279631452245720,"basis":"fixed","egress_note":null}]`); err != nil {
		t.Fatal(err)
	}
	r10, b10 := w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"toobig","product":"web","estimate_id":"`+tooBigEst.Id+`","shape":{"size":"standard-1"}}`, ownerCk)
	if r10.StatusCode != 409 {
		t.Fatalf("a stored amount ABOVE MaxMonthly returned %d %s — want 409, same as a negative one. "+
			"Both are out of range and both need the same remediation", r10.StatusCode, b10)
	}

	// (e) The other side of (d): a decode failure that is NOT a money-range
	// problem must stay a 500 with an event_id, not become a "re-price this
	// estimate" 409. An earlier version wrapped every json.Unmarshal error as
	// unrepresentable, so a truncated or schema-drifted blob was diagnosed as a
	// money problem and a genuine corruption never reached problem.Internal —
	// an outage disguised as a client error.
	r7, b7 := w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"web","name":"corrupt","shape":{"size":"standard-1"}}]}`, ownerCk)
	if r7.StatusCode != 200 {
		t.Fatalf("createEstimate: %d %s", r7.StatusCode, b7)
	}
	var corruptEst struct{ Id string }
	_ = json.Unmarshal([]byte(b7), &corruptEst)
	// Valid JSON, wrong shape: `name` is a number. Not a money error.
	if _, err := w.pool.Exec(ctx,
		`UPDATE estimates SET lines = $2::jsonb WHERE id = $1`, corruptEst.Id,
		`[{"name":12345,"product":"web","monthly_cents":100}]`); err != nil {
		t.Fatal(err)
	}
	r8, b8 := w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"corrupt","product":"web","estimate_id":"`+corruptEst.Id+`","shape":{"size":"standard-1"}}`, ownerCk)
	if r8.StatusCode != 500 {
		t.Fatalf("a CORRUPT stored lines blob returned %d %s — want 500. It is not a money-range problem, "+
			"and diagnosing it as one hides an outage behind a client error the caller cannot act on", r8.StatusCode, b8)
	}
	if strings.Contains(b8, "representable") {
		t.Fatalf("a corrupt blob was reported as an unrepresentable amount: %s", b8)
	}
}

// THE ORG'S OWN PLAN, END TO END — the control-plane half of the envelope.
//
// The cell-agent side is well covered: it proves the renderer emits what it is
// handed. billing proves the numbers are the founder's. NOTHING proved the JOIN
// — that THIS org's plan becomes THIS service's envelope. Measured: rewriting
// `envelopeFor` to ignore `org.Plan` and always return pro's envelope survived
// the entire container-backed suite, and so did `CreateService` shipping an
// empty Quota. That is the exact defect the task exists to remove ("a Free org
// and a Business org get the identical envelope"), and it is the sibling
// representation of the mutation already killed on the renderer.
//
// Two plans, because one plan cannot distinguish a lookup from a constant.
func TestTheDesiredDocCarriesTheOrgsOwnPlanEnvelope(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	want := map[string]map[string]any{
		"free":     {"cpu": "1", "memory": "2Gi", "storage": "10Gi"},
		"business": {"cpu": "12", "memory": "24Gi", "storage": "200Gi"},
	}

	for plan, envelope := range want {
		t.Run(plan, func(t *testing.T) {
			ck, uid := w.signupUser(t, plan+"-envelope@example.com")
			resp, body := w.post(t, "/v1/orgs", `{"name":"`+plan+`co"}`, ck)
			if resp.StatusCode != 201 {
				t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
			}
			var org struct{ Id string }
			_ = json.Unmarshal([]byte(body), &org)
			if _, err := w.pool.Exec(ctx, `UPDATE orgs SET plan = $2 WHERE id = $1`, org.Id, plan); err != nil {
				t.Fatal(err)
			}
			orgRow, err := w.svc.GetOrg(ctx, org.Id)
			if err != nil {
				t.Fatal(err)
			}
			_, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", uid)
			if err != nil {
				t.Fatal(err)
			}
			resp, body = w.post(t, "/v1/estimates",
				`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev"}}]}`, ck)
			if resp.StatusCode != 200 {
				t.Fatalf("estimate: %d %s", resp.StatusCode, body)
			}
			var est struct{ Id string }
			_ = json.Unmarshal([]byte(body), &est)
			resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
				`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev"}}`, ck)
			if resp.StatusCode != 201 {
				t.Fatalf("create: %d %s", resp.StatusCode, body)
			}
			var svc struct{ Id string }
			_ = json.Unmarshal([]byte(body), &svc)

			var desired map[string]any
			if err := w.pool.QueryRow(ctx, `SELECT desired FROM services WHERE id = $1`, svc.Id).Scan(&desired); err != nil {
				t.Fatal(err)
			}
			got, ok := desired["quota"].(map[string]any)
			if !ok {
				t.Fatalf("a %s org's service shipped NO envelope — the agent refuses to render, "+
					"so the service never converges and the environment has no ceiling: %v", plan, desired)
			}
			for dim, wantVal := range envelope {
				if got[dim] != wantVal {
					t.Errorf("a %s org's service carries %s=%v, want %v — the control plane is not "+
						"resolving THIS org's plan", plan, dim, got[dim], wantVal)
				}
			}
		})
	}
}

// EVERY PATH THAT WRITES `desired` MUST SHIP AN ENVELOPE.
//
// desiredDoc gained a parameter and four call sites changed; three of them
// (UpdateService, the override-expiry sweep, delete) had no test inspecting the
// resulting document, and dropping the envelope from all three was green.
// Table-driven so a new writer of `desired` has to be added here.
func TestEveryDesiredDocWritingPathShipsAnEnvelope(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ck, uid := w.signupUser(t, "paths@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"pathsco"}`, ck)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	if _, err := w.pool.Exec(ctx, `UPDATE orgs SET plan = 'business' WHERE id = $1`, org.Id); err != nil {
		t.Fatal(err)
	}
	orgRow, err := w.svc.GetOrg(ctx, org.Id)
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", uid)
	if err != nil {
		t.Fatal(err)
	}
	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev"}}]}`, ck)
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev"}}`, ck)
	if resp.StatusCode != 201 {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)

	envelopeOf := func(t *testing.T, label string) {
		t.Helper()
		var desired map[string]any
		if err := w.pool.QueryRow(ctx, `SELECT desired FROM services WHERE id = $1`, svc.Id).Scan(&desired); err != nil {
			t.Fatal(err)
		}
		q, ok := desired["quota"].(map[string]any)
		if !ok || q["cpu"] != "12" {
			t.Fatalf("after %s the desired doc carries %v, want business's 12/24Gi/200Gi — "+
				"a doc with no envelope makes the agent refuse to render, forever, with no "+
				"writeback", label, desired["quota"])
		}
	}
	envelopeOf(t, "create")

	if _, err := w.prov.UpdateService(ctx, mustGetSvc(t, w, svc.Id), org.Id, uid,
		map[string]any{"ha": true}, nil, nil); err != nil {
		t.Fatal(err)
	}
	envelopeOf(t, "a shape PATCH")

	// The override-expiry SWEEP is the third writer of `desired`. Seeded directly
	// rather than through the handler, because the handler's own validation is
	// not what this test is about — the sweep rewriting a doc WITHOUT an envelope
	// is.
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if _, err := w.pool.Exec(ctx,
		`UPDATE services SET override = $2::jsonb WHERE id = $1`, svc.Id,
		`{"instances":3,"reason":"load test","expires_at":"`+past+`"}`); err != nil {
		t.Fatal(err)
	}
	sweepCtx, stop := context.WithCancel(ctx)
	go w.prov.RunOverrideExpiry(sweepCtx, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	deadline := time.Now().Add(10 * time.Second)
	for {
		var ov []byte
		if err := w.pool.QueryRow(ctx, `SELECT override FROM services WHERE id = $1`, svc.Id).Scan(&ov); err != nil {
			t.Fatal(err)
		}
		if len(ov) == 0 || string(ov) == "null" {
			break
		}
		if time.Now().After(deadline) {
			stop()
			t.Fatal("the override-expiry sweep never cleared the expired pin")
		}
		time.Sleep(50 * time.Millisecond)
	}
	stop()
	envelopeOf(t, "the override-expiry sweep")
}

// THE BACKFILL, RUN AS THE SHIPPED FILE, AGAINST ROWS THAT ACTUALLY EXIST.
//
// Every container-backed test already applies this migration — against an EMPTY
// database, where an UPDATE that matches nothing is indistinguishable from one
// that is correct. This seeds the rows the migration exists for and then
// executes the .sql file itself (not a paraphrase of it, which would test this
// test's SQL and ship the file's).
//
// What makes it a deploy-stop rather than a nicety: tenancy.Render REFUSES a
// spec with no envelope, and the renderer calls it on every converge, so a
// service whose doc predates US-3.3e does not run unbounded — it fails to
// converge forever, in a retry loop with no customer-visible status.
func TestTheBackfillGivesEveryExistingServiceItsOrgsEnvelope(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	sql, err := os.ReadFile("../platform/db/migrations/20260823140000_service_quota_backfill.up.sql")
	if err != nil {
		t.Fatal(err)
	}

	// Two plans, because one cannot tell a per-org lookup from a constant, plus a
	// row that must NOT be touched.
	type seeded struct{ id, plan string }
	var rows []seeded
	var untouched string
	for i, plan := range []string{"free", "enterprise"} {
		ck, uid := w.signupUser(t, fmt.Sprintf("backfill-%d@example.com", i))
		resp, body := w.post(t, "/v1/orgs", fmt.Sprintf(`{"name":"backfill%d"}`, i), ck)
		if resp.StatusCode != 201 {
			t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
		}
		var org struct{ Id string }
		_ = json.Unmarshal([]byte(body), &org)
		if _, err := w.pool.Exec(ctx, `UPDATE orgs SET plan = $2 WHERE id = $1`, org.Id, plan); err != nil {
			t.Fatal(err)
		}
		orgRow, err := w.svc.GetOrg(ctx, org.Id)
		if err != nil {
			t.Fatal(err)
		}
		_, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", uid)
		if err != nil {
			t.Fatal(err)
		}
		resp, body = w.post(t, "/v1/estimates",
			`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"dev"}}]}`, ck)
		if resp.StatusCode != 200 {
			t.Fatalf("estimate: %d %s", resp.StatusCode, body)
		}
		var est struct{ Id string }
		_ = json.Unmarshal([]byte(body), &est)
		resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
			`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev"}}`, ck)
		if resp.StatusCode != 201 {
			t.Fatalf("create: %d %s", resp.StatusCode, body)
		}
		var svc struct{ Id string }
		_ = json.Unmarshal([]byte(body), &svc)
		rows = append(rows, seeded{svc.Id, plan})

		// Age the row back to what a pre-US-3.3e doc looks like.
		if _, err := w.pool.Exec(ctx,
			`UPDATE services SET desired = desired - 'quota' WHERE id = $1`, svc.Id); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			// A second service in the SAME org whose doc is '{}' — it predates the
			// reconciler columns and has no doc to extend. desiredDoc writes it
			// whole on the next touch; the migration must leave it alone rather
			// than manufacture a doc whose only key is a quota.
			resp, body = w.post(t, "/v1/estimates",
				`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"old","shape":{"size":"dev"}}]}`, ck)
			if resp.StatusCode != 200 {
				t.Fatalf("estimate: %d %s", resp.StatusCode, body)
			}
			_ = json.Unmarshal([]byte(body), &est)
			resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
				`{"name":"old","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"dev"}}`, ck)
			if resp.StatusCode != 201 {
				t.Fatalf("create: %d %s", resp.StatusCode, body)
			}
			var old struct{ Id string }
			_ = json.Unmarshal([]byte(body), &old)
			untouched = old.Id
			if _, err := w.pool.Exec(ctx,
				`UPDATE services SET desired = '{}'::jsonb WHERE id = $1`, old.Id); err != nil {
				t.Fatal(err)
			}
		}
	}

	gen := func(id string) int64 {
		var g int64
		if err := w.pool.QueryRow(ctx, `SELECT generation FROM services WHERE id = $1`, id).Scan(&g); err != nil {
			t.Fatal(err)
		}
		return g
	}
	before := map[string]int64{}
	for _, r := range rows {
		before[r.id] = gen(r.id)
	}
	beforeUntouched := gen(untouched)

	if _, err := w.pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("the shipped backfill did not execute: %v", err)
	}

	want := map[string]map[string]any{
		"free":       {"cpu": "1", "memory": "2Gi", "storage": "10Gi"},
		"enterprise": {"cpu": "16", "memory": "32Gi", "storage": "250Gi"},
	}
	for _, r := range rows {
		var desired map[string]any
		if err := w.pool.QueryRow(ctx, `SELECT desired FROM services WHERE id = $1`, r.id).Scan(&desired); err != nil {
			t.Fatal(err)
		}
		got, _ := desired["quota"].(map[string]any)
		if !reflect.DeepEqual(got, want[r.plan]) {
			t.Errorf("%s service: backfilled %v, want %v", r.plan, got, want[r.plan])
		}
		// The doc it extended must survive — `desired = jsonb_build_object(...)`
		// would pass the assertion above while erasing product and namespace.
		if desired["product"] != "postgres" || desired["namespace"] == nil {
			t.Errorf("%s service: the backfill replaced the doc instead of extending it: %v", r.plan, desired)
		}
		if g := gen(r.id); g != before[r.id]+1 {
			t.Errorf("%s service: generation %d, want %d — the agent polls "+
				"observed_generation < generation, so without the bump the corrected doc is never fetched",
				r.plan, g, before[r.id]+1)
		}
	}

	var emptyDoc map[string]any
	if err := w.pool.QueryRow(ctx, `SELECT desired FROM services WHERE id = $1`, untouched).Scan(&emptyDoc); err != nil {
		t.Fatal(err)
	}
	if len(emptyDoc) != 0 {
		t.Errorf("a '{}' doc was given a quota and nothing else: %v", emptyDoc)
	}
	if g := gen(untouched); g != beforeUntouched {
		t.Errorf("a skipped row's generation was bumped (%d → %d), which schedules a converge "+
			"of a doc that still cannot render", beforeUntouched, g)
	}

	// Idempotent: migrations get re-run in recovery, and `NOT (desired ? 'quota')`
	// is what stops a second pass from bumping every generation again.
	if _, err := w.pool.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if g := gen(r.id); g != before[r.id]+1 {
			t.Errorf("%s service: a second run bumped generation to %d", r.plan, g)
		}
	}
}

// A SIZE CHANGE MUST PRICE THE SAME WHENEVER THE ROW WAS CREATED.
//
// T3.4c made an unset postgres storage_gb resolve to the size's included_gb, so
// a `standard` created after it persists 50. On a later downgrade to `dev` the
// merge carries that 50, and dev includes 0, so the row prices
// 1900 + 50GB*50c = 4400. A row created BEFORE T3.4c stores 0 and the identical
// PATCH priced 1900 — two customers, one resulting configuration, two prices,
// decided by signup date.
//
// 4400 is TODAY'S ARITHMETIC, and this test pins it as such — not as a ruling.
// What the customer should PAY for a size downgrade while storage is retained is
// recorded as ❓ NEEDS FOUNDER INPUT in docs/founder-config.md §5, with three
// live options, one of which (refuse the downgrade) would make this test fail.
// That is the correct outcome if it is ruled: the test would be updated with the
// ruling. Stated here because an earlier revision called 4400 "the right
// answer", which frames an open pricing question as settled — the one thing
// implementation must never do (founder, 2026-07-27).
//
// What is NOT open is the arithmetic below, which is the catalog's own rather
// than a new pricing rule: a PersistentVolumeClaim CANNOT SHRINK (Kubernetes supports
// expansion only, and a request below .status.capacity is rejected), so the
// customer keeps the 50Gi volume and pays for the 50 GB beyond dev's zero
// included. Billing 1900 would be giving away storage the cluster is still
// carrying — the same declared-vs-provisioned split T3.4c exists to close.
//
// The legacy rows are reconciled by migration 20260823120000, which is
// price-neutral at rest (raising a stored 0 to included_gb adds exactly zero,
// since Price charges only the positive part of storage_gb - included_gb).
// This test drives BOTH provenances through the real HTTP surface and requires
// one answer.
func TestASizeDowngradePricesTheSameWhicheverProvenanceTheRowHas(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "ratchet@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"ratchetco"}`, ownerCk)
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

	newStandard := func(name string) string {
		t.Helper()
		resp, body := w.post(t, "/v1/estimates",
			`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"`+name+`","shape":{"size":"standard"}}]}`, ownerCk)
		if resp.StatusCode != 200 {
			t.Fatalf("estimate: %d %s", resp.StatusCode, body)
		}
		var est struct{ Id string }
		_ = json.Unmarshal([]byte(body), &est)
		resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
			`{"name":"`+name+`","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"standard"}}`, ownerCk)
		if resp.StatusCode != 201 {
			t.Fatalf("create: %d %s", resp.StatusCode, body)
		}
		var svc struct{ Id string }
		_ = json.Unmarshal([]byte(body), &svc)
		return svc.Id
	}

	// (a) A row created today. The gate resolves the included storage explicitly.
	fresh := newStandard("fresh")
	var freshShape map[string]any
	if err := w.pool.QueryRow(ctx, `SELECT shape FROM services WHERE id = $1`, fresh).Scan(&freshShape); err != nil {
		t.Fatal(err)
	}
	if got := freshShape["storage_gb"]; got != float64(50) {
		t.Fatalf("a standard created today stored storage_gb=%v, want the included 50", got)
	}

	// (b) A row with the PRE-T3.4c shape, written straight to the column — the
	// state the migration exists to reconcile.
	legacy := newStandard("legacy")
	if _, err := w.pool.Exec(ctx,
		`UPDATE services SET shape = jsonb_set(shape, '{storage_gb}', '0', true) WHERE id = $1`, legacy); err != nil {
		t.Fatal(err)
	}

	// EXECUTE THE SHIPPED MIGRATION, not a paraphrase of it.
	//
	// An earlier version of this test hand-copied a DIFFERENT statement —
	// `COALESCE((shape ->> 'storage_gb')::int, 0)`, the unguarded cast the branch
	// had already replaced — under a comment claiming it "reproduces its
	// statement". So the real SQL had zero coverage, and two defects in it were
	// invisible: an unguarded cast on the JSON-number arm (a fractional or
	// out-of-int-range legacy value aborts the whole migration and leaves
	// schema_migrations dirty), and a floor-only WHERE that skipped the string
	// `"78"` — the very class the migration was written for.
	//
	// The migration runs at startup against an EMPTY database, where an UPDATE
	// that matches nothing is indistinguishable from a correct one. This runs it
	// again, against rows that exist.
	migration, err := os.ReadFile(
		"../platform/db/migrations/20260823120000_postgres_storage_ratchet.up.sql")
	if err != nil {
		t.Fatalf("the ratchet migration is missing: %v", err)
	}
	if _, err := w.pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("the shipped migration did not execute against seeded rows: %v", err)
	}

	priceAfterDowngrade := func(id string) int64 {
		t.Helper()
		row, err := w.prov.UpdateService(ctx, mustGetSvc(t, w, id), org.Id, ownerID,
			map[string]any{"size": "dev"}, nil, nil)
		if err != nil {
			t.Fatalf("downgrade %s: %v", id, err)
		}
		return row.MonthlyEstimateCents
	}

	gotFresh, gotLegacy := priceAfterDowngrade(fresh), priceAfterDowngrade(legacy)
	if gotFresh != gotLegacy {
		t.Fatalf("the same downgrade prices %d for a row created today and %d for a pre-T3.4c row — "+
			"two customers with one resulting configuration, billed differently by signup date",
			gotFresh, gotLegacy)
	}
	// 1900 (dev base) + 50 GB beyond dev's 0 included, at 50c/GB.
	const want = 1900 + 50*50
	if gotFresh != want {
		t.Fatalf("a dev keeping its 50Gi volume priced %d, want %d — the volume cannot shrink, "+
			"so billing dev's base alone gives away storage the cluster still carries", gotFresh, want)
	}
}

// A PVC CANNOT SHRINK, so a PATCH that lowers storage_gb must be refused.
//
// The T3.4c ratchet only floored at included_gb; ABOVE that a PATCH silently
// lowered both the stored value and the bill — measured at dev 200GB → 20
// dropping 11900c to 2900c — for storage the cluster is still carrying. Worse,
// the driver then renders the smaller PVC and the CSI driver rejects the shrink,
// so the row sits outstanding forever with nothing written back. Same defect
// class as the one the ratchet exists to close, in the opposite direction.
//
// Refused, not silently floored: a customer who asks for 20 and gets 200 with no
// error has been ignored. This is not a pricing decision — nobody's bill moves.
func TestStorageCannotBeReducedBecauseAVolumeCannotShrink(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, ownerID := w.signupUser(t, "shrink@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"shrinkco"}`, ownerCk)
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
		`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":{"size":"standard","storage_gb":200}}]}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("estimate: %d %s", resp.StatusCode, body)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":{"size":"standard","storage_gb":200}}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var svc struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svc)
	before := mustGetSvc(t, w, svc.Id)

	// The reduction is refused, and says what to do instead.
	_, err = w.prov.UpdateService(ctx, before, org.Id, ownerID,
		map[string]any{"storage_gb": 20}, nil, nil)
	if err == nil {
		t.Fatal("a storage reduction was accepted — the bill drops for a volume that " +
			"cannot shrink, and the next converge asks the CSI driver for a smaller PVC")
	}
	// A FIELD ERROR the customer can act on, not a 500. err.Error() is only
	// "Validation failed"; the remediation lives in the problem document, which
	// is what actually reaches the client.
	var carrier problem.Carrier
	if !errors.As(err, &carrier) {
		t.Fatalf("the refusal is not a problem+json field error — a client typo class becomes "+
			"a 500 with an event id: %v", err)
	}
	doc, _ := json.Marshal(carrier.Problem())
	for _, want := range []string{"shape.storage_gb", "cannot shrink"} {
		if !strings.Contains(string(doc), want) {
			t.Fatalf("the problem document does not carry %q — the customer is told no with no "+
				"reason and no way forward: %s", want, doc)
		}
	}

	// Nothing moved: not the stored shape, not the bill.
	after := mustGetSvc(t, w, svc.Id)
	if string(after.Shape) != string(before.Shape) {
		t.Fatalf("the refused PATCH still rewrote the shape:\n before %s\n after  %s", before.Shape, after.Shape)
	}
	if after.MonthlyEstimateCents != before.MonthlyEstimateCents {
		t.Fatalf("the refused PATCH moved the bill %d → %d",
			before.MonthlyEstimateCents, after.MonthlyEstimateCents)
	}

	// GROWING is still allowed — a volume can expand, and the bill follows.
	grown, err := w.prov.UpdateService(ctx, before, org.Id, ownerID,
		map[string]any{"storage_gb": 300}, nil, nil)
	if err != nil {
		t.Fatalf("a storage INCREASE was refused: %v", err)
	}
	if grown.MonthlyEstimateCents <= before.MonthlyEstimateCents {
		t.Fatalf("growing to 300 GB did not raise the bill (%d → %d)",
			before.MonthlyEstimateCents, grown.MonthlyEstimateCents)
	}
	// And an unrelated PATCH is unaffected by the guard.
	if _, err := w.prov.UpdateService(ctx, mustGetSvc(t, w, svc.Id), org.Id, ownerID,
		map[string]any{"ha": true}, nil, nil); err != nil {
		t.Fatalf("an unrelated shape PATCH was refused: %v", err)
	}
}

// THE PRICED STORAGE MUST SURVIVE EVERY DESIRED-DOC REWRITE, not just create.
//
// Measured: `UpdateService`'s desired doc shipping `storage_gb: 0`, and
// `expireOverride`'s doing the same, BOTH survived the full container-backed
// suite — only the create path was pinned. Downstream that is not a cosmetic
// drift: the driver floors a 0 to the size's included GB, so a 200 GB service
// re-renders as 50Gi, the CSI driver REFUSES the shrink, and the row stays
// outstanding forever with nothing written back. A PATCH that does not mention
// storage would silently do that.
func TestEveryDesiredDocRewriteKeepsThePricedStorage(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ck, uid := w.signupUser(t, "storage-doc@example.com")

	resp, body := w.post(t, "/v1/orgs", `{"name":"storageco"}`, ck)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	orgRow, err := w.svc.GetOrg(ctx, org.Id)
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", uid)
	if err != nil {
		t.Fatal(err)
	}
	const shape = `{"size":"standard","storage_gb":200}`
	resp, body = w.post(t, "/v1/estimates",
		`{"env":"`+env.ID+`","services":[{"product":"postgres","name":"db","shape":`+shape+`}]}`, ck)
	if resp.StatusCode != 200 {
		t.Fatalf("estimate: %d %s", resp.StatusCode, body)
	}
	var est struct{ Id string }
	_ = json.Unmarshal([]byte(body), &est)
	resp, body = w.post(t, "/v1/envs/"+env.ID+"/services",
		`{"name":"db","product":"postgres","estimate_id":"`+est.Id+`","shape":`+shape+`}`, ck)
	if resp.StatusCode != 201 {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var svcRow struct{ Id string }
	_ = json.Unmarshal([]byte(body), &svcRow)

	storageOf := func(t *testing.T, label string) {
		t.Helper()
		var shapeDoc, desired map[string]any
		if err := w.pool.QueryRow(ctx, `SELECT shape, desired FROM services WHERE id = $1`,
			svcRow.Id).Scan(&shapeDoc, &desired); err != nil {
			t.Fatal(err)
		}
		if got := shapeDoc["storage_gb"]; got != float64(200) {
			t.Errorf("%s: stored shape storage_gb = %v, want 200", label, got)
		}
		ds, _ := desired["shape"].(map[string]any)
		if ds == nil {
			t.Fatalf("%s: the desired doc has no shape", label)
		}
		if got := ds["storage_gb"]; got != float64(200) {
			t.Errorf("%s: DESIRED doc storage_gb = %v, want 200 — the driver floors a 0 to the "+
				"size's included GB, the CSI driver refuses the shrink, and the row stays "+
				"outstanding forever", label, got)
		}
	}
	storageOf(t, "after create")

	// A PATCH that does not mention storage at all.
	if _, err := w.prov.UpdateService(ctx, mustGetSvc(t, w, svcRow.Id), org.Id, uid,
		map[string]any{"size": "standard", "ha": true}, nil, nil); err != nil {
		t.Fatalf("patch: %v", err)
	}
	storageOf(t, "after a PATCH that never mentions storage")

	// ...and the override-expiry sweep, which rebuilds the doc from the stored shape.
	if _, err := w.pool.Exec(ctx,
		`UPDATE services SET override = $2::jsonb WHERE id = $1`, svcRow.Id,
		`{"instances":3,"reason":"test","expires_at":"2020-01-01T00:00:00Z"}`); err != nil {
		t.Fatal(err)
	}
	// RunOverrideExpiry sweeps once at startup and then on a ticker, so it is
	// driven as the production code runs it and polled until the pin clears.
	sweepCtx, stop := context.WithCancel(ctx)
	go w.prov.RunOverrideExpiry(sweepCtx, time.Hour, slog.New(slog.DiscardHandler))
	t.Cleanup(stop)
	deadline := time.Now().Add(10 * time.Second)
	for {
		var override []byte
		if err := w.pool.QueryRow(ctx, `SELECT override FROM services WHERE id = $1`,
			svcRow.Id).Scan(&override); err != nil {
			t.Fatal(err)
		}
		if len(override) == 0 || string(override) == "null" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the expired pin was never cleared")
		}
		time.Sleep(50 * time.Millisecond)
	}
	storageOf(t, "after the override-expiry sweep")
}

// THE SHIPPED RATCHET MIGRATION, ROW BY ROW.
//
// The sibling test executes the same file, but asserts through a PRICE
// comparison — so a migration that is uniformly wrong (every legacy row raised
// to ten times the included GB, say) prices both sides identically and passes.
// Measured against the pre-round-4 file: six behavioural mutations of this SQL
// survived the whole suite, including `included_gb * 10`, `GREATEST` → `LEAST`,
// dropping the `desired` half, and neutering the UPDATE entirely.
//
// One row per representation, each asserted by VALUE and by whether its
// generation moved.
func TestTheShippedRatchetMigrationRowByRow(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ck, uid := w.signupUser(t, "ratchet-rows@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"ratchetco"}`, ck)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)
	orgRow, err := w.svc.GetOrg(ctx, org.Id)
	if err != nil {
		t.Fatal(err)
	}
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", uid)
	if err != nil {
		t.Fatal(err)
	}

	type row struct {
		id, shape string
		wantGB    any // nil = the key must be absent
		wantMoved bool
	}
	rows := []row{
		{"svc_r_absent", `{"size":"standard"}`, float64(50), true},
		{"svc_r_zero", `{"size":"standard","storage_gb":0}`, float64(50), true},
		{"svc_r_str78", `{"size":"standard","storage_gb":"78"}`, float64(78), true},
		{"svc_r_50gi", `{"size":"standard","storage_gb":"50Gi"}`, float64(50), true},
		{"svc_r_frac", `{"size":"standard","storage_gb":32.5}`, float64(50), true},
		{"svc_r_huge", `{"size":"standard","storage_gb":3000000000}`, float64(3000000000), false},
		{"svc_r_ok200", `{"size":"standard","storage_gb":200}`, float64(200), false},
		{"svc_r_dev4", `{"size":"dev","storage_gb":4}`, float64(4), false},
		{"svc_r_valkey", `{"size":"standard","storage_gb":0}`, float64(0), false}, // not postgres
	}
	before := map[string]int64{}
	for _, r := range rows {
		product := "postgres"
		if r.id == "svc_r_valkey" {
			product = "valkey"
		}
		if _, err := w.pool.Exec(ctx,
			`INSERT INTO services (id, env_id, name, product, status, shape, desired, cell_id, generation, observed_generation)
			 VALUES ($1, $2, $3, $4::text, 'ready', $5::jsonb,
			         jsonb_build_object('product', $4::text, 'shape', $5::jsonb), 'cell-0', 7, 7)`,
			r.id, env.ID, r.id, product, r.shape); err != nil {
			t.Fatalf("seed %s: %v", r.id, err)
		}
		before[r.id] = 7
	}
	_ = prj

	migration, err := os.ReadFile("../platform/db/migrations/20260823120000_postgres_storage_ratchet.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("the shipped migration aborted on legacy rows: %v", err)
	}

	for _, r := range rows {
		var shape, desired map[string]any
		var gen int64
		if err := w.pool.QueryRow(ctx,
			`SELECT shape, desired, generation FROM services WHERE id = $1`, r.id).
			Scan(&shape, &desired, &gen); err != nil {
			t.Fatalf("%s: %v", r.id, err)
		}
		if got := shape["storage_gb"]; got != r.wantGB {
			t.Errorf("%s: shape.storage_gb = %v (%T), want %v", r.id, got, got, r.wantGB)
		}
		ds, _ := desired["shape"].(map[string]any)
		if ds == nil {
			t.Errorf("%s: the desired doc lost its shape", r.id)
		} else if got := ds["storage_gb"]; got != r.wantGB {
			t.Errorf("%s: DESIRED shape.storage_gb = %v, want %v — the cell renders from this "+
				"document, not from `shape`", r.id, got, r.wantGB)
		}
		if moved := gen > before[r.id]; moved != r.wantMoved {
			t.Errorf("%s: generation moved=%v (%d → %d), want moved=%v. The agent polls "+
				"observed_generation < generation, so a row corrected without a bump is fixed "+
				"in the database and NOT on the cell.", r.id, moved, before[r.id], gen, r.wantMoved)
		}
	}

	// Idempotent: a re-run must not bump anything a second time.
	after := map[string]int64{}
	for _, r := range rows {
		var g int64
		_ = w.pool.QueryRow(ctx, `SELECT generation FROM services WHERE id = $1`, r.id).Scan(&g)
		after[r.id] = g
	}
	if _, err := w.pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		var g int64
		_ = w.pool.QueryRow(ctx, `SELECT generation FROM services WHERE id = $1`, r.id).Scan(&g)
		if g != after[r.id] {
			t.Errorf("%s: a second run bumped generation %d → %d", r.id, after[r.id], g)
		}
	}
}
