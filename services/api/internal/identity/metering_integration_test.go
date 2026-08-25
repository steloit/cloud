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

	"github.com/steloit/cloud/services/api/internal/identity/store"

	"github.com/steloit/cloud/services/api/internal/metering"
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

// O38: A METERING EDGE IS IDEMPOTENT, AND THE OVER-DEDUPE CASE MATTERS MORE.
//
// The point of the key is not mainly that a retry double-bills today —
// MustEmitSpan does not retry, it logs. It is that BECAUSE retrying was unsafe we
// did not retry, so a failed emit was a silent billing GAP whose recovery path
// was a human reading a log line. Once a replay is safe, the emit can be retried
// and the gap closes.
//
// Both directions are asserted, and the second is the one that would hurt more:
// a key that dedupes too EAGERLY collapses two genuine transitions into one and
// silently drops revenue, which is invisible in a way double-billing is not.
func TestAMeteringEdgeIsIdempotentWithoutCollapsingRealTransitions(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)

	tags := metering.Tags{OrgID: "org_x", ProjectID: "prj_x", EnvID: "env_x", ServiceID: "svc_x"}
	em := metering.NewEmitter(q)
	count := func() int {
		t.Helper()
		var n int
		if err := w.pool.QueryRow(ctx,
			`select count(*) from usage_events where service_id = $1`, tags.ServiceID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	at := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	key := metering.SpanKeyForStatus(tags.ServiceID, "open", at)

	dup, err := em.EmitSpan(ctx, tags, key, "open", "postgres", 2400)
	if err != nil || dup {
		t.Fatalf("first emit: dup=%v err=%v — the first write must insert", dup, err)
	}
	// The retry. It must SUCCEED, or the retry path becomes error handling and
	// nobody writes it.
	dup, err = em.EmitSpan(ctx, tags, key, "open", "postgres", 2400)
	if err != nil {
		t.Fatalf("retrying the same edge must not error, or retry is unusable: %v", err)
	}
	if !dup {
		t.Fatal("the retry was not reported as a duplicate")
	}
	if got := count(); got != 1 {
		t.Fatalf("the same edge emitted twice wrote %d rows, want 1 — this bills twice", got)
	}

	// THE OVER-DEDUPE CASE. A service that cycles ready -> suspended -> ready
	// produces a SECOND open edge with the same (service, edge, from->to). Only
	// status_changed_at separates them, which is exactly why the column exists.
	later := at.Add(2 * time.Hour)
	if _, err := em.EmitSpan(ctx, tags, metering.SpanKeyForStatus(tags.ServiceID, "open", later),
		"open", "postgres", 2400); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 2 {
		t.Fatalf("a genuinely later transition wrote %d rows, want 2 — the key is collapsing real "+
			"transitions, which drops revenue silently and is worse than billing twice", got)
	}

	// A close at the same instant is a different edge, not a duplicate.
	if _, err := em.EmitSpan(ctx, tags, metering.SpanKeyForStatus(tags.ServiceID, "close", later),
		"close", "postgres", 2400); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 3 {
		t.Fatalf("open and close at the same instant collapsed: %d rows, want 3", got)
	}

	// A reprice pair shares a generation and is separated by the edge.
	for _, edge := range []string{"close", "open"} {
		if _, err := em.EmitSpan(ctx, tags, metering.SpanKeyForReprice(tags.ServiceID, edge, 7),
			edge, "postgres", 2400); err != nil {
			t.Fatal(err)
		}
	}
	if got := count(); got != 5 {
		t.Fatalf("the reprice pair wrote %d rows, want 5 total", got)
	}
	// …and replaying that whole pair is a no-op.
	for _, edge := range []string{"close", "open"} {
		if _, err := em.EmitSpan(ctx, tags, metering.SpanKeyForReprice(tags.ServiceID, edge, 7),
			edge, "postgres", 2400); err != nil {
			t.Fatal(err)
		}
	}
	if got := count(); got != 5 {
		t.Fatalf("replaying the reprice pair wrote %d rows, want 5 — a replay must be safe", got)
	}
}

// A random key satisfies the column and defeats the mechanism, which is worse
// than refusing because it looks like it worked.
func TestAnEmitWithoutADedupeKeyIsRefused(t *testing.T) {
	w := newWorld(t, time.Hour)
	em := metering.NewEmitter(store.New(w.pool))
	_, err := em.EmitSpan(context.Background(), metering.Tags{
		OrgID: "o", ProjectID: "p", EnvID: "e", ServiceID: "s",
	}, "", "open", "postgres", 100)
	if err == nil {
		t.Fatal("an emit with no dedupe key was accepted — a caller that forgets one would " +
			"silently lose idempotency for that edge forever")
	}
	if !strings.Contains(err.Error(), "never at random") {
		t.Fatalf("the refusal does not say what to do instead: %v", err)
	}
}
