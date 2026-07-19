package identity_test

// T6.3: the rollup against real spans — idempotent recomputation, raw events
// retained, still-open spans accrue to the cutoff.

import (
	"context"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/metering"
)

func TestQuotaRollup(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	_, ownerID := w.signupUser(t, "roll-owner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "rollco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.ID)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_ = prj

	// Two spans, planted directly in the raw store with known timestamps:
	// svc A: open T0, close T0+3600s (1h at 2400¢/mo)
	// svc B: open T0+1800, never closed (still running at rollup)
	t0 := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	period := metering.Period(t0)
	plant := func(svc, edge string, at time.Time, rate int64) {
		if _, err := w.pool.Exec(ctx,
			`insert into usage_events (id, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
			 values ('use_'||substr(md5(random()::text),1,12), $1, $2, $3, $4, 'service_span', $5, 'postgres', $6, $7)`,
			org.ID, prj.ID, env.ID, svc, edge, rate, at); err != nil {
			t.Fatal(err)
		}
	}
	plant("svc_a", "open", t0, 2400)
	plant("svc_a", "close", t0.Add(time.Hour), 2400)
	plant("svc_b", "open", t0.Add(30*time.Minute), 5800)

	em := metering.NewEmitter(store.New(w.pool))
	now := t0.Add(2 * time.Hour) // rollup runs 2h after T0

	if err := em.Rollup(ctx, org.ID, period, now); err != nil {
		t.Fatal(err)
	}
	var used, weighted int64
	read := func() {
		if err := w.pool.QueryRow(ctx,
			`select used, rate_cents from quota_usage where org_id=$1 and meter='service_span_seconds' and period=$2`,
			org.ID, period).Scan(&used, &weighted); err != nil {
			t.Fatal(err)
		}
	}
	read()
	// A: 3600s · B: open 90min before cutoff → 5400s. total 9000s
	if used != 9000 {
		t.Fatalf("span seconds: %d (want 9000)", used)
	}
	if weighted != 3600*2400+5400*5800 {
		t.Fatalf("weighted: %d", weighted)
	}

	// idempotent: rerun → identical row, no accumulation
	if err := em.Rollup(ctx, org.ID, period, now); err != nil {
		t.Fatal(err)
	}
	prevUsed := used
	read()
	if used != prevUsed {
		t.Fatalf("rollup accumulated: %d → %d", prevUsed, used)
	}

	// raw events retained (never consumed/deleted by rollup)
	var raw int
	if err := w.pool.QueryRow(ctx, "select count(*) from usage_events where org_id=$1", org.ID).Scan(&raw); err != nil || raw != 3 {
		t.Fatalf("raw events: %d %v", raw, err)
	}

	// a LATER rollup (B closed meanwhile) recomputes upward, still idempotent
	plant("svc_b", "close", t0.Add(3*time.Hour), 5800)
	if err := em.Rollup(ctx, org.ID, period, t0.Add(4*time.Hour)); err != nil {
		t.Fatal(err)
	}
	read()
	// A 3600 + B 2.5h=9000 → 12600
	if used != 12600 {
		t.Fatalf("recompute: %d (want 12600)", used)
	}
}
