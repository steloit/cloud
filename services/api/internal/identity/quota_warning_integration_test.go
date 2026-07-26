package identity_test

// US-11.2 — the 80% warning routes end-to-end: a member gets a bell notification
// (and email if on) carrying the meter, the percent, and the overage price.

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/billing"
	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/notify"
	"github.com/steloit/cloud/services/api/internal/quota"
)

func TestQuotaWarningRoutesToBell(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, _ := w.signupUser(t, "qw-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"qwco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	// QA scenario 2 (canon-pinned Business egress): 87/100 GB. The overage rate
	// is SOURCED from the billing table (plans.json), never retyped — a second
	// copy of a pricing constant is a bug (billing pack $note).
	tbl, err := billing.Load()
	if err != nil {
		t.Fatal(err)
	}
	rate := int64(tbl.Overage.EgressCentsPerGB)
	msg, warn := quota.WarnMessage("egress", 100, 87, rate)
	if !warn {
		t.Fatal("87/100 must warn")
	}
	// pin the clock to noon UTC (outside quiet hours) so the EMAIL route is
	// deterministic — the notify email path is quiet-hours-gated and flakes by
	// wall clock without this (the #277 lesson).
	noonUTC := func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) }
	fake := &recordingSender{}
	router := notify.NewRouter(store.New(w.pool), w.kek).WithEmail(fake).WithClock(noonUTC)
	if err := identity.EmitQuotaWarning(ctx, router, org.Id, msg); err != nil {
		t.Fatalf("emit: %v", err)
	}

	// the owner's BELL carries the warning with the math (percent + the price).
	_, lb := w.get(t, "/v1/me/notifications", ownerCk)
	if !strings.Contains(lb, "egress at 87%") {
		t.Fatalf("bell missing the 80%% warning: %s", lb)
	}
	priceStr := "$0.0" + strconv.FormatInt(rate, 10) // 9 -> $0.09 (rate is <10¢/GB in canon)
	if !strings.Contains(lb, priceStr) {
		t.Fatalf("warning did not show the overage math (%s): %s", priceStr, lb)
	}
	// the EMAIL route ALSO fired (banner+bell+EMAIL), carrying the same math.
	if fake.count() != 1 {
		t.Fatalf("the 80%% warning must route to email too, got %d mails", fake.count())
	}
}
