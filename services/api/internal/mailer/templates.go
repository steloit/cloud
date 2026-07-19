package mailer

import (
	"context"

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
		subject := "You've been invited to " + d["org"] + " on Steloit"
		text := "You've been invited to join " + d["org"] + " as " + d["role"] + ".\n\n" +
			"Accept: " + d["accept_url"] + "\n\nThis invitation expires in 7 days."
		html := "<p>You've been invited to join <strong>" + d["org"] + "</strong> as " + d["role"] + ".</p>" +
			`<p><a href="` + d["accept_url"] + `">Accept the invitation</a></p>` +
			"<p>This invitation expires in 7 days.</p>"
		return subject, html, text
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
	}
}
