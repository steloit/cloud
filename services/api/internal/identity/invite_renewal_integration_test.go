package identity_test

// T7.5: an expired invite's renewal request notifies the INVITER by email, and
// a declined invite's link is invalidated.

import (
	"context"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/mailer"
)

func TestInviteRenewalNotifiesInviter(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)

	// owner (the inviter) with a real address
	_, ownerID := w.signupUser(t, "inviter@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "renewco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := w.svc.CreateInvite(ctx, org.ID, "invitee@example.com", "developer", ownerID, false)
	if err != nil {
		t.Fatal(err)
	}
	// force it expired so renew is allowed (renew requires status=expired)
	if _, err := w.pool.Exec(ctx, "update invites set expires_at = now() - interval '1 day' where id=$1", inv.ID); err != nil {
		t.Fatal(err)
	}

	// the invitee requests a fresh link (public endpoint) → 202
	resp, out := w.post(t, "/v1/invites/"+inv.ID+"/renew", "", "")
	if resp.StatusCode != 202 {
		t.Fatalf("renew: %d %s", resp.StatusCode, out)
	}

	// the org-event outbox delivers the renewal email TO THE INVITER
	spy := &spyProvider{}
	disp := mailer.NewDispatcher(spy, q, identity.NewMailDirectory(q, "https://c", w.kek), "noreply@steloit.app")
	if _, err := disp.ProcessPending(ctx); err != nil {
		t.Fatal(err)
	}
	// two org events fired: the original invite (to the invitee) and the
	// renewal request (to the inviter). Find the renewal one.
	var renewal *mailer.Message
	for i := range spy.sent {
		if spy.sent[i].To == "inviter@example.com" {
			renewal = &spy.sent[i]
		}
	}
	if renewal == nil {
		t.Fatalf("no renewal email to the inviter among %d sent", spy.count())
	}
	if !contains(renewal.Text, "invitee@example.com") || !contains(renewal.Text, "renewco") {
		t.Fatalf("renewal email missing context: %q", renewal.Text)
	}
}

func TestDeclineInvalidatesInvite(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	_, ownerID := w.signupUser(t, "owner-d@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "declineco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := w.svc.CreateInvite(ctx, org.ID, "declinee@example.com", "developer", ownerID, false)
	if err != nil {
		t.Fatal(err)
	}

	// decline → 204
	resp, _ := w.del(t, "/v1/invites/"+inv.ID, "")
	if resp.StatusCode != 204 {
		t.Fatalf("decline: %d", resp.StatusCode)
	}
	// the public view now reports the link is gone (410), and accept is refused
	resp, _ = w.get(t, "/v1/invites/"+inv.ID, "")
	if resp.StatusCode != 410 {
		t.Fatalf("declined invite public view should be 410: %d", resp.StatusCode)
	}
	// declining again is refused (already gone)
	resp, _ = w.del(t, "/v1/invites/"+inv.ID, "")
	if resp.StatusCode == 204 {
		t.Fatal("a declined invite was declined again (link not invalidated)")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
