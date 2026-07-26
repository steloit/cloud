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

// A crafted org name must NOT inject markup into the HTML body (phishing from a
// trusted sender). The plain-text body is inert and left unescaped.
func TestInviteTemplateEscapesHTML(t *testing.T) {
	_, htmlBody, _ := inviteTemplate.Render(map[string]string{
		"org":        `"><a href="https://evil.example">Reset</a>`,
		"role":       "<img src=x onerror=alert(1)>",
		"accept_url": "https://c/invite/inv_1",
	})
	if strings.Contains(htmlBody, "<a href=\"https://evil.example\"") || strings.Contains(htmlBody, "<img src=x") {
		t.Fatalf("HTML injection not escaped: %q", htmlBody)
	}
	if !strings.Contains(htmlBody, "&lt;") && !strings.Contains(htmlBody, "&gt;") && !strings.Contains(htmlBody, "&#34;") {
		t.Fatalf("expected escaped entities in: %q", htmlBody)
	}
}

// The renewal template also escapes user-controlled data (org name is
// renamable to arbitrary text) — no HTML injection into the inviter's email.
func TestInviteRenewalTemplateEscapesHTML(t *testing.T) {
	_, htmlBody, _ := inviteRenewalTemplate.Render(map[string]string{
		"invitee": "x@y.z",
		"org":     `<a href="https://evil.example">Verify</a>`,
	})
	if strings.Contains(htmlBody, `<a href="https://evil.example"`) {
		t.Fatalf("HTML injection not escaped: %q", htmlBody)
	}
	if !strings.Contains(htmlBody, "&lt;") {
		t.Fatalf("expected escaped entities: %q", htmlBody)
	}
}

// The registry IS the event→template map: only listed actions send mail.
func TestRegistryIsTheEventMap(t *testing.T) {
	reg := registry()
	for _, action := range []string{"invite.created", "invite.renewal_requested", "password.reset_requested"} {
		if _, ok := reg[action]; !ok {
			t.Errorf("%s should trigger an email", action)
		}
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
