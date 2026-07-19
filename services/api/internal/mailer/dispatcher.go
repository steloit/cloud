package mailer

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// Dispatcher turns spine Events into emails — and it is the ONLY thing that
// sends mail: its single public input is a store.Event (T10.4 AC). An event
// with no rule sends nothing. Delivery is idempotent (claim-then-send), so a
// re-processed event never double-sends.
type Dispatcher struct {
	provider Provider
	q        *store.Queries
	dir      Directory
	from     string
	rules    map[string]Rule
}

func NewDispatcher(provider Provider, q *store.Queries, dir Directory, from string) *Dispatcher {
	return &Dispatcher{provider: provider, q: q, dir: dir, from: from, rules: registry()}
}

// Sends reports whether an event action triggers an email (test/introspection).
func (d *Dispatcher) Sends(action string) bool {
	_, ok := d.rules[action]
	return ok
}

// actions returns every mail-triggering event action (the outbox scan filter).
func (d *Dispatcher) actions() []string {
	as := make([]string, 0, len(d.rules))
	for a := range d.rules {
		as = append(as, a)
	}
	return as
}

// ProcessPending drains one batch of the durable outbox: spine events that
// should send mail but haven't yet. Idempotent — safe to call repeatedly and
// after a restart. Returns the number of events processed.
func (d *Dispatcher) ProcessPending(ctx context.Context) (int, error) {
	evts, err := d.q.ListPendingMailEvents(ctx, d.actions())
	if err != nil {
		return 0, err
	}
	for _, e := range evts {
		if err := d.Dispatch(ctx, e); err != nil {
			// one bad event must not stall the batch; it stays pending (no
			// delivery row on a resolve failure) for the next pass.
			slog.Error("mailer: dispatch failed", "event", e.ID, "action", e.Action, "err", err)
		}
	}
	return len(evts), nil
}

// RunOutbox polls the durable outbox until ctx is cancelled — the near-real-time
// dispatch loop (a River-backed worker with backoff is the follow-up; the spine
// + delivery ledger make this loop safe and lossless meanwhile).
func (d *Dispatcher) RunOutbox(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := d.ProcessPending(ctx); err != nil {
				slog.Error("mailer: outbox scan failed", "err", err)
			}
		}
	}
}

// Dispatch processes one event: resolve its rule (none ⇒ no mail), claim the
// (event, recipient) delivery, render, send, and record the outcome. Returns
// nil when there is nothing to send or the mail is already delivered.
func (d *Dispatcher) Dispatch(ctx context.Context, evt store.Event) error {
	rule, ok := d.rules[evt.Action]
	if !ok {
		return nil // this event does not send mail
	}
	recipient, orgID, data, err := rule.Resolve(ctx, d.dir, evt)
	if err != nil {
		return err
	}
	if recipient == "" {
		// Nothing to send (vanished invite / missing org). Record a TERMINAL
		// skipped row so the event drops out of the scan — otherwise the outbox
		// re-resolves it every poll and can starve newer mail (poison-event).
		return d.q.RecordSkippedDelivery(ctx, store.RecordSkippedDeliveryParams{
			ID: ids.New("eml"), EventID: evt.ID, OrgID: nullText(orgID),
			Recipient: "(skipped)", Template: rule.Template.Name,
			TemplateVersion: int32(rule.Template.Version), Provider: d.provider.Name(),
		})
	}

	// Claim first as `pending` (written BEFORE the send): the idempotency gate.
	// No id back ⇒ already sent/skipped or retries exhausted ⇒ skip.
	id, err := d.q.ClaimEmailDelivery(ctx, store.ClaimEmailDeliveryParams{
		ID: ids.New("eml"), EventID: evt.ID, OrgID: nullText(orgID),
		Recipient: recipient, Template: rule.Template.Name,
		TemplateVersion: int32(rule.Template.Version), Provider: d.provider.Name(),
	})
	if err != nil {
		return err
	}
	if id == "" {
		return nil // already delivered/terminal, or retries exhausted
	}

	subject, html, text := rule.Template.Render(data)
	providerID, sendErr := d.provider.Send(ctx, Message{
		From: d.from, To: recipient, Subject: subject, HTML: html, Text: text,
	})
	if sendErr != nil {
		// A durable retry-worker is a follow-up; for now the failure is recorded
		// and logged loudly (never silently swallowed).
		if err := d.q.MarkEmailDeliveryFailed(ctx, store.MarkEmailDeliveryFailedParams{ID: id, Error: nullText(sendErr.Error())}); err != nil {
			slog.Error("mailer: mark-failed write failed", "delivery", id, "err", err)
		}
		return sendErr
	}
	if err := d.q.MarkEmailDeliverySent(ctx, store.MarkEmailDeliverySentParams{ID: id, ProviderID: nullText(providerID)}); err != nil {
		slog.Error("mailer: mark-sent write failed", "delivery", id, "err", err)
	}
	return nil
}

func nullText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
