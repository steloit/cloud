package identity_test

// T3.3: services over live HTTP — the estimate gate enforced AT THE API
// LAYER (US-3.2's law), the guarded status machine, D22 update semantics.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
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
	resp, _ = w.patch(t, "/v1/services/"+svc.Id, `{"override":{"instances":5,"reason":"load test"}}`, ownerCk)
	if resp.StatusCode != 200 {
		t.Fatalf("override: %d", resp.StatusCode)
	}
	var overrideJSON string
	if err := w.pool.QueryRow(ctx, "select override->>'expires_at' from services where id=$1", svc.Id).Scan(&overrideJSON); err != nil || overrideJSON == "" {
		t.Fatalf("override auto-expiry not recorded: %q %v", overrideJSON, err)
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
