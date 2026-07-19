package mailer

import (
	"context"
	"html"

	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// Template is a versioned, provider-agnostic email: it renders to {subject,
// html, text} BEFORE the provider boundary, so a provider swap never touches
// template state and every send records which template+version produced it
// (reproducibility, ADR-0009). Bump Version on any content change.
type Template struct {
	Name    string
	Version int
	render  func(data map[string]string) (subject, html, text string)
}

func (t Template) Render(data map[string]string) (subject, html, text string) {
	return t.render(data)
}

// Directory resolves the domain data an email needs (recipient, org, template
// data) from a spine Event, behind an interface so the mailer never imports the
// identity store directly. The identity module provides the adapter.
type Directory interface {
	// InviteEmail returns the invitee's address, the org id, and template data
	// for an invite event whose Subject is the invite id.
	InviteEmail(ctx context.Context, inviteID string) (recipient, orgID string, data map[string]string, err error)
	// PasswordResetEmail returns the account's address and template data (incl.
	// the reset link built from the sealed token) for a reset event whose
	// Subject is the reset-token id. Account-level: orgID is "".
	PasswordResetEmail(ctx context.Context, resetID string) (recipient, orgID string, data map[string]string, err error)
}

// Rule binds a spine Event action to the template it sends and how to resolve
// its recipient. The registry IS the event→template map (T10.4 AC).
type Rule struct {
	Template Template
	Resolve  func(ctx context.Context, dir Directory, evt store.Event) (recipient, orgID string, data map[string]string, err error)
}

// --- the V1 templates -------------------------------------------------------

var inviteTemplate = Template{
	Name:    "org-invite",
	Version: 1,
	render: func(d map[string]string) (string, string, string) {
		// org/role are free-form user input (an org can be renamed to arbitrary
		// text). The plain-text subject/body are inert; the HTML body MUST
		// escape every interpolated value or a crafted org name injects markup
		// into an email sent from a trusted domain (phishing).
		org, role, url := d["org"], d["role"], d["accept_url"]
		eOrg, eRole, eURL := html.EscapeString(org), html.EscapeString(role), html.EscapeString(url)
		subject := "You've been invited to " + org + " on Steloit"
		text := "You've been invited to join " + org + " as " + role + ".\n\n" +
			"Accept: " + url + "\n\nThis invitation expires in 7 days."
		htmlBody := "<p>You've been invited to join <strong>" + eOrg + "</strong> as " + eRole + ".</p>" +
			`<p><a href="` + eURL + `">Accept the invitation</a></p>` +
			"<p>This invitation expires in 7 days.</p>"
		return subject, htmlBody, text
	},
}

var passwordResetTemplate = Template{
	Name:    "password-reset",
	Version: 1,
	render: func(d map[string]string) (string, string, string) {
		url := d["reset_url"]
		eURL := html.EscapeString(url)
		subject := "Reset your Steloit password"
		text := "We received a request to reset your Steloit password.\n\n" +
			"Reset it here: " + url + "\n\nThis link expires in 1 hour. " +
			"If you didn't request this, you can ignore this email — your password is unchanged."
		htmlBody := "<p>We received a request to reset your Steloit password.</p>" +
			`<p><a href="` + eURL + `">Reset your password</a></p>` +
			"<p>This link expires in 1 hour. If you didn't request this, you can ignore this email — your password is unchanged.</p>"
		return subject, htmlBody, text
	},
}

// registry maps event actions to their email rule. Adding a mail-triggering
// event = adding a rule here; nothing else can send mail.
func registry() map[string]Rule {
	return map[string]Rule{
		"invite.created": {
			Template: inviteTemplate,
			Resolve: func(ctx context.Context, dir Directory, evt store.Event) (string, string, map[string]string, error) {
				return dir.InviteEmail(ctx, evt.Subject)
			},
		},
		"password.reset_requested": {
			Template: passwordResetTemplate,
			Resolve: func(ctx context.Context, dir Directory, evt store.Event) (string, string, map[string]string, error) {
				return dir.PasswordResetEmail(ctx, evt.Subject)
			},
		},
	}
}
