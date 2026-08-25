package identity_test

import (
	"context"
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
	return &closeWorld{world: w, org: org.ID, prj: prj.ID, env: env.ID,
		em: metering.NewEmitter(q), inv: invoice.NewService(q, plans), q: q}
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
	cf, err := c.q.UnappliedCarryForward(ctx, c.org)
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
	if cf, _ := c.q.UnappliedCarryForward(ctx, c.org); len(cf) != 0 {
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

	// Once. A second close must not bill it again.
	left, err := c.q.UnappliedCarryForward(ctx, c.org)
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
	cf, err := c.q.UnappliedCarryForward(ctx, c.org)
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
	if cf, _ := c.q.UnappliedCarryForward(ctx, c.org); len(cf) != 0 {
		t.Fatalf("usage that arrived before close was carried forward: %+v", cf)
	}
}
