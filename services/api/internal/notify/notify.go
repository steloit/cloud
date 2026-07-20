// Package notify is the T10.3 routing matrix: one spine event (T2.5) projected
// onto the routes a principal chose — the in-app bell, email, and org webhooks.
// Prefs ROUTE; they never gag a recording (glossary law). The webhook route
// runs off a durable outbox (the spine + delivery ledger, like the mailer);
// the bell/email route is an explicit projection a caller pushes with a title.
package notify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

// Store is the persistence the router needs (sqlc queries satisfy it).
type Store interface {
	ListOrgMemberRecipients(ctx context.Context, orgID string) ([]store.ListOrgMemberRecipientsRow, error)
	GetNotificationPrefs(ctx context.Context, userID string) (store.NotificationPref, error)
	InsertNotification(ctx context.Context, arg store.InsertNotificationParams) (store.Notification, error)
	ListPendingBellEvents(ctx context.Context) ([]store.Event, error)
	ClaimBellEvent(ctx context.Context, eventID string) (string, error)
	MarkBellEventUnrenderable(ctx context.Context, eventID string) error
	GetDeployNotificationContext(ctx context.Context, arg store.GetDeployNotificationContextParams) (store.GetDeployNotificationContextRow, error)
	ListPendingWebhookEvents(ctx context.Context) ([]store.ListPendingWebhookEventsRow, error)
	GetWebhook(ctx context.Context, id string) (store.Webhook, error)
	ClaimWebhookDelivery(ctx context.Context, arg store.ClaimWebhookDeliveryParams) (string, error)
	MarkWebhookDeliverySent(ctx context.Context, arg store.MarkWebhookDeliverySentParams) error
	MarkWebhookDeliveryFailed(ctx context.Context, arg store.MarkWebhookDeliveryFailedParams) error
}

// EmailSender routes a notification to email. Optional — nil means the email
// channel is a no-op (the bell is the durable record). Notification email is
// best-effort by design; transactional mail (invites, reset) is the mailer's
// durable outbox, never gated by notification prefs.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Router fans events onto routes. Webhook secrets are sealed (recoverable to
// sign), so it holds a KEK.
type Router struct {
	q        Store
	kek      secrets.KEK
	email    EmailSender
	http     *http.Client
	now      func() time.Time
	insecure bool // test-only: deliver to http/loopback targets
	tx       Txer // US-10.3: transaction source for the bell projection
}

func NewRouter(q Store, kek secrets.KEK) *Router {
	return &Router{q: q, kek: kek, http: guardedClient(), now: time.Now}
}

func (r *Router) WithClock(now func() time.Time) *Router { r.now = now; return r }
func (r *Router) WithEmail(s EmailSender) *Router        { r.email = s; return r }
func (r *Router) WithHTTPClient(c *http.Client) *Router  { r.http = c; return r }

// WithInsecureTargets relaxes the SSRF scheme/address guard for tests that
// deliver to an httptest loopback server. NEVER call this in production.
func (r *Router) WithInsecureTargets() *Router { r.insecure = true; return r }

// webhookAAD binds a sealed webhook secret to its row — ciphertext lifted to
// another webhook won't open.
func webhookAAD(id string) []byte { return []byte("webhook:" + id) }

// MintSecret creates a signing secret for a new webhook: the plaintext exists
// once (returned to seal into WebhookCreated.secret), the sealed envelope is
// stored so every future delivery can recover it to sign.
func (r *Router) MintSecret(webhookID string) (secret string, sealed secrets.Sealed, err error) {
	b := make([]byte, 24)
	if _, err = rand.Read(b); err != nil {
		return "", secrets.Sealed{}, err
	}
	secret = "whsec_" + hex.EncodeToString(b)
	sealed, err = secrets.Seal(r.kek, []byte(secret), webhookAAD(webhookID))
	return secret, sealed, err
}

// openSecret recovers a webhook's signing secret from its envelope.
func (r *Router) openSecret(w store.Webhook) ([]byte, error) {
	return secrets.Open(r.kek, secrets.Sealed{
		Ciphertext: w.Ciphertext, Nonce: w.Nonce,
		WrappedDEK: w.WrappedDek, DEKNonce: w.DekNonce, KEKID: w.KekID,
	}, webhookAAD(w.ID))
}

// NotifyInput is a bell/email projection of an event: the content a caller
// supplies (title/body/link are microcopy the router must not invent).
type NotifyInput struct {
	OrgID   string
	EventID string // spine event id (empty ⇒ null; the bell row still records)
	Kind    string
	Title   string
	Body    string
	Link    string
	// Urgent marks a security/paging-class notification: quiet hours NEVER gate
	// it (US-10.2 — quiet hours affect routing only; safety/paging is never
	// deferred). The bell + spine recording are unaffected either way.
	Urgent bool
}

// Notify fans a projection to every org member's bell + email, each gated by
// that member's prefs. A pref-off suppresses the ROUTE, never the underlying
// spine recording (which happened before this call).
func (r *Router) Notify(ctx context.Context, in NotifyInput) error {
	recips, err := r.q.ListOrgMemberRecipients(ctx, in.OrgID)
	if err != nil {
		return err
	}
	for _, m := range recips {
		p := r.prefsFor(ctx, m.UserID)
		if p.inapp {
			if _, err := r.q.InsertNotification(ctx, store.InsertNotificationParams{
				ID: ids.New("ntf"), UserID: m.UserID, EventID: nullText(in.EventID),
				Kind: in.Kind, Title: in.Title, Body: in.Body, Link: in.Link,
			}); err != nil {
				slog.Error("notify: insert bell", "user", m.UserID, "err", err)
			}
		}
		// email routes unless quiet hours suppress it — but an Urgent
		// (security/paging) notification bypasses quiet hours entirely (never
		// gated). Recording (the bell above) already happened regardless.
		if p.email && r.email != nil && (in.Urgent || !p.quietNow(r.now())) {
			body := in.Body
			if in.Link != "" {
				body += "\n\n" + in.Link
			}
			if err := r.email.Send(ctx, m.Email, in.Title, body); err != nil {
				// best-effort: the bell is the durable record.
				slog.Warn("notify: email route", "user", m.UserID, "err", err)
			}
		}
	}
	return nil
}

// ProcessWebhooks drains one batch of the durable webhook outbox: spine events
// an active webhook subscribes to and hasn't been delivered. Idempotent and
// lossless on restart (the spine + ledger ARE the outbox). Returns the count.
func (r *Router) ProcessWebhooks(ctx context.Context) (int, error) {
	rows, err := r.q.ListPendingWebhookEvents(ctx)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		w, err := r.q.GetWebhook(ctx, row.WebhookID)
		if err != nil {
			slog.Error("notify: load webhook", "webhook", row.WebhookID, "err", err)
			continue
		}
		secret, err := r.openSecret(w)
		if err != nil {
			slog.Error("notify: open webhook secret", "webhook", w.ID, "err", err)
			continue
		}
		body, _ := json.Marshal(payload{
			ID: row.ID, Kind: row.Kind, Action: row.Action, Subject: row.Subject,
			Actor: row.Actor, OrgID: row.OrgID, At: row.At.Time,
			Detail: json.RawMessage(row.Detail),
		})
		r.deliver(ctx, w, row.ID, secret, body)
	}
	return len(rows), nil
}

// RunOutbox polls the webhook outbox until ctx is cancelled (a River-backed
// worker with backoff is the follow-up; the ledger makes this loop lossless).
func (r *Router) RunOutbox(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := r.ProcessWebhooks(ctx); err != nil {
				slog.Error("notify: webhook outbox scan", "err", err)
			}
		}
	}
}

// Test delivers a synthetic event to a webhook SYNCHRONOUSLY and returns the
// result inline (U8: never a dead end). It bypasses the ledger — a test is not
// a real event and must not consume the (webhook, event) idempotency slot.
func (r *Router) Test(ctx context.Context, webhookID string) (Result, error) {
	w, err := r.q.GetWebhook(ctx, webhookID)
	if err != nil {
		return Result{}, err
	}
	secret, err := r.openSecret(w)
	if err != nil {
		return Result{}, err
	}
	body, _ := json.Marshal(payload{
		ID: "evt_test", Kind: "test", Action: "webhook.test",
		OrgID: w.OrgID, At: r.now(),
	})
	return r.post(ctx, w, secret, body)
}

// prefs is a resolved per-user routing decision.
type prefs struct {
	inapp, email bool
	quiet        *quietHours
}

type quietHours struct {
	start, end, tz string
}

// quietNow reports whether now falls in the user's quiet window. During it the
// email route is SKIPPED (notification email is best-effort — the bell is the
// durable record; there is no deferral queue); bell + webhook are unaffected.
// A malformed window is never quiet.
func (p prefs) quietNow(now time.Time) bool {
	if p.quiet == nil || p.quiet.start == "" || p.quiet.end == "" {
		return false
	}
	loc := time.UTC
	if p.quiet.tz != "" {
		if l, err := time.LoadLocation(p.quiet.tz); err == nil {
			loc = l
		}
	}
	hm := now.In(loc).Format("15:04")
	if p.quiet.start <= p.quiet.end {
		return hm >= p.quiet.start && hm < p.quiet.end // same-day window
	}
	return hm >= p.quiet.start || hm < p.quiet.end // window crosses midnight
}

// prefsFor loads a user's routing prefs; a missing row is the default
// (bell + email on, no quiet hours) — never a route-suppression.
func (r *Router) prefsFor(ctx context.Context, userID string) prefs {
	return r.prefsForStore(ctx, r.q, userID)
}

// prefsForStore is prefsFor against an explicit Store, so the bell projection
// reads prefs on its OWN transaction rather than the pool — one consistent
// snapshot per fan-out instead of a mix of tx and non-tx reads.
func (r *Router) prefsForStore(ctx context.Context, q Store, userID string) prefs {
	def := prefs{inapp: true, email: true}
	row, err := q.GetNotificationPrefs(ctx, userID)
	if err != nil {
		if err != pgx.ErrNoRows {
			slog.Error("notify: load prefs", "user", userID, "err", err)
		}
		return def
	}
	var ch struct {
		Email *bool `json:"email"`
		Inapp *bool `json:"inapp"`
	}
	if len(row.Channels) > 0 {
		_ = json.Unmarshal(row.Channels, &ch)
	}
	if ch.Inapp != nil {
		def.inapp = *ch.Inapp
	}
	if ch.Email != nil {
		def.email = *ch.Email
	}
	if len(row.QuietHours) > 0 && string(row.QuietHours) != "null" {
		var q struct {
			Start    string `json:"start"`
			End      string `json:"end"`
			Timezone string `json:"timezone"`
		}
		if json.Unmarshal(row.QuietHours, &q) == nil && q.Start != "" {
			def.quiet = &quietHours{start: q.Start, end: q.End, tz: q.Timezone}
		}
	}
	return def
}

func nullText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
