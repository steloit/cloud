package mailer

import (
	"context"
	"strings"
	"testing"
)

// The invite template renders provider-agnostic {subject, html, text} with the
// data substituted — no provider involved.
func TestInviteTemplateRenders(t *testing.T) {
	subject, html, text := inviteTemplate.Render(map[string]string{
		"org": "Acme", "role": "developer", "accept_url": "https://c/invite/inv_1",
	})
	if !strings.Contains(subject, "Acme") {
		t.Fatalf("subject: %q", subject)
	}
	for _, body := range []string{html, text} {
		if !strings.Contains(body, "Acme") || !strings.Contains(body, "developer") || !strings.Contains(body, "https://c/invite/inv_1") {
			t.Fatalf("body missing data: %q", body)
		}
	}
	if inviteTemplate.Version < 1 {
		t.Fatal("template must be versioned")
	}
}

// The registry IS the event→template map: only listed actions send mail.
func TestRegistryIsTheEventMap(t *testing.T) {
	reg := registry()
	if _, ok := reg["invite.created"]; !ok {
		t.Fatal("invite.created should trigger an email")
	}
	for _, action := range []string{"member.added", "org.created", "policy.updated", "authz.denied"} {
		if _, ok := reg[action]; ok {
			t.Errorf("%s should NOT trigger an email (only explicit rules do)", action)
		}
	}
}

// The Noop provider (no key) succeeds without a wire call, so the app runs
// without credentials.
func TestNoopProvider(t *testing.T) {
	id, err := Noop{}.Send(context.Background(), Message{To: "a@b.c", Subject: "hi"})
	if err != nil || id == "" {
		t.Fatalf("noop send: %q %v", id, err)
	}
	if (Noop{}).Name() != "noop" {
		t.Fatal("noop name")
	}
}
