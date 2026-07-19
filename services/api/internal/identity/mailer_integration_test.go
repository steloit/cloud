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
