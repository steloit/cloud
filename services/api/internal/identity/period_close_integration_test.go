package identity_test

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/invoice"
	"github.com/steloit/cloud/services/api/internal/metering"
)

// closeWorld is one org with a project, plus the two services a period close
// needs. Every test here plants raw span edges, because the point is what happens
// to accounting when edges arrive at awkward times.
type closeWorld struct {
	*world
	org, prj, env string
	em            *metering.Emitter
	inv           *invoice.Service
	q             *store.Queries
}

func newCloseWorld(t *testing.T, email, name string) *closeWorld {
	t.Helper()
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	_, ownerID := w.signupUser(t, email)
	org, err := w.svc.CreateOrgWithOwner(ctx, name, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	orgRow, _ := w.svc.GetOrg(ctx, org.ID)
	prj, env, err := w.prov.CreateProject(ctx, orgRow, "shop", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := billing.Load()
	if err != nil {
		t.Fatal(err)
	}
	q := store.New(w.pool)
	// A FIXED CLOCK, well past every period these tests use. `Close` now refuses a
	// period that has not ended (closing early silently turns the rest of the
	// month into "late usage"), so the wall clock would make these tests pass or
	// fail depending on the date they are run — which is exactly the kind of
	// accidental dependency a billing suite must not have.
	clock := func() time.Time { return time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC) }
	return &closeWorld{world: w, org: org.ID, prj: prj.ID, env: env.ID,
		em: metering.NewEmitter(q), inv: invoice.NewService(w.pool, plans).WithClock(clock), q: q}
}

func (c *closeWorld) plant(t *testing.T, svc, edge string, at time.Time) {
	t.Helper()
	if _, err := c.pool.Exec(context.Background(),
		`insert into usage_events (id, dedupe_key, org_id, project_id, env_id, service_id, meter, edge, product, rate_cents, at)
		 values ('use_'||substr(md5(random()::text),1,12), 'seed_'||substr(md5(random()::text),1,16), $1, $2, $3, $4, 'service_span', $5, 'postgres', 2400, $6)`,
		c.org, c.prj, c.env, svc, edge, at); err != nil {
		t.Fatal(err)
	}
}

// carried is every unapplied CHARGE, whatever its origin.
func (c *closeWorld) carried(t *testing.T) []store.UsageCarryForward {
	t.Helper()
	rows, err := c.q.UnappliedCarryForward(context.Background(),
		store.UnappliedCarryForwardParams{OrgID: c.org, OriginPeriod: "2099-01"})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func (c *closeWorld) used(t *testing.T, period string) int64 {
	t.Helper()
	var n int64
	if err := c.pool.QueryRow(context.Background(),
		`select coalesce(sum(used),0) from quota_usage where org_id=$1 and period=$2`, c.org, period).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// THE DEFECT, pinned. Measured before the fix:
//
//	closed: invoice total=8640000, rollup used=3600s
//	after a late edge + one GET /usage: rollup used=14400s, invoice total=8640000
//
// 10800 seconds billed by nobody, and two customer-facing surfaces permanently
// disagreeing. Not a race — GET /orgs/{org}/usage takes a caller-supplied month
// and calls Rollup on the read path.
func TestAClosedPeriodCannotBeMovedByALateEvent(t *testing.T) {
	c := newCloseWorld(t, "close-late@example.com", "closelate")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	if err := c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	frozen := c.used(t, "2026-07")
	inv, err := c.inv.Close(ctx, c.org, "2026-07")
	if err != nil {
		t.Fatal(err)
	}

	c.plant(t, "svc_late", "open", t0.Add(2*time.Hour))
	c.plant(t, "svc_late", "close", t0.Add(5*time.Hour))
	if err := c.em.Rollup(ctx, c.org, "2026-07", time.Now()); err != nil {
		t.Fatalf("a rollup of a closed period must not ERROR — GET /usage of any past "+
			"month would become a permanent 500 with a remediation that cannot succeed: %v", err)
	}

	if got := c.used(t, "2026-07"); got != frozen {
		t.Fatalf("the closed rollup moved %ds -> %ds; the invoice its total came from cannot follow", frozen, got)
	}
	inv2, _ := c.inv.Close(ctx, c.org, "2026-07")
	if inv2.TotalCents != inv.TotalCents {
		t.Fatalf("the frozen invoice changed: %d -> %d", inv.TotalCents, inv2.TotalCents)
	}

	// AND THE LATE USAGE IS NOT LOST — carrying it forward is only better than
	// dropping it if something eventually bills it.
	cf, err := c.q.UnappliedCarryForward(ctx, store.UnappliedCarryForwardParams{OrgID: c.org, OriginPeriod: "2099-01"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cf) != 1 || cf[0].Used != 10800 {
		t.Fatalf("late usage was not carried forward: %+v — dropping it is invisible under-billing", cf)
	}
	if cf[0].OriginPeriod != "2026-07" {
		t.Fatalf("carry-forward lost which period the usage belongs to: %q", cf[0].OriginPeriod)
	}
}

// The database refuses even when the application does not ask.
func TestAClosedPeriodIsFrozenByTheDatabaseNotByTheCaller(t *testing.T) {
	c := newCloseWorld(t, "close-db@example.com", "closedb")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	if err := c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.inv.Close(ctx, c.org, "2026-07"); err != nil {
		t.Fatal(err)
	}

	// A support script, a backfill, a future query that forgets to check — the
	// enforcement must not depend on any of them remembering.
	_, err := c.pool.Exec(ctx,
		`update quota_usage set used = 999999 where org_id=$1 and period='2026-07'`, c.org)
	if err == nil {
		t.Fatal("a direct UPDATE rewrote a closed period — the freeze is caller discipline, not enforcement")
	}
	if !strings.Contains(err.Error(), "is frozen") {
		t.Fatalf("refused, but not by the close guard: %v", err)
	}
	_, err = c.pool.Exec(ctx, `delete from quota_usage where org_id=$1 and period='2026-07'`, c.org)
	if err == nil {
		t.Fatal("a closed period could be DELETED")
	}
}

// An OPEN period must still be freely recomputable, or recompute-on-read breaks.
func TestAnOpenPeriodIsStillRecomputable(t *testing.T) {
	c := newCloseWorld(t, "close-open@example.com", "closeopen")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	if err := c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	first := c.used(t, "2026-07")
	c.plant(t, "svc_b", "open", t0.Add(2*time.Hour))
	c.plant(t, "svc_b", "close", t0.Add(3*time.Hour))
	if err := c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	if got := c.used(t, "2026-07"); got <= first {
		t.Fatalf("an OPEN period stopped accumulating: %ds -> %ds", first, got)
	}
	if cf, _ := c.q.UnappliedCarryForward(ctx, store.UnappliedCarryForwardParams{OrgID: c.org, OriginPeriod: "2099-01"}); len(cf) != 0 {
		t.Fatalf("an open period produced a carry-forward: %+v", cf)
	}
}

// Carried usage is BILLED on the next close, once, with its origin named.
func TestCarriedUsageIsBilledOnceOnTheNextInvoice(t *testing.T) {
	c := newCloseWorld(t, "close-carry@example.com", "closecarry")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0))
	if _, err := c.inv.Close(ctx, c.org, "2026-07"); err != nil {
		t.Fatal(err)
	}
	c.plant(t, "svc_late", "open", t0.Add(2*time.Hour))
	c.plant(t, "svc_late", "close", t0.Add(5*time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", time.Now())

	aug, err := c.inv.Close(ctx, c.org, "2026-08")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aug.Lines), "late usage from 2026-07") {
		t.Fatalf("the August invoice does not name where the late usage came from: %s", aug.Lines)
	}
	if !strings.Contains(string(aug.Lines), "carry:service_span_seconds:2026-07") {
		t.Fatalf("the carried line has no usage_ref back to its origin: %s", aug.Lines)
	}

	// ONCE — asserted on the AMOUNT, not on containment. Review mutation-verified
	// that duplicating `carried` inside Close (every carry billed twice on the SAME
	// invoice) left the whole suite green, because strings.Contains cannot see a
	// duplicate. The total is the only thing that can.
	if n := strings.Count(string(aug.Lines), "carry:service_span_seconds:2026-07"); n != 1 {
		t.Fatalf("the carried line appears %d times on one invoice, want 1: %s", n, aug.Lines)
	}
	plans, _ := billing.Load()
	fee, _ := plans.PlanFeeCents("free")
	wantTotal := int64(fee) + 25920000 // plan fee + the 3h carried at 2400c/mo, cent-seconds
	if aug.TotalCents != wantTotal {
		t.Fatalf("August total is %d, want %d (plan fee %d + carried 25920000) — a duplicated "+
			"carry-forward is invisible to a containment check and shows up only here",
			aug.TotalCents, wantTotal, fee)
	}
	left, err := c.q.UnappliedCarryForward(ctx, store.UnappliedCarryForwardParams{OrgID: c.org, OriginPeriod: "2099-01"})
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("carried usage stayed unapplied after being billed — it would bill again: %+v", left)
	}
	sep, err := c.inv.Close(ctx, c.org, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sep.Lines), "late usage from 2026-07") {
		t.Fatalf("the same late usage was billed twice: %s", sep.Lines)
	}
}

// Re-detecting the same shortfall must not compound it: the carried amount is a
// DELTA against a frozen number, so recomputing is idempotent by construction.
func TestRepeatedDetectionOfTheSameLateUsageDoesNotCompound(t *testing.T) {
	c := newCloseWorld(t, "close-idem@example.com", "closeidem")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0))
	_, _ = c.inv.Close(ctx, c.org, "2026-07")
	c.plant(t, "svc_late", "open", t0.Add(2*time.Hour))
	c.plant(t, "svc_late", "close", t0.Add(5*time.Hour))

	// Five GET /usage calls on the closed month.
	for range 5 {
		if err := c.em.Rollup(ctx, c.org, "2026-07", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	cf, err := c.q.UnappliedCarryForward(ctx, store.UnappliedCarryForwardParams{OrgID: c.org, OriginPeriod: "2099-01"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cf) != 1 {
		t.Fatalf("five recomputes produced %d carry-forward rows, want 1", len(cf))
	}
	if cf[0].Used != 10800 {
		t.Fatalf("the carried delta compounded to %ds across recomputes, want 10800", cf[0].Used)
	}
}

// An edge landing just BEFORE close is ordinary usage and must be billed in its
// own period, not carried. The boundary is the invoice, not the wall clock.
func TestAnEdgeThatLandsBeforeCloseIsBilledInItsOwnPeriod(t *testing.T) {
	c := newCloseWorld(t, "close-boundary@example.com", "closebound")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	c.plant(t, "svc_b", "open", t0.Add(2*time.Hour))
	c.plant(t, "svc_b", "close", t0.Add(3*time.Hour))
	if err := c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.inv.Close(ctx, c.org, "2026-07"); err != nil {
		t.Fatal(err)
	}
	if got := c.used(t, "2026-07"); got != 7200 {
		t.Fatalf("both spans should be in the period's own rollup: %ds, want 7200", got)
	}
	// THE ROLLUP AFTER CLOSE IS THE WHOLE POINT, and the first version of this
	// test omitted it — so it never reached carryForward at all. Mutation-verified
	// by review: disabling carry-forward entirely left this test PASS while the
	// three that own the behaviour went RED. A test that cannot reach the code it
	// is named for asserts nothing about it.
	if err := c.em.Rollup(ctx, c.org, "2026-07", time.Now()); err != nil {
		t.Fatal(err)
	}
	if cf := c.carried(t); len(cf) != 0 {
		t.Fatalf("usage that arrived BEFORE close was carried forward: %+v — the boundary is "+
			"the invoice, and everything here was inside it", cf)
	}
}

// A SECOND late arrival must be carried too. The first design lost it: the ledger
// was keyed (org, meter, origin) and upserted `WHERE applied_period IS NULL`, so
// once the first carry was billed the conflict clause failed its WHERE, the
// :exec discarded the row count, and nothing errored. Measured by review: 14,400s
// on no invoice, ever — the exact Stripe failure carry-forward was chosen to avoid.
func TestASecondLateArrivalIsAlsoCarried(t *testing.T) {
	c := newCloseWorld(t, "close-second@example.com", "closesecond")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0))
	if _, err := c.inv.Close(ctx, c.org, "2026-07"); err != nil {
		t.Fatal(err)
	}

	// First late pair → carried and billed on August.
	c.plant(t, "svc_l1", "open", t0.Add(2*time.Hour))
	c.plant(t, "svc_l1", "close", t0.Add(5*time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", time.Now())
	if _, err := c.inv.Close(ctx, c.org, "2026-08"); err != nil {
		t.Fatal(err)
	}
	if left := c.carried(t); len(left) != 0 {
		t.Fatalf("the first carry was not billed: %+v", left)
	}

	// SECOND late pair, after the first was already billed.
	c.plant(t, "svc_l2", "open", t0.Add(6*time.Hour))
	c.plant(t, "svc_l2", "close", t0.Add(7*time.Hour))
	if err := c.em.Rollup(ctx, c.org, "2026-07", time.Now()); err != nil {
		t.Fatal(err)
	}
	second := c.carried(t)
	if len(second) != 1 {
		t.Fatalf("a SECOND late arrival produced %d carry rows, want 1 — it was silently "+
			"discarded, which is invisible under-billing", len(second))
	}
	// It must be the NEW hour only — not the first shortfall re-carried.
	if second[0].Used != 3600 {
		t.Fatalf("the second remainder is %ds, want 3600 (the new hour only). The remainder "+
			"must subtract what is ALREADY carried, or the first shortfall bills twice",
			second[0].Used)
	}
	sep, err := c.inv.Close(ctx, c.org, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sep.Lines), "late usage from 2026-07") {
		t.Fatalf("the second late arrival reached no invoice: %s", sep.Lines)
	}
}

// A MIXED-SIGN remainder must not poison every future invoice. The first design's
// guard was `deltaSeconds <= 0 && deltaCents <= 0`, so a remainder that was
// positive on one axis and negative on the other wrote rate_cents < 0; Close then
// refused to total it — permanently, for that org, with no query able to repair
// the row. Review reproduced it with ordinary late edges.
func TestAnOverBillIsRecordedAsACreditAndNeverPoisonsAnInvoice(t *testing.T) {
	c := newCloseWorld(t, "close-credit@example.com", "closecredit")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	// A span left OPEN at the period end: the rollup credits it to the period end.
	c.plant(t, "svc_open", "open", t0)
	_ = c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0))
	frozen := c.used(t, "2026-07")
	if _, err := c.inv.Close(ctx, c.org, "2026-07"); err != nil {
		t.Fatal(err)
	}

	// The late `close` arrives: the span actually stopped an hour in, so the
	// recompute is SMALLER than the frozen figure. This is the commonest late shape.
	c.plant(t, "svc_open", "close", t0.Add(time.Hour))
	if err := c.em.Rollup(ctx, c.org, "2026-07", time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := c.used(t, "2026-07"); got != frozen {
		t.Fatalf("the closed rollup moved: %d -> %d", frozen, got)
	}

	// It is recorded as a CREDIT, not discarded — over-billing is the customer's
	// money and the first design returned nil with no row and no log.
	credits, err := c.q.UnappliedCredits(ctx, c.org)
	if err != nil {
		t.Fatal(err)
	}
	if len(credits) != 1 {
		t.Fatalf("an over-bill produced %d credit rows, want 1 — under-billing was carried "+
			"with a WARN while over-billing vanished, and that asymmetry was the defect", len(credits))
	}
	if credits[0].RateCents >= 0 {
		t.Fatalf("the credit is not negative: %d", credits[0].RateCents)
	}

	// AND every later invoice still closes. The old behaviour was a poison pill.
	for _, p := range []string{"2026-08", "2026-09", "2026-10"} {
		if _, err := c.inv.Close(ctx, c.org, p); err != nil {
			t.Fatalf("close(%s) failed after an over-bill — one bad ledger row poisoned every "+
				"future invoice for this org: %v", p, err)
		}
	}
	// A credit is NOT auto-applied: refunding is commercial, surfacing is not.
	if left, _ := c.q.UnappliedCredits(ctx, c.org); len(left) != 1 {
		t.Fatalf("the credit was auto-applied; issuing a refund is a commercial decision")
	}
}

// An invoice dated June must never carry July's usage.
func TestACarryForwardNeverLandsOnAnEarlierInvoice(t *testing.T) {
	c := newCloseWorld(t, "close-order@example.com", "closeorder")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0))
	_, _ = c.inv.Close(ctx, c.org, "2026-07")
	c.plant(t, "svc_late", "open", t0.Add(2*time.Hour))
	c.plant(t, "svc_late", "close", t0.Add(5*time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", time.Now())

	june, err := c.inv.Close(ctx, c.org, "2026-06")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(june.Lines), "late usage from 2026-07") {
		t.Fatalf("a JUNE invoice carried JULY's usage — not defensible to a customer or an "+
			"auditor: %s", june.Lines)
	}
	if left := c.carried(t); len(left) != 1 {
		t.Fatalf("the carry was consumed by an earlier invoice: %+v", left)
	}
}

// A period that has not ended cannot be closed: closing early silently converts
// the rest of the month into "late usage".
func TestAPeriodThatHasNotEndedCannotBeClosed(t *testing.T) {
	c := newCloseWorld(t, "close-early@example.com", "closeearly")
	// The injected clock is 2027-01-15, so 2027-01 is still running.
	_, err := c.inv.Close(context.Background(), c.org, "2027-01")
	if err == nil {
		t.Fatal("a period that has not ended was closed — the rest of the month would become " +
			"'late usage' and be billed on a later invoice")
	}
	if !strings.Contains(err.Error(), "has not ended") {
		t.Fatalf("refused, but not for the right reason: %v", err)
	}
}

// The freeze covers INSERT and TRUNCATE, not only UPDATE/DELETE.
func TestAClosedPeriodCannotGainANewMeterRow(t *testing.T) {
	c := newCloseWorld(t, "close-insert@example.com", "closeinsert")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0))
	if _, err := c.inv.Close(ctx, c.org, "2026-07"); err != nil {
		t.Fatal(err)
	}

	// A meter with NO row at close time could still gain one afterwards — a rollup
	// the invoice never saw. The first trigger was BEFORE UPDATE OR DELETE only.
	_, err := c.pool.Exec(ctx,
		`insert into quota_usage (org_id, meter, period, used, rate_cents)
		 values ($1, 'egress_bytes', '2026-07', 999, 500000)`, c.org)
	if err == nil {
		t.Fatal("a CLOSED period gained a brand-new meter row — the freeze covered two of the " +
			"three ways a rollup can change")
	}
	if !strings.Contains(err.Error(), "is frozen") {
		t.Fatalf("refused, but not by the close guard: %v", err)
	}

	// A row trigger never fires for TRUNCATE, so the whole table could be emptied
	// out from under every closed invoice.
	if _, err := c.pool.Exec(ctx, `truncate quota_usage`); err == nil {
		t.Fatal("quota_usage was TRUNCATEd — every closed invoice lost its input")
	}
}

// THE SIGN MATRIX, stated exhaustively rather than left to whichever branch runs.
// The first design's guard was `deltaSeconds <= 0 && deltaCents <= 0`, which is
// only correct when both axes agree — and they need not.
func TestEverySignCombinationOfARemainderIsHandled(t *testing.T) {
	for _, tc := range []struct {
		name string
		frozenSec, frozenCents,
		recompSec, recompCents int64
		wantRows int
		wantKind string
	}{
		{"zero remainder — no row at all", 3600, 100, 3600, 100, 0, ""},
		{"positive seconds, positive money — a charge", 3600, 100, 7200, 200, 1, "charge"},
		{"negative seconds, negative money — a credit", 7200, 200, 3600, 100, 1, "credit"},
		{"positive seconds, NEGATIVE money — a credit, never a poisoned charge", 3600, 500, 7200, 100, 1, "credit"},
		{"negative seconds, POSITIVE money — a charge", 7200, 100, 3600, 500, 1, "charge"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCloseWorld(t, "sign-"+strings.ReplaceAll(tc.name[:8], " ", "")+"@example.com", "sign"+tc.name[:4])
			ctx := context.Background()
			// Plant the frozen figure directly, then close, then drive carryForward
			// with a recompute of the given shape. Going through spans cannot
			// produce every sign combination, and the point here is the arithmetic.
			if _, err := c.pool.Exec(ctx,
				`insert into quota_usage (org_id, meter, period, used, rate_cents)
				 values ($1,'service_span_seconds','2026-07',$2,$3)`, c.org, tc.frozenSec, tc.frozenCents); err != nil {
				t.Fatal(err)
			}
			if _, err := c.inv.Close(ctx, c.org, "2026-07"); err != nil {
				t.Fatal(err)
			}
			if err := c.em.CarryForwardForTest(ctx, c.org, "2026-07", tc.recompSec, tc.recompCents); err != nil {
				t.Fatalf("carryForward refused a %s remainder: %v", tc.name, err)
			}
			var rows int
			var kind string
			_ = c.pool.QueryRow(ctx,
				`select count(*), coalesce(max(kind),'') from usage_carry_forward where org_id=$1`, c.org).Scan(&rows, &kind)
			if rows != tc.wantRows {
				t.Fatalf("%s produced %d ledger rows, want %d", tc.name, rows, tc.wantRows)
			}
			if tc.wantRows > 0 && kind != tc.wantKind {
				t.Fatalf("%s recorded kind %q, want %q", tc.name, kind, tc.wantKind)
			}
			// AND every later invoice must still close. A mixed-sign remainder that
			// wrote a negative CHARGE made Close refuse to total, permanently.
			for _, p := range []string{"2026-08", "2026-09"} {
				if _, err := c.inv.Close(ctx, c.org, p); err != nil {
					t.Fatalf("close(%s) failed after a %s remainder — one ledger row poisoned "+
						"every future invoice: %v", p, tc.name, err)
				}
			}
		})
	}
}

// A CARRY ALREADY TAKEN BY ANOTHER CLOSE MUST NOT BE BILLED AGAIN.
//
// This is the property that makes concurrent closes safe, and it is asserted
// DETERMINISTICALLY rather than by racing goroutines. A first attempt did spawn
// two closes and count lines — and mutation testing showed it did not
// discriminate: the goroutines serialised, so the second close read no carries and
// the mutation survived. A test whose outcome depends on scheduling proves
// nothing about the mechanism.
//
// Here the claim is taken out from under `Close` first, exactly as a concurrent
// close would, and the invoice must then contain no carried line.
func TestACarryAlreadyClaimedElsewhereIsNotBilledAgain(t *testing.T) {
	c := newCloseWorld(t, "close-claimed@example.com", "closeclaimed")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0))
	if _, err := c.inv.Close(ctx, c.org, "2026-07"); err != nil {
		t.Fatal(err)
	}
	c.plant(t, "svc_late", "open", t0.Add(2*time.Hour))
	c.plant(t, "svc_late", "close", t0.Add(5*time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", time.Now())

	pending := c.carried(t)
	if len(pending) != 1 {
		t.Fatalf("expected one carry to claim, got %d", len(pending))
	}
	// Another close takes it first.
	taken, err := c.q.ClaimCarryForward(ctx, store.ClaimCarryForwardParams{
		OrgID: c.org, AppliedPeriod: pgtype.Text{String: "2026-08", Valid: true},
		Column3: []string{pending[0].ID},
	})
	if err != nil || len(taken) != 1 {
		t.Fatalf("the first claim did not take the row: %d rows, %v", len(taken), err)
	}
	// A SECOND claim of the same id must return nothing — that atomicity is what
	// makes two closes safe.
	again, err := c.q.ClaimCarryForward(ctx, store.ClaimCarryForwardParams{
		OrgID: c.org, AppliedPeriod: pgtype.Text{String: "2026-09", Valid: true},
		Column3: []string{pending[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("the same carry was claimed twice (%d rows) — two closes would both bill it", len(again))
	}

	// And Close must bill only what IT claimed, not what it merely read.
	sep, err := c.inv.Close(ctx, c.org, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sep.Lines), "carry:service_span_seconds:2026-07") {
		t.Fatalf("an invoice billed a carry it did not claim — Close is billing what it READ, "+
			"not what it TOOK: %s", sep.Lines)
	}
}

// DELETING AN INVOICE MUST NOT UN-FREEZE ITS PERIOD. The freeze is defined by the
// invoice's existence, so removing the evidence of the boundary removed the
// boundary — and orphaned every carry-forward referencing it as origin.
func TestAnInvoiceCannotBeDeletedOrRewritten(t *testing.T) {
	c := newCloseWorld(t, "close-inv@example.com", "closeinv")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0))
	if _, err := c.inv.Close(ctx, c.org, "2026-07"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.pool.Exec(ctx, `delete from invoices where org_id=$1 and period='2026-07'`, c.org); err == nil {
		t.Fatal("an invoice was DELETED — its period is mutable again and every carry-forward " +
			"referencing it as origin is orphaned")
	}
	if _, err := c.pool.Exec(ctx,
		`update invoices set total_cents=1 where org_id=$1 and period='2026-07'`, c.org); err == nil {
		t.Fatal("a closed invoice's total was rewritten")
	}
	if _, err := c.pool.Exec(ctx, `truncate invoices`); err == nil {
		t.Fatal("invoices was TRUNCATEd — every closed period became mutable")
	}
	// The lifecycle status is deliberately still mutable.
	if _, err := c.pool.Exec(ctx,
		`update invoices set status='paid' where org_id=$1 and period='2026-07'`, c.org); err != nil {
		t.Fatalf("status must remain changeable (open -> paid/void): %v", err)
	}
}

// A METER carryForward CANNOT ACCOUNT FOR MUST BE LOUD, not skipped. Carrying one
// meter while silently dropping another is the invisible under-billing this task
// exists to prevent, and `quota_usage.meter` already names four others.
func TestAnUnaccountableMeterInAClosedPeriodIsRefusedNotSkipped(t *testing.T) {
	c := newCloseWorld(t, "close-meter@example.com", "closemeter")
	ctx := context.Background()
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	c.plant(t, "svc_a", "open", t0)
	c.plant(t, "svc_a", "close", t0.Add(time.Hour))
	_ = c.em.Rollup(ctx, c.org, "2026-07", t0.AddDate(0, 1, 0))
	// A second meter in the same period, BEFORE close so the freeze allows it.
	if _, err := c.pool.Exec(ctx,
		`insert into quota_usage (org_id, meter, period, used, rate_cents)
		 values ($1,'egress_bytes','2026-07',5,500)`, c.org); err != nil {
		t.Fatal(err)
	}
	if _, err := c.inv.Close(ctx, c.org, "2026-07"); err != nil {
		t.Fatal(err)
	}
	c.plant(t, "svc_late", "open", t0.Add(2*time.Hour))
	c.plant(t, "svc_late", "close", t0.Add(3*time.Hour))

	err := c.em.Rollup(ctx, c.org, "2026-07", time.Now())
	if err == nil {
		t.Fatal("a closed period holding a meter carryForward cannot account for was rolled up " +
			"silently — late usage on that meter would vanish")
	}
	if !strings.Contains(err.Error(), "cannot account for") {
		t.Fatalf("refused, but not for the right reason: %v", err)
	}
}

// BILLING MUST READ WHAT WAS CLAIMED, NOT WHAT WAS READ.
//
// The discriminating case is a carry that is READ but deliberately NOT claimed: a
// zero-amount row is excluded from the claim (there is nothing to bill), so if
// Close loops over `carried` instead of `claimed` it emits a phantom zero line and
// — worse — the row is never claimed, so it stays unapplied forever while having
// appeared on an invoice.
//
// A previous version of this test claimed the row externally first, which made
// `carried` EMPTY and so could not tell the two loops apart at all. Mutation
// testing caught that; the zero-amount row is what actually discriminates.
func TestCloseBillsWhatItClaimedNotWhatItRead(t *testing.T) {
	c := newCloseWorld(t, "close-claimread@example.com", "closeclaimread")
	ctx := context.Background()

	// A carry whose money is zero but whose seconds are not — reachable whenever a
	// span's duration changes at a zero rate.
	if _, err := c.pool.Exec(ctx,
		`insert into usage_carry_forward (id, org_id, meter, origin_period, used, rate_cents, kind)
		 values ('cf_zero', $1, 'service_span_seconds', '2026-06', 3600, 0, 'charge')`, c.org); err != nil {
		t.Fatal(err)
	}
	inv, err := c.inv.Close(ctx, c.org, "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(inv.Lines), "carry:service_span_seconds:2026-06") {
		t.Fatalf("a zero-amount carry was billed as a line — Close is iterating what it READ "+
			"rather than what it CLAIMED: %s", inv.Lines)
	}
	// And it must remain unapplied: it was never claimed, so nothing may say it was.
	var applied *string
	if err := c.pool.QueryRow(ctx,
		`select applied_period from usage_carry_forward where id='cf_zero'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != nil {
		t.Fatalf("an unclaimed carry was marked applied to %q — money marked billed that never "+
			"reached a line", *applied)
	}
}
