package identity_test

// T11.3: the invoice generator against a real DB — the meter frozen into an
// invoice whose lines carry usage_refs, integer-cent totals, and an idempotent
// monthly close.

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/invoice"
)

func TestInvoiceGenerator(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)
	plans, err := billing.Load()
	if err != nil {
		t.Fatal(err)
	}

	_, ownerID := w.signupUser(t, "inv@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "invco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, "update orgs set plan='business' where id=$1", org.ID); err != nil {
		t.Fatal(err)
	}
	period := "2026-07"
	// seed the meter (the T6.3 rollup output the invoice freezes)
	for _, m := range []struct {
		meter string
		rate  int64
	}{{"service_span_seconds", 20800}, {"egress_bytes", 162}} {
		if _, err := w.pool.Exec(ctx,
			"insert into quota_usage (org_id, meter, period, used, rate_cents) values ($1,$2,$3,100,$4)",
			org.ID, m.meter, period, m.rate); err != nil {
			t.Fatal(err)
		}
	}

	gen := invoice.NewService(q, plans)

	// --- close: plan fee + metered lines, integer cents ---------------------
	inv, err := gen.Close(ctx, org.ID, period)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if inv.Status != "open" || !strings.HasPrefix(inv.ID, "inv_") {
		t.Fatalf("invoice shape: %+v", inv)
	}
	// total = business plan fee 9900 + 20800 + 162
	if inv.TotalCents != 9900+20800+162 {
		t.Fatalf("total = %d, want %d", inv.TotalCents, 9900+20800+162)
	}
	// every line carries a usage_ref; the plan fee line is present
	var lines []struct {
		Description string `json:"description"`
		Cents       int64  `json:"cents"`
		UsageRef    string `json:"usage_ref"`
	}
	_ = json.Unmarshal(inv.Lines, &lines)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (plan + 2 meters), got %d", len(lines))
	}
	var sum int64
	planLine := false
	for _, l := range lines {
		if l.UsageRef == "" {
			t.Fatalf("line missing usage_ref: %+v", l)
		}
		// The unified line rule (taxonomy §74, US-11.6): EVERY line is either a
		// stated plan allowance (plan:*) or a metered quantity (meter:*) — nothing
		// else exists. A line that is neither is an ungrammatical charge.
		if !strings.HasPrefix(l.UsageRef, "plan:") && !strings.HasPrefix(l.UsageRef, "meter:") {
			t.Fatalf("line %q violates the §74 grammar (not plan:* or meter:*): usage_ref=%q", l.Description, l.UsageRef)
		}
		// non-negative line (int64 is already integer per ADR-025; a negative meter
		// rate would be a pricing bug, never a valid line).
		if l.Cents < 0 {
			t.Fatalf("negative line cents: %+v", l)
		}
		if strings.Contains(l.Description, "business plan") {
			planLine = true
			if l.Cents != 9900 {
				t.Fatalf("plan fee line = %d, want 9900", l.Cents)
			}
		}
		sum += l.Cents
	}
	if !planLine {
		t.Fatal("no plan-fee line")
	}
	if sum != inv.TotalCents {
		t.Fatalf("Σ lines %d ≠ total %d", sum, inv.TotalCents)
	}

	// --- idempotent monthly close: re-close returns the SAME invoice --------
	inv2, err := gen.Close(ctx, org.ID, period)
	if err != nil {
		t.Fatalf("re-close: %v", err)
	}
	if inv2.ID != inv.ID || inv2.TotalCents != inv.TotalCents {
		t.Fatalf("re-close produced a different invoice: %s vs %s", inv2.ID, inv.ID)
	}
	var count int
	_ = w.pool.QueryRow(ctx, "select count(*) from invoices where org_id=$1 and period=$2", org.ID, period).Scan(&count)
	if count != 1 {
		t.Fatalf("monthly close not idempotent: %d invoices for the period", count)
	}

	// --- the freeze is real: usage changing after close does NOT re-price -----
	if _, err := w.pool.Exec(ctx, "update quota_usage set rate_cents=999999 where org_id=$1 and meter='egress_bytes'", org.ID); err != nil {
		t.Fatal(err)
	}
	inv3, err := gen.Close(ctx, org.ID, period)
	if err != nil {
		t.Fatal(err)
	}
	if inv3.TotalCents != inv.TotalCents {
		t.Fatalf("re-close after a usage change re-priced the frozen invoice: %d vs %d", inv3.TotalCents, inv.TotalCents)
	}

	// --- the B3 read surface lists it --------------------------------------
	ck, _ := w.loginCookie(t, "inv@example.com")
	resp, body := w.get(t, "/v1/orgs/"+org.ID+"/billing/invoices", ck)
	if resp.StatusCode != 200 || !strings.Contains(body, inv.ID) || !strings.Contains(body, "usage_ref") {
		t.Fatalf("listInvoices: %d %s", resp.StatusCode, body)
	}
}

// AN INVOICE NEVER FREEZES A TOTAL ITS LINES DO NOT SUM TO.
//
// `quota_usage.rate_cents` is `bigint NOT NULL` with no CHECK, so a degenerate
// row is storable — and `Rollup` is not the only writer of that table, it only
// ever writes `service_span_seconds`. Measured on an earlier revision of this
// branch, which summed the rows through the SATURATING SpendToDate: one row at
// -500 froze TotalCents at 9223372036854775807 against Σlines of 30200. Because
// UpsertInvoiceForPeriod is ON CONFLICT DO NOTHING, that $92-quadrillion invoice
// was permanent and re-closing returned it unchanged.
//
// Saturation is right for a figure that must be RENDERED and catastrophic for one
// that is FROZEN. Close refuses instead.
func TestAnInvoiceNeverFreezesATotalItsLinesDoNotSumTo(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows map[string]int64
	}{
		{"a negative meter row", map[string]int64{"service_span_seconds": 20800, "egress_bytes": -500}},
		{"a row at the int64 ceiling", map[string]int64{"service_span_seconds": 20800, "egress_bytes": math.MaxInt64}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, time.Hour)
			ctx := context.Background()
			q := store.New(w.pool)
			plans, err := billing.Load()
			if err != nil {
				t.Fatal(err)
			}
			_, ownerID := w.signupUser(t, strings.ReplaceAll(tc.name, " ", "-")+"@example.com")
			org, err := w.svc.CreateOrgWithOwner(ctx, "invsum", ownerID)
			if err != nil {
				t.Fatal(err)
			}
			const period = "2026-07"
			for meter, rate := range tc.rows {
				if _, err := w.pool.Exec(ctx,
					`insert into quota_usage (org_id, meter, period, used, rate_cents) values ($1,$2,$3,100,$4)`,
					org.ID, meter, period, rate); err != nil {
					t.Fatal(err)
				}
			}

			inv, err := invoice.NewService(q, plans).Close(ctx, org.ID, period)
			if err != nil {
				// Refusing is the correct outcome; nothing must have been frozen.
				var n int
				if err := w.pool.QueryRow(ctx,
					`select count(*) from invoices where org_id=$1 and period=$2`, org.ID, period).Scan(&n); err != nil {
					t.Fatal(err)
				}
				if n != 0 {
					t.Errorf("Close refused but %d invoice row(s) were written anyway", n)
				}
				return
			}
			// If it DID close, the invariant must hold exactly.
			if inv.TotalCents == math.MaxInt64 {
				t.Fatalf("the frozen invoice total is MaxInt64 — a charge nobody can act on, and "+
					"ON CONFLICT DO NOTHING makes it permanent (lines: %s)", inv.Lines)
			}
			var lines []struct {
				Cents int64 `json:"cents"`
			}
			if err := json.Unmarshal(inv.Lines, &lines); err != nil {
				t.Fatal(err)
			}
			var sum int64
			for _, l := range lines {
				sum += l.Cents
			}
			if sum != inv.TotalCents {
				t.Errorf("Σ lines %d != total %d — the invoice's own invariant", sum, inv.TotalCents)
			}
		})
	}
}
