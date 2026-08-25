package identity_test

// T6.3: the rollup against real spans — idempotent recomputation, raw events
// retained, still-open spans accrue to the cutoff.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
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
			`insert into usage_events (id, dedupe_key, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
			 values ('use_'||substr(md5(random()::text),1,12), 'seed_'||substr(md5(random()::text),1,16), $1, $2, $3, $4, 'service_span', $5, 'postgres', $6, $7)`,
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
			`insert into usage_events (id, dedupe_key, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
			 values ('use_'||substr(md5(random()::text),1,12), 'seed_'||substr(md5(random()::text),1,16), $1, $2, $3, $4, 'service_span', $5, 'postgres', $6, $7)`,
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
			`insert into usage_events (id, dedupe_key, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
			 values ('use_'||substr(md5(random()::text),1,12), 'seed_'||substr(md5(random()::text),1,16), $1, $2, $3, $4, 'service_span', $5, 'postgres', $6, $7)`,
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

// CONSECUTIVE PERIODS MUST PARTITION THE TIMELINE — no second counted twice, and
// none dropped.
//
// Two clamps in `credit()` enforce this and only one was covered. `SpanEdgesForOrg`
// has no lower time bound, so a span opened in a PRIOR period is returned to this
// period's rollup; deleting the period-START clamp on the still-open path was a
// full-suite survivor and makes a July rollup report 4,147,200s instead of
// 2,764,800s — a 50% over-bill, with June counting the same seconds again.
//
// And the window itself was pinned only by LENGTH: `AddDate(0,1,0)` replaced by
// `AddDate(0,0,31)` also survived, which makes February 31 days long and gives
// consecutive periods a 3-day overlap. A partition assertion catches both, where
// three per-period constants catch neither.
func TestConsecutivePeriodsPartitionOneLongSpan(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	_, ownerID := w.signupUser(t, "partition-owner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "partitionco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.ID)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	plant := func(edge string, at time.Time) {
		if _, err := w.pool.Exec(ctx,
			`insert into usage_events (id, dedupe_key, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
			 values ('use_'||substr(md5(random()::text),1,12), 'seed_'||substr(md5(random()::text),1,16), $1, $2, $3, 'svc_long', 'service_span', $4, 'postgres', 2400, $5)`,
			org.ID, prj.ID, env.ID, edge, at); err != nil {
			t.Fatal(err)
		}
	}
	// One span across three calendar months, opened in February (28 days in 2026)
	// and closed in April. February is chosen deliberately: a fixed 31-day window
	// would report more seconds than February contains.
	open := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	closed := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	plant("open", open)
	plant("close", closed)

	em := metering.NewEmitter(store.New(w.pool))
	now := closed.AddDate(0, 1, 0)
	total := int64(0)
	want := map[string]int64{"2026-02": 28 * 86400, "2026-03": 31 * 86400}
	for _, period := range []string{"2026-02", "2026-03"} {
		if err := em.Rollup(ctx, org.ID, period, now); err != nil {
			t.Fatalf("%s: %v", period, err)
		}
		var used, rate int64
		if err := w.pool.QueryRow(ctx,
			`select used, rate_cents from quota_usage where org_id=$1 and meter='service_span_seconds' and period=$2`,
			org.ID, period).Scan(&used, &rate); err != nil {
			t.Fatal(err)
		}
		if used != want[period] {
			t.Errorf("%s: %d seconds, want %d — the period window is not the calendar month, "+
				"or a prior-period open is not clamped to the period start", period, used, want[period])
		}
		if rate != used*2400 {
			t.Errorf("%s: rate_cents %d, want %d", period, rate, used*2400)
		}
		total += used
	}
	// THE PARTITION: the two periods together must equal the span exactly. Three
	// per-period constants can each be wrong in a way that cancels; this cannot.
	if span := int64(closed.Sub(open) / time.Second); total != span {
		t.Errorf("the two periods account for %d seconds of a %d-second span — %d seconds are "+
			"double-counted or dropped", total, span, total-span)
	}
}

// A NEGATIVE RATE IS A DIFFERENT BRANCH FROM AN OVERSIZED ONE, and it returns a
// different error. usage_events.rate_cents is a plain bigint with no CHECK >= 0,
// so the row is storable; before this change a negative rate silently REDUCED the
// bill. Measured: `accrue` skipping such spans entirely survived the whole suite,
// because both sibling tests assert on ErrOverflow and this path is ErrNegative.
func TestARollupRefusesASpanWithANegativeRate(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	_, ownerID := w.signupUser(t, "negrate-owner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "negrateco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.ID)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	period := metering.Period(t0)
	plant := func(svc, edge string, at time.Time, rate int64) {
		if _, err := w.pool.Exec(ctx,
			`insert into usage_events (id, dedupe_key, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
			 values ('use_'||substr(md5(random()::text),1,12), 'seed_'||substr(md5(random()::text),1,16), $1, $2, $3, $4, 'service_span', $5, 'postgres', $6, $7)`,
			org.ID, prj.ID, env.ID, svc, edge, rate, at); err != nil {
			t.Fatal(err)
		}
	}
	plant("svc_ok", "open", t0, 2400)
	plant("svc_ok", "close", t0.Add(time.Hour), 2400)
	plant("svc_neg", "open", t0, -2400)
	plant("svc_neg", "close", t0.Add(time.Hour), -2400)

	err = metering.NewEmitter(store.New(w.pool)).Rollup(ctx, org.ID, period, t0.AddDate(0, 1, 0))
	if err == nil {
		t.Fatal("a span with a negative rate was accumulated — it would REDUCE the bill")
	}
	if !errors.Is(err, money.ErrNegative) {
		t.Fatalf("err = %v, want money.ErrNegative (not ErrOverflow — a different branch)", err)
	}
	var n int
	if err := w.pool.QueryRow(ctx,
		`select count(*) from quota_usage where org_id=$1 and period=$2`, org.ID, period).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("the refused rollup wrote %d rows", n)
	}
}

// AC 1 SAYS THE FAILURE MUST BE OBSERVABLE, and only the "does not write" half
// was asserted: deleting BOTH slog.Error calls survived the whole suite. A
// rollup that silently declines to recompute is a billing gap nobody is told
// about (D10), which is the same failure mode as writing a wrong number.
func TestAFailedRollupIsLoud(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	_, ownerID := w.signupUser(t, "loud-owner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "loudco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.ID)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	period := metering.Period(t0)
	for _, id := range []string{"svc_a", "svc_b"} {
		for _, e := range []struct {
			edge string
			at   time.Time
		}{{"open", t0}, {"close", t0.AddDate(0, 1, 0)}} {
			if _, err := w.pool.Exec(ctx,
				`insert into usage_events (id, dedupe_key, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
				 values ('use_'||substr(md5(random()::text),1,12), 'seed_'||substr(md5(random()::text),1,16), $1, $2, $3, $4, 'service_span', $5, 'postgres', $6, $7)`,
				org.ID, prj.ID, env.ID, id, e.edge, money.MaxMonthly, e.at); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := metering.NewEmitter(store.New(w.pool)).Rollup(ctx, org.ID, period, t0.AddDate(0, 2, 0)); err == nil {
		t.Fatal("the oversized rollup succeeded — this test's premise is gone")
	}

	logged := buf.String()
	if logged == "" {
		t.Fatal("a rollup failed and logged NOTHING at error level — the billing gap is silent")
	}
	for _, want := range []string{org.ID, period, "ROLLUP OVERFLOW"} {
		if !strings.Contains(logged, want) {
			t.Errorf("the failure log does not name %q — an operator cannot act on it:\n%s", want, logged)
		}
	}
}

// AND THE OTHER ARM — the one that can actually happen.
//
// Rollup logs from TWO places: the accumulation error and the narrowing error.
// The test above drives the NARROWING arm, which needs a fleet whose accrual
// exceeds int64. Measured: deleting only the accumulation arm's slog.Error
// survives the full identity package, so observability was verified on the arm
// that cannot occur and unverified on the arm that can — an unrepresentable or
// negative `usage_events.rate_cents`, in a column that is a plain bigint with no
// CHECK.
func TestAnUnrepresentableRateIsAlsoLoud(t *testing.T) {
	for _, tc := range []struct {
		name string
		rate int64
	}{
		{"above the representable ceiling", money.MaxMonthly + 1},
		{"negative", -2400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, time.Hour)
			ctx := context.Background()
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))
			t.Cleanup(func() { slog.SetDefault(prev) })

			_, ownerID := w.signupUser(t, strings.ReplaceAll(tc.name, " ", "-")+"@example.com")
			org, err := w.svc.CreateOrgWithOwner(ctx, "loudrate", ownerID)
			if err != nil {
				t.Fatal(err)
			}
			orgRow, _ := w.svc.GetOrg(ctx, org.ID)
			prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
			if err != nil {
				t.Fatal(err)
			}
			t0 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
			period := metering.Period(t0)
			for _, e := range []struct {
				edge string
				at   time.Time
			}{{"open", t0}, {"close", t0.Add(time.Hour)}} {
				if _, err := w.pool.Exec(ctx,
					`insert into usage_events (id, dedupe_key, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
					 values ('use_'||substr(md5(random()::text),1,12), 'seed_'||substr(md5(random()::text),1,16), $1, $2, $3, 'svc_bad', 'service_span', $4, 'postgres', $5, $6)`,
					org.ID, prj.ID, env.ID, e.edge, tc.rate, e.at); err != nil {
					t.Fatal(err)
				}
			}
			if err := metering.NewEmitter(store.New(w.pool)).Rollup(ctx, org.ID, period, t0.AddDate(0, 1, 0)); err == nil {
				t.Fatal("the rollup accepted an unrepresentable rate")
			}
			logged := buf.String()
			if !strings.Contains(logged, "ROLLUP OVERFLOW") || !strings.Contains(logged, org.ID) {
				t.Errorf("the accumulation failure was SILENT — an operator is told nothing:\n%s", logged)
			}
		})
	}
}
