package identity

// US-11.2 — the 80% quota warning (QA scenario 2): when a soft meter crosses
// 80% of its allowance, a warning fans to banner+bell+email with the MATH shown
// up front (the overage unit price), never discovered on the invoice. This
// composes quota.WarnMessage into a notify projection routed by the member's
// prefs (the same path lifecycle/deploy notifications take).

import (
	"context"
	"fmt"

	"github.com/steloit/cloud/services/api/internal/notify"
	"github.com/steloit/cloud/services/api/internal/quota"
)

// EmitQuotaWarning routes an 80% quota warning through the notify router.
// Idempotency (one warning per meter per period) is the caller's concern (the
// rollup hook) — this is the composition + routing.
func EmitQuotaWarning(ctx context.Context, router *notify.Router, orgID string, w quota.QuotaWarning) error {
	title := fmt.Sprintf("%s at %d%% — %d of %d used", w.Meter, w.PercentUsed, w.Used, w.Allowance)
	body := fmt.Sprintf(
		"You've used %d%% of your %s allowance (%d of %d). It keeps working; beyond the allowance %s bills at %s per unit — warned now at 80%%, never discovered on the invoice.",
		w.PercentUsed, w.Meter, w.Used, w.Allowance, w.Meter, dollarsPerUnit(w.OverageRate),
	)
	return router.Notify(ctx, notify.NotifyInput{
		OrgID: orgID, Kind: "lifecycle", Title: title, Body: body, Link: "/billing/usage",
	})
}

// dollarsPerUnit renders an integer-cents unit price (ADR-025) as $X.YZ.
func dollarsPerUnit(cents int64) string {
	neg := ""
	if cents < 0 {
		neg, cents = "-", -cents
	}
	return fmt.Sprintf("%s$%d.%02d", neg, cents/100, cents%100)
}
