package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// Txer runs fn inside a database transaction against a tx-bound Store,
// committing when fn returns nil and rolling back otherwise.
//
// It yields a Store — deliberately not a concrete *store.Queries — so a test
// can wrap it and fail the Nth insert, which is the only way to observe the
// partial-fan-out rollback the design depends on.
type Txer interface {
	WithTx(ctx context.Context, fn func(Store) error) error
}

// WithTxer supplies the transaction source ProcessBell needs. It is a builder
// rather than a NewRouter parameter so the existing construction sites — and
// the seam test that pins them — keep compiling.
func (r *Router) WithTxer(t Txer) *Router { r.tx = t; return r }

// errClaimedElsewhere unwinds the transaction when another replica owns the
// event. It is a control signal, not a failure: rolling back is exactly right,
// since this replica has nothing to commit.
var errClaimedElsewhere = errors.New("notify: event claimed by another replica")

// ProcessBell drains one batch of unprojected spine events onto the bell.
//
// Per event, in ONE transaction: claim it in bell_scanned, classify it, and —
// when it is notification-worthy and renderable — insert a bell row for every
// org member whose in-app pref is on. The claim is written for EVERY event,
// worthy or not; that terminal marker is what stops silent events from filling
// the scan window forever.
//
// Returns the number of events projected (worthy, rendered and fanned out), not
// the number scanned.
func (r *Router) ProcessBell(ctx context.Context) (int, error) {
	if r.tx == nil {
		return 0, errors.New("notify: ProcessBell requires WithTxer")
	}
	events, err := r.q.ListPendingBellEvents(ctx)
	if err != nil {
		return 0, fmt.Errorf("notify: scan pending bell events: %w", err)
	}

	projected := 0
	for _, ev := range events {
		// fanned distinguishes "handled" from "rang the bell". A silent event is
		// handled successfully and must NOT be counted, or the return value
		// reports scan volume instead of notifications — which is what the
		// caller and every test actually mean by "projected".
		fanned := false
		err := r.tx.WithTx(ctx, func(s Store) error {
			fanned = false
			// Claim first. No row means a concurrent replica already owns this
			// event, so unwind rather than fan out twice.
			if _, err := s.ClaimBellEvent(ctx, ev.ID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return errClaimedElsewhere
				}
				return fmt.Errorf("claim: %w", err)
			}

			c, worthy := classify(ev.Action)
			if !worthy {
				return nil // scanned and silent — the common case
			}
			title, ok, err := c.render(ctx, s, ev)
			if err != nil {
				return fmt.Errorf("render %s: %w", ev.Action, err)
			}
			if !ok {
				// Renderable data missing. Stay silent rather than invent copy;
				// the event stays marked so the scan still drains.
				slog.WarnContext(ctx, "bell: worthy event is unrenderable, skipping",
					"event", ev.ID, "action", ev.Action)
				return nil
			}
			if err := r.fanOut(ctx, s, ev, c.kind, title); err != nil {
				return err
			}
			fanned = true
			return nil
		})
		switch {
		case err == nil:
			if fanned {
				projected++
			}
		case errors.Is(err, errClaimedElsewhere):
			// Another replica has it; not an error and not our count.
		default:
			// One event's failure rolls back only its own transaction. Keep
			// draining — a poison event must not starve the other 99 slots.
			// It has no attempts column, so it retries every tick until fixed
			// (dead-letter gap filed in the ledger).
			slog.ErrorContext(ctx, "bell: projecting event failed", "event", ev.ID, "err", err)
		}
	}
	return projected, nil
}

// fanOut writes one bell row per org member whose in-app pref is on.
//
// Every insert carries EventID so the partial unique index dedupes a re-run.
// A pgx.ErrNoRows from InsertNotification is that index doing its job — the
// query is `:one` over ON CONFLICT DO NOTHING RETURNING, so a successful dedupe
// returns no row. Treating it as a failure would roll back the batch and turn
// the guard into a retry loop. Any OTHER error aborts, because a partial
// fan-out that commits is never retried and would leave members silently
// un-notified forever.
func (r *Router) fanOut(ctx context.Context, s Store, ev store.Event, kind, title string) error {
	recipients, err := s.ListOrgMemberRecipients(ctx, ev.OrgID)
	if err != nil {
		return fmt.Errorf("recipients: %w", err)
	}
	for _, rec := range recipients {
		p := r.prefsForStore(ctx, s, rec.UserID)
		if !p.inapp {
			continue
		}
		_, err := s.InsertNotification(ctx, store.InsertNotificationParams{
			ID:      ids.New("ntf"),
			UserID:  rec.UserID,
			EventID: nullText(ev.ID),
			Kind:    kind,
			Title:   title,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("insert bell row for %s: %w", rec.UserID, err)
		}
	}
	return nil
}

// RunBell drains the projection on a ticker until ctx is cancelled.
func (r *Router) RunBell(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := r.ProcessBell(ctx); err != nil {
				slog.ErrorContext(ctx, "bell: projection batch failed", "err", err)
			}
		}
	}
}
