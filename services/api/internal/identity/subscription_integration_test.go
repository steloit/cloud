package identity_test

// T11.2: the subscription lifecycle state machine against a real DB, clock-warped
// (never real waits). Covers every status from the models.md enum and the
// Borealis B9–B12 flows (trial, dunning day-8, cancel/reactivate).

import (
	"context"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/subscription"
)

func TestSubscriptionLifecycle(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)

	_, ownerID := w.signupUser(t, "sub@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "subco", ownerID)
	if err != nil {
		t.Fatal(err)
	}

	// a warpable clock — the whole dunning timeline runs in-memory
	clock := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	svc := subscription.NewService(q, w.rec).WithClock(func() time.Time { return clock })

	// the org is born with an inert `current`/free subscription (T2.7)
	sub, _ := svc.Get(ctx, org.ID)
	if sub.Status != "current" || sub.Plan != "free" {
		t.Fatalf("inert sub: %s/%s", sub.Status, sub.Plan)
	}

	// --- trial (B9) --------------------------------------------------------
	sub, err = svc.StartTrial(ctx, org.ID, "pro", 14)
	if err != nil || sub.Status != "trial" || sub.Plan != "pro" || !sub.TrialEndsAt.Valid {
		t.Fatalf("start trial: %v %+v", err, sub)
	}

	// --- payment succeeds → current ---------------------------------------
	sub, err = svc.Activate(ctx, org.ID)
	if err != nil || sub.Status != "current" {
		t.Fatalf("activate: %v %s", err, sub.Status)
	}

	// --- payment fails → dunning track (B11 dunning) -----------------------
	sub, err = svc.FailPayment(ctx, org.ID)
	if err != nil || sub.Status != "grace" || !sub.DunningStartedAt.Valid || !sub.NextRetryAt.Valid {
		t.Fatalf("fail payment: %v %+v", err, sub)
	}

	// day 8: grace → provisioning_paused (billing pack: 7–21 paused)
	clock = clock.Add(8 * 24 * time.Hour)
	sub, err = svc.AdvanceLifecycle(ctx, org.ID)
	if err != nil || sub.Status != "provisioning_paused" {
		t.Fatalf("day 8 should pause provisioning: %v %s", err, sub.Status)
	}

	// day 22: provisioning_paused → suspended (state kept)
	clock = clock.Add(14 * 24 * time.Hour) // now day 22 from dunning start
	sub, err = svc.AdvanceLifecycle(ctx, org.ID)
	if err != nil || sub.Status != "suspended" {
		t.Fatalf("day 22 should suspend: %v %s", err, sub.Status)
	}

	// payment clears → current instantly, dunning cleared (billing pack)
	sub, err = svc.Activate(ctx, org.ID)
	if err != nil || sub.Status != "current" || sub.DunningStartedAt.Valid {
		t.Fatalf("recover: %v %+v", err, sub)
	}

	// --- cancel at anchor (B12) — resources keep running -------------------
	sub, err = svc.Cancel(ctx, org.ID)
	if err != nil || sub.Status != "cancelled_at_anchor" || !sub.PlanEndsAt.Valid {
		t.Fatalf("cancel: %v %+v", err, sub)
	}
	// reactivate before the anchor → current
	sub, err = svc.Reactivate(ctx, org.ID)
	if err != nil || sub.Status != "current" || sub.PlanEndsAt.Valid {
		t.Fatalf("reactivate: %v %+v", err, sub)
	}
	// cancel again, then let the anchor pass → plan drops to free, current
	sub, _ = svc.Cancel(ctx, org.ID)
	clock = sub.PlanEndsAt.Time.Add(time.Hour) // past the anchor
	sub, err = svc.AdvanceLifecycle(ctx, org.ID)
	if err != nil || sub.Status != "current" || sub.Plan != "free" {
		t.Fatalf("plan should end at anchor → free/current: %v %+v", err, sub)
	}

	// --- guards: illegal transitions are refused ---------------------------
	if _, err := svc.Reactivate(ctx, org.ID); err == nil {
		t.Fatal("reactivate from current should be refused")
	}
	if _, err := svc.FailPayment(ctx, org.ID); err != nil {
		t.Fatalf("fail payment from current should be allowed: %v", err)
	}
	if _, err := svc.StartTrial(ctx, org.ID, "pro", 14); err == nil {
		t.Fatal("starting a trial from grace should be refused")
	}

	// the spine recorded the lifecycle (billing events)
	var n int
	_ = w.pool.QueryRow(ctx, "select count(*) from events where org_id=$1 and action like 'subscription.%'", org.ID).Scan(&n)
	if n < 8 {
		t.Fatalf("expected the lifecycle on the spine, got %d subscription events", n)
	}
}
