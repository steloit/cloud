package identity_test

// T6.3: the rollup against real spans — idempotent recomputation, raw events
// retained, still-open spans accrue to the cutoff.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/metering"
	"github.com/steloit/cloud/services/api/internal/platform/money"
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

// O19 · AC 1: a rollup that cannot be represented FAILS CLOSED.
//
// money.MaxMonthly is derived so ONE service-month exactly fits an int64, which
// makes the SECOND service the wrap — in quota_usage.rate_cents, the number
// billing derives charges from. The old accumulator was `weighted += secs *
// rate`, and a wrapped value is just a number: nothing detected it.
//
// This drives the REAL Rollup against real span rows and asserts three things a
// silent wrap would each get wrong: it errors, it does not write, and it does
// not leave a stale row looking freshly computed.
func TestARollupThatCannotBeRepresentedFailsClosedAndWritesNothing(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	_, ownerID := w.signupUser(t, "overflow-owner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "overflowco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.ID)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}

	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	period := metering.Period(t0)
	plant := func(svc, edge string, at time.Time, rate int64) {
		if _, err := w.pool.Exec(ctx,
			`insert into usage_events (id, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
			 values ('use_'||substr(md5(random()::text),1,12), $1, $2, $3, $4, 'service_span', $5, 'postgres', $6, $7)`,
			org.ID, prj.ID, env.ID, svc, edge, rate, at); err != nil {
			t.Fatal(err)
		}
	}

	// A first, representable rollup, so there is a real prior value to protect.
	plant("svc_small", "open", t0, 2400)
	plant("svc_small", "close", t0.Add(time.Hour), 2400)
	em := metering.NewEmitter(store.New(w.pool))
	now := t0.AddDate(0, 1, 0)
	if err := em.Rollup(ctx, org.ID, period, now); err != nil {
		t.Fatalf("the representable rollup failed: %v", err)
	}
	var beforeUsed, beforeRate int64
	var beforeAt time.Time
	if err := w.pool.QueryRow(ctx,
		`select used, rate_cents, computed_at from quota_usage where org_id=$1 and meter='service_span_seconds' and period=$2`,
		org.ID, period).Scan(&beforeUsed, &beforeRate, &beforeAt); err != nil {
		t.Fatal(err)
	}
	if beforeRate != 3600*2400 {
		t.Fatalf("baseline rate_cents = %d, want %d", beforeRate, 3600*2400)
	}

	// Now two services at the representable ceiling for the whole month. Each
	// alone is exactly what MaxMonthly permits; together they are the wrap.
	for _, id := range []string{"svc_big_a", "svc_big_b"} {
		plant(id, "open", t0, money.MaxMonthly)
		plant(id, "close", t0.AddDate(0, 1, 0), money.MaxMonthly)
	}

	err = em.Rollup(ctx, org.ID, period, now)
	if err == nil {
		t.Fatal("a rollup whose accrual cannot be represented SUCCEEDED — the previous " +
			"implementation wrapped here, and a wrapped rate_cents is what the invoice, the " +
			"MTD spend and the hard cap all read as money")
	}
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("err = %v, want money.ErrOverflow", err)
	}

	var afterUsed, afterRate int64
	var afterAt time.Time
	if err := w.pool.QueryRow(ctx,
		`select used, rate_cents, computed_at from quota_usage where org_id=$1 and meter='service_span_seconds' and period=$2`,
		org.ID, period).Scan(&afterUsed, &afterRate, &afterAt); err != nil {
		t.Fatal(err)
	}
	if afterRate != beforeRate || afterUsed != beforeUsed {
		t.Errorf("the failed rollup still wrote: used %d→%d, rate_cents %d→%d. A partial or "+
			"wrapped figure upserted over the previous value is worse than not recomputing",
			beforeUsed, afterUsed, beforeRate, afterRate)
	}
	if !afterAt.Equal(beforeAt) {
		t.Errorf("computed_at moved (%v → %v) on a rollup that failed — the row would read as "+
			"freshly computed when it was not", beforeAt, afterAt)
	}

	// The failure is scoped to the PERIOD whose accrual is unrepresentable, not
	// to the org or the emitter. usage_events is append-only (a trigger refuses
	// DELETE — corrections are new events), so this is asserted the only way it
	// can be: the next period, whose spans all closed at its start, still rolls
	// up. An org with one unbillable month must remain billable in the others.
	next := metering.Period(t0.AddDate(0, 1, 0))
	if err := em.Rollup(ctx, org.ID, next, t0.AddDate(0, 2, 0)); err != nil {
		t.Fatalf("period %s could not be rolled up after %s failed — the failure is not scoped: %v",
			next, period, err)
	}
	var nextRate int64
	if err := w.pool.QueryRow(ctx,
		`select rate_cents from quota_usage where org_id=$1 and meter='service_span_seconds' and period=$2`,
		org.ID, next).Scan(&nextRate); err != nil {
		t.Fatal(err)
	}
	if nextRate != 0 {
		t.Errorf("period %s accrued %d cents from spans that closed at its start", next, nextRate)
	}
}

// O19: A SPAN WHOSE RATE IS NOT A REPRESENTABLE AMOUNT.
//
// The sibling test above overflows at the NARROWING step (Accrual.Int64), which
// leaves the accumulation itself valid. This one fails inside the accumulation —
// `money.FromInt(rate)` refusing a rate above MaxMonthly — and they are different
// branches: measured, `if accErr != nil` could be disabled entirely and the
// sibling test stayed green, because it never produces an accErr.
//
// rate_cents is a plain bigint column, so a value above MaxMonthly is storable;
// the estimate path bounds what it accepts, and this is the assertion that the
// ROLLUP does not trust that bound. Skipping the span instead would under-bill
// silently, which is the failure the money type exists to prevent.
func TestARollupRefusesASpanWhoseRateIsNotARepresentableAmount(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	_, ownerID := w.signupUser(t, "badrate-owner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "badrateco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.ID)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}

	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	period := metering.Period(t0)
	plant := func(svc, edge string, at time.Time, rate int64) {
		if _, err := w.pool.Exec(ctx,
			`insert into usage_events (id, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
			 values ('use_'||substr(md5(random()::text),1,12), $1, $2, $3, $4, 'service_span', $5, 'postgres', $6, $7)`,
			org.ID, prj.ID, env.ID, svc, edge, rate, at); err != nil {
			t.Fatal(err)
		}
	}
	em := metering.NewEmitter(store.New(w.pool))
	now := t0.AddDate(0, 1, 0)

	// A representable baseline first, so there is a prior value to protect.
	plant("svc_ok", "open", t0, 2400)
	plant("svc_ok", "close", t0.Add(time.Hour), 2400)
	if err := em.Rollup(ctx, org.ID, period, now); err != nil {
		t.Fatal(err)
	}
	var beforeRate int64
	if err := w.pool.QueryRow(ctx,
		`select rate_cents from quota_usage where org_id=$1 and meter='service_span_seconds' and period=$2`,
		org.ID, period).Scan(&beforeRate); err != nil {
		t.Fatal(err)
	}

	// One second at a rate above the representable ceiling: too small to overflow
	// the accumulator, so ONLY the per-rate check can catch it.
	plant("svc_bad", "open", t0, money.MaxMonthly+1)
	plant("svc_bad", "close", t0.Add(time.Second), money.MaxMonthly+1)

	err = em.Rollup(ctx, org.ID, period, now)
	if err == nil {
		t.Fatal("a span priced above the representable ceiling was accumulated silently")
	}
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("err = %v, want money.ErrOverflow", err)
	}
	var afterRate int64
	if err := w.pool.QueryRow(ctx,
		`select rate_cents from quota_usage where org_id=$1 and meter='service_span_seconds' and period=$2`,
		org.ID, period).Scan(&afterRate); err != nil {
		t.Fatal(err)
	}
	if afterRate != beforeRate {
		t.Errorf("rate_cents moved %d → %d on a refused rollup", beforeRate, afterRate)
	}
}
