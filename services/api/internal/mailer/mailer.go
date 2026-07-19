// Package mailer is the M11 comms integration (T10.4, ADR-0009): outbound email
// behind a Provider interface (Resend is the implementation, never a platform
// dependency), driven ONLY by spine Events — the Dispatcher's single input is a
// store.Event, so nothing but an Event kind can send mail (the AC). Templates
// are versioned and provider-agnostic: rendered to {subject, html, text} before
// the provider boundary. "Buy, don't build" — comms infra is a never-build.
package mailer

import (
	"context"
	"log/slog"
)

// Message is a fully-rendered, provider-agnostic email. Templates produce it;
// the provider only transports it.
type Message struct {
	From    string
	To      string
	Subject string
	HTML    string
	Text    string
}

// Provider transports a rendered Message. Swappable (Resend today; SES/Postmark/
// SMTP later) without touching templates, dispatch, or callers (ADR-0009).
type Provider interface {
	Name() string
	Send(ctx context.Context, m Message) (providerID string, err error)
}

// Noop is the provider used when no real provider is configured (no API key):
// it logs and succeeds, so every environment runs safely without credentials —
// the send path is exercised, the wire call is not. Never used in prod config.
type Noop struct{}

func (Noop) Name() string { return "noop" }

func (Noop) Send(_ context.Context, m Message) (string, error) {
	slog.Info("email (noop provider — no key configured)", "to", m.To, "subject", m.Subject)
	return "noop_" + m.To, nil
}
