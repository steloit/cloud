package identity_test

// T10.4: the email outbox end-to-end against a real spine — an invite.created
// event is drained by the dispatcher, sent via a spy provider, and recorded
// once (idempotent). Nothing but the Event triggers the send.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/mailer"
)

// spyProvider captures sent messages instead of calling a wire API.
type spyProvider struct {
	mu   sync.Mutex
	sent []mailer.Message
}

func (s *spyProvider) Name() string { return "spy" }
func (s *spyProvider) Send(_ context.Context, m mailer.Message) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, m)
	return "spy_" + m.To, nil
}
func (s *spyProvider) count() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.sent) }

// flakyProvider fails its first `failFirst` sends, then succeeds.
type flakyProvider struct {
	failFirst int
	calls     int
}

func (*flakyProvider) Name() string { return "flaky" }
func (f *flakyProvider) Send(_ context.Context, m mailer.Message) (string, error) {
	f.calls++
	if f.calls <= f.failFirst {
		return "", errFlaky
	}
	return "flaky_ok", nil
}

var errFlaky = errFlakySend{}

type errFlakySend struct{}

func (errFlakySend) Error() string { return "flaky: transient" }

func TestEmailOutboxDeliversInvite(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)

	_, ownerID := w.signupUser(t, "mailowner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "mailco", ownerID)
	if err != nil {
		t.Fatal(err)
	}

	// creating an invite emits invite.created on the spine
	inv, err := w.svc.CreateInvite(ctx, org.ID, "invitee@example.com", "developer", ownerID, false)
	if err != nil {
		t.Fatal(err)
	}

	spy := &spyProvider{}
	dispatcher := mailer.NewDispatcher(spy, q, identity.NewMailDirectory(q, "https://console.test"), "Steloit <noreply@steloit.app>")

	// only invite.created is a mail action
	if !dispatcher.Sends("invite.created") || dispatcher.Sends("member.added") {
		t.Fatal("event→template map wrong")
	}

	n, err := dispatcher.ProcessPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 || spy.count() != 1 {
		t.Fatalf("expected 1 email sent, got %d processed / %d sent", n, spy.count())
	}
	msg := spy.sent[0]
	if msg.To != "invitee@example.com" || msg.From == "" || msg.Subject == "" {
		t.Fatalf("email shape: %+v", msg)
	}

	// the delivery is recorded (sent), keyed to the triggering event
	var status, tmpl string
	if err := w.pool.QueryRow(ctx,
		"select status, template from email_deliveries where event_id=(select id from events where action='invite.created' and subject=$1)",
		inv.ID).Scan(&status, &tmpl); err != nil {
		t.Fatalf("delivery not recorded: %v", err)
	}
	if status != "sent" || tmpl != "org-invite" {
		t.Fatalf("delivery row: status=%q template=%q", status, tmpl)
	}

	// idempotent: re-draining sends nothing more (the delivery gate holds).
	if _, err := dispatcher.ProcessPending(ctx); err != nil {
		t.Fatal(err)
	}
	if spy.count() != 1 {
		t.Fatalf("re-drain double-sent: %d", spy.count())
	}
}

// A transient provider failure is recorded (not phantom-sent) and RECLAIMED on
// the next drain — the pending/failed row is re-scannable, so mail is not lost.
func TestEmailOutboxRetriesTransientFailure(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)

	_, ownerID := w.signupUser(t, "retry@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "retryco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := w.svc.CreateInvite(ctx, org.ID, "r-invitee@example.com", "developer", ownerID, false)
	if err != nil {
		t.Fatal(err)
	}

	flaky := &flakyProvider{failFirst: 1} // fail once, then succeed
	dispatcher := mailer.NewDispatcher(flaky, q, identity.NewMailDirectory(q, "https://c"), "noreply@steloit.app")

	// first drain: provider fails → recorded 'failed', not lost, not phantom-sent
	_, _ = dispatcher.ProcessPending(ctx)
	var status string
	_ = w.pool.QueryRow(ctx, "select status from email_deliveries where event_id=(select id from events where subject=$1 and action='invite.created')", inv.ID).Scan(&status)
	if status != "failed" {
		t.Fatalf("transient failure should record 'failed', got %q", status)
	}
	// second drain: reclaimed and sent
	_, _ = dispatcher.ProcessPending(ctx)
	_ = w.pool.QueryRow(ctx, "select status from email_deliveries where event_id=(select id from events where subject=$1 and action='invite.created')", inv.ID).Scan(&status)
	if status != "sent" {
		t.Fatalf("reclaim should send on retry, got %q", status)
	}
}

// A vanished invite yields a terminal 'skipped' row so the event drops out of
// the scan instead of being re-resolved forever (poison-event guard).
func TestEmailOutboxSkipsVanishedInvite(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)

	_, ownerID := w.signupUser(t, "skip@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "skipco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := w.svc.CreateInvite(ctx, org.ID, "gone@example.com", "developer", ownerID, false)
	if err != nil {
		t.Fatal(err)
	}
	// the invite vanishes between event and send
	if _, err := w.pool.Exec(ctx, "delete from invites where id=$1", inv.ID); err != nil {
		t.Fatal(err)
	}

	spy := &spyProvider{}
	dispatcher := mailer.NewDispatcher(spy, q, identity.NewMailDirectory(q, "https://c"), "noreply@steloit.app")
	if _, err := dispatcher.ProcessPending(ctx); err != nil {
		t.Fatal(err)
	}
	if spy.count() != 0 {
		t.Fatalf("sent mail for a vanished invite: %d", spy.count())
	}
	var status string
	if err := w.pool.QueryRow(ctx, "select status from email_deliveries where event_id=(select id from events where subject=$1 and action='invite.created')", inv.ID).Scan(&status); err != nil {
		t.Fatalf("no terminal row recorded: %v", err)
	}
	if status != "skipped" {
		t.Fatalf("vanished invite should be 'skipped', got %q", status)
	}
}
