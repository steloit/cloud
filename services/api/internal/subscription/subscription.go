// Package subscription is the M7 billing lifecycle state machine (T11.2). Status
// is the single master state — trial → current, the dunning track
// (current → grace → provisioning_paused → suspended), and cancel
// (current → cancelled_at_anchor → the plan ends at the anchor, resources keep
// running). The API's dunning{} and wind_down{} objects are DERIVED from status
// + the timestamps here, never a second source. Payment is external (T11.4,
// Stripe): its webhooks call the transitions; this package holds no provider
// concept. Time is injected so dunning/anchor progression is clock-warped in
// tests, never real waits (billing pack).
package subscription

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// The dunning timeline (billing pack, exact): day 0 fail → retries → day 7
// provisioning paused → day 21 suspend → day 90 deletion (deletion is a
// separate org sweep, not this state machine).
const (
	graceToPausedDays   = 7
	pausedToSuspendDays = 21
	retryIntervalHours  = 24
	defaultTrialDays    = 14
)

var (
	ErrNoSubscription = errors.New("subscription: not found")
	ErrBadTransition  = errors.New("subscription: transition not allowed from the current state")
	// ErrConcurrentModification: the row changed under us (CAS lost). The
	// caller's decision was stale; re-read. Benign for the idempotent worker.
	ErrConcurrentModification = errors.New("subscription: concurrently modified")
)

// Store is the persistence the machine needs (the sqlc queries satisfy it).
type Store interface {
	GetSubscription(ctx context.Context, orgID string) (store.Subscription, error)
	SetSubscriptionState(ctx context.Context, arg store.SetSubscriptionStateParams) (store.Subscription, error)
	ListSubscriptionsToAdvance(ctx context.Context) ([]string, error)
}

type Service struct {
	q   Store
	rec *events.Recorder
	now func() time.Time
}

func NewService(q Store, rec *events.Recorder) *Service {
	return &Service{q: q, rec: rec, now: time.Now}
}

// WithClock injects a clock (tests warp time; never real waits).
func (s *Service) WithClock(now func() time.Time) *Service { s.now = now; return s }

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// Get returns the raw subscription row.
func (s *Service) Get(ctx context.Context, orgID string) (store.Subscription, error) {
	sub, err := s.q.GetSubscription(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Subscription{}, ErrNoSubscription
	}
	return sub, err
}

// --- transitions ------------------------------------------------------------

// StartTrial puts a subscription into a trial — ONLY from the inert free row
// (status current + plan free). Guarding on plan too prevents flipping a paying
// customer back to trial (which would stop billing). trialDays<=0 = default.
func (s *Service) StartTrial(ctx context.Context, orgID, plan string, trialDays int) (store.Subscription, error) {
	sub, err := s.Get(ctx, orgID)
	if err != nil {
		return sub, err
	}
	if sub.Status != "current" || sub.Plan != "free" {
		return sub, ErrBadTransition
	}
	if trialDays <= 0 {
		trialDays = defaultTrialDays
	}
	end := s.now().Add(time.Duration(trialDays) * 24 * time.Hour)
	return s.commit(ctx, sub, "trial", plan, txParams{trialEnds: ts(end)}, "subscription.trial_started")
}

// Activate is payment-succeeded: from trial or ANY dunning state (or cancel) →
// current, clearing dunning + wind-down. Payment clears everything instantly
// (billing pack).
func (s *Service) Activate(ctx context.Context, orgID string) (store.Subscription, error) {
	sub, err := s.Get(ctx, orgID)
	if err != nil {
		return sub, err
	}
	switch sub.Status {
	case "trial", "current", "grace", "provisioning_paused", "suspended", "cancelled_at_anchor":
		// all recover to current; clears dunning + plan_ends
	default:
		return sub, ErrBadTransition
	}
	return s.commit(ctx, sub, "current", sub.Plan, txParams{}, "subscription.activated")
}

// FailPayment enters the dunning track: current → grace, day 0, first retry
// scheduled. Payment execution/retry is external (T11.4/Stripe); this only
// records the state.
func (s *Service) FailPayment(ctx context.Context, orgID string) (store.Subscription, error) {
	sub, err := s.Get(ctx, orgID)
	if err != nil {
		return sub, err
	}
	if sub.Status != "current" {
		return sub, ErrBadTransition
	}
	now := s.now()
	return s.commit(ctx, sub, "grace", sub.Plan, txParams{
		dunningStarted: ts(now), nextRetry: ts(now.Add(retryIntervalHours * time.Hour)),
	}, "subscription.payment_failed")
}

// Cancel schedules cancellation at the billing anchor: current/trial →
// cancelled_at_anchor, plan_ends_at = the next anchor. Resources keep running
// and metering (cancel ≠ delete).
func (s *Service) Cancel(ctx context.Context, orgID string) (store.Subscription, error) {
	sub, err := s.Get(ctx, orgID)
	if err != nil {
		return sub, err
	}
	// Cancel is never gated (billing pack: self-service always available) — allow
	// it from any active/dunning state, just not from an already-cancelled one.
	switch sub.Status {
	case "current", "trial", "grace", "provisioning_paused", "suspended":
	default:
		return sub, ErrBadTransition
	}
	ends := nextAnchor(s.now(), int(sub.AnchorDay))
	return s.commit(ctx, sub, "cancelled_at_anchor", sub.Plan, txParams{planEnds: ts(ends)}, "subscription.cancelled")
}

// Reactivate undoes a pending cancel before the anchor: cancelled_at_anchor →
// current, clearing the wind-down.
func (s *Service) Reactivate(ctx context.Context, orgID string) (store.Subscription, error) {
	sub, err := s.Get(ctx, orgID)
	if err != nil {
		return sub, err
	}
	if sub.Status != "cancelled_at_anchor" {
		return sub, ErrBadTransition
	}
	return s.commit(ctx, sub, "current", sub.Plan, txParams{}, "subscription.reactivated")
}

// AdvanceLifecycle is the time-driven progression a daily worker runs (and tests
// clock-warp): it advances dunning by elapsed days and applies a due cancel.
// Idempotent — safe to call repeatedly.
func (s *Service) AdvanceLifecycle(ctx context.Context, orgID string) (store.Subscription, error) {
	sub, err := s.Get(ctx, orgID)
	if err != nil {
		return sub, err
	}
	now := s.now()

	// a trial that reached its end without converting enters the dunning track
	// (payment is now required). T11.4/Stripe may convert-on-charge before this.
	if sub.Status == "trial" && sub.TrialEndsAt.Valid && !now.Before(sub.TrialEndsAt.Time) {
		return s.commit(ctx, sub, "grace", sub.Plan, txParams{
			dunningStarted: ts(now), nextRetry: ts(now.Add(retryIntervalHours * time.Hour)),
		}, "subscription.trial_expired")
	}

	// cancel takes effect at the anchor: drop to free, back to current.
	if sub.Status == "cancelled_at_anchor" && sub.PlanEndsAt.Valid && !now.Before(sub.PlanEndsAt.Time) {
		return s.commit(ctx, sub, "current", "free", txParams{}, "subscription.plan_ended")
	}

	// dunning day progression.
	if sub.DunningStartedAt.Valid {
		days := int(now.Sub(sub.DunningStartedAt.Time).Hours()) / 24
		if sub.Status == "grace" && days >= graceToPausedDays {
			return s.commit(ctx, sub, "provisioning_paused", sub.Plan, txParams{
				dunningStarted: sub.DunningStartedAt, nextRetry: ts(now.Add(retryIntervalHours * time.Hour)),
			}, "subscription.provisioning_paused")
		}
		if sub.Status == "provisioning_paused" && days >= pausedToSuspendDays {
			return s.commit(ctx, sub, "suspended", sub.Plan, txParams{dunningStarted: sub.DunningStartedAt}, "subscription.suspended")
		}
	}
	return sub, nil
}

// AdvanceAll runs the lifecycle sweep once — every sub in a dunning track or
// with a due cancel is advanced. Idempotent; returns the count swept.
func (s *Service) AdvanceAll(ctx context.Context) (int, error) {
	orgs, err := s.q.ListSubscriptionsToAdvance(ctx)
	if err != nil {
		return 0, err
	}
	for _, orgID := range orgs {
		// A concurrent transition (another replica's sweep, a webhook) winning
		// the CAS is expected and benign — the state already moved.
		if _, err := s.AdvanceLifecycle(ctx, orgID); err != nil && !errors.Is(err, ErrConcurrentModification) {
			slog.Error("subscription: advance failed", "org", orgID, "err", err)
		}
	}
	return len(orgs), nil
}

// RunLifecycle ticks the sweep until ctx is cancelled — the daily dunning/anchor
// progression (checks hourly so a day boundary is caught promptly). The
// thresholds are day-granular, so re-checks are cheap and idempotent.
func (s *Service) RunLifecycle(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := s.AdvanceAll(ctx); err != nil {
				slog.Error("subscription: lifecycle sweep failed", "err", err)
			}
		}
	}
}

// --- persistence + audit ----------------------------------------------------

type txParams struct {
	dunningStarted pgtype.Timestamptz
	nextRetry      pgtype.Timestamptz
	planEnds       pgtype.Timestamptz
	trialEnds      pgtype.Timestamptz
}

func (s *Service) commit(ctx context.Context, cur store.Subscription, status, plan string, p txParams, action string) (store.Subscription, error) {
	// Compare-and-swap on status: the write lands only if the row is still in the
	// status we read (cur.Status). A concurrent transition wins the race and this
	// returns no row → stale decision, never a lost update.
	out, err := s.q.SetSubscriptionState(ctx, store.SetSubscriptionStateParams{
		OrgID: cur.OrgID, Expected: cur.Status, Status: status, Plan: plan,
		DunningStartedAt: p.dunningStarted, NextRetryAt: p.nextRetry,
		PlanEndsAt: p.planEnds, TrialEndsAt: p.trialEnds,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Subscription{}, ErrConcurrentModification
	}
	if err != nil {
		return store.Subscription{}, fmt.Errorf("subscription: set state: %w", err)
	}
	if s.rec != nil {
		_, _ = s.rec.Append(ctx, events.Input{
			OrgID: cur.OrgID, Kind: "billing", Via: "system", Actor: "system",
			Action: action, Subject: cur.OrgID,
			Detail: []byte(fmt.Sprintf(`{"from":%q,"to":%q,"plan":%q}`, cur.Status, status, plan)),
		})
	}
	return out, nil
}

// nextAnchor returns the next occurrence of anchorDay (1–28) at/after now.
func nextAnchor(now time.Time, anchorDay int) time.Time {
	y, m, _ := now.Date()
	cand := time.Date(y, m, anchorDay, 0, 0, 0, 0, now.Location())
	if !cand.After(now) {
		cand = cand.AddDate(0, 1, 0)
	}
	return cand
}
