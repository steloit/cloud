package notify

// T10.3 webhook route: a durable, signed, SSRF-guarded outbound POST per active
// org webhook, delivered through the same claim-before-send outbox shape as
// email (T10.4). The spine + the delivery ledger ARE the outbox — nothing is
// lost on restart.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// blockedIP reports whether a concrete address is one a webhook must never
// reach — loopback, RFC1918/ULA private, link-local (incl. 169.254.169.254
// cloud metadata), or unspecified.
func blockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// guardedClient is the production webhook client: it pins SSRF safety to the
// ACTUAL dialed IP (closing the DNS-rebinding TOCTOU between validation and
// connect) and refuses redirects (a public target must not 302 to an internal
// host). Tests replace it via WithHTTPClient with an unguarded loopback client.
func guardedClient() *http.Client {
	d := &net.Dialer{
		Timeout: deliverTimeout,
		// Control runs with the resolved address about to be connected — the
		// real dial IP, not a separate lookup that DNS can race.
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || blockedIP(ip) {
				return fmt.Errorf("webhook target resolved to a blocked address: %s", host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:   deliverTimeout,
		Transport: &http.Transport{DialContext: d.DialContext},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return fmt.Errorf("webhook target must not redirect")
		},
	}
}

// signatureHeader carries the HMAC-SHA256 of the raw body so a receiver can
// verify authenticity without the platform ever sending the secret itself.
const signatureHeader = "X-Steloit-Signature"

// deliverTimeout bounds a single POST — a slow receiver must never hang the
// outbox loop or (for a test) the request.
const deliverTimeout = 8 * time.Second

// payload is the wire shape delivered to a webhook: the spine event, verbatim.
type payload struct {
	ID      string          `json:"id"`
	Kind    string          `json:"kind"`
	Action  string          `json:"action"`
	Subject string          `json:"subject"`
	Actor   string          `json:"actor"`
	OrgID   string          `json:"org_id"`
	At      time.Time       `json:"at"`
	Detail  json.RawMessage `json:"detail,omitempty"`
}

// Result is a single delivery outcome (the testWebhook response, and the log).
type Result struct {
	Delivered  bool
	StatusCode int
	DurationMs int
}

// sign returns the hex HMAC-SHA256 of body under secret (the reveal-once value
// the caller held once at create time — we verify against its hash elsewhere).
func sign(secret, body []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

// ValidateURL is the create-time SSRF gate (https + routable host). Delivery
// re-checks, since DNS can be re-pointed between create and send.
func ValidateURL(raw string) error { return safeURL(raw, false) }

// safeURL rejects a webhook target that isn't https or that points at a
// private/loopback/link-local address — the SSRF guard, applied at create AND
// at delivery (DNS can be re-pointed between the two). insecure relaxes the
// scheme + address checks for tests that deliver to an httptest loopback
// server; it is NEVER set in production.
func safeURL(raw string, insecure bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	if insecure {
		return nil
	}
	if u.Scheme != "https" {
		return fmt.Errorf("webhook url must be https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook url has no host")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("webhook host does not resolve")
	}
	for _, ip := range ips {
		if blockedIP(ip) {
			return fmt.Errorf("webhook url resolves to a non-routable address")
		}
	}
	return nil
}

// post signs and delivers the body to the webhook, returning the outcome. A
// transport error or a non-2xx is `Delivered:false` with whatever status code
// was observed (0 on a transport error).
func (r *Router) post(ctx context.Context, w store.Webhook, secret, body []byte) (Result, error) {
	start := r.now()
	if err := safeURL(w.Url, r.insecure); err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, deliverTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.Url, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(signatureHeader, sign(secret, body))
	resp, err := r.http.Do(req)
	dur := int(r.now().Sub(start).Milliseconds())
	if err != nil {
		return Result{Delivered: false, StatusCode: 0, DurationMs: dur}, err
	}
	defer resp.Body.Close()
	return Result{
		Delivered:  resp.StatusCode >= 200 && resp.StatusCode < 300,
		StatusCode: resp.StatusCode,
		DurationMs: dur,
	}, nil
}

// deliver claims the (webhook, event) then signs+POSTs, recording the outcome.
// Idempotent: a `sent`/`skipped` row is terminal (claim returns no id → skip),
// so a re-processed event never double-delivers. The signing secret is the
// plaintext the caller recovers by decrypting the webhook's envelope (the
// reveal-once value the receiver was given once) — HMAC signing needs the real
// secret on every delivery, which is exactly why it is sealed-and-recoverable,
// not hashed.
func (r *Router) deliver(ctx context.Context, w store.Webhook, eventID string, secret, body []byte) {
	id, err := r.q.ClaimWebhookDelivery(ctx, store.ClaimWebhookDeliveryParams{
		WebhookID: w.ID, EventID: eventID, ID: ids.New("whd"),
	})
	if err != nil {
		slog.Error("notify: claim webhook delivery", "webhook", w.ID, "event", eventID, "err", err)
		return
	}
	if id == "" {
		return // terminal or retries exhausted
	}
	res, err := r.post(ctx, w, secret, body)
	if err != nil || !res.Delivered {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		if e := r.q.MarkWebhookDeliveryFailed(ctx, store.MarkWebhookDeliveryFailedParams{
			ID: id, StatusCode: int4(res.StatusCode), Error: text(msg),
		}); e != nil {
			slog.Error("notify: mark webhook failed", "delivery", id, "err", e)
		}
		return
	}
	if e := r.q.MarkWebhookDeliverySent(ctx, store.MarkWebhookDeliverySentParams{
		ID: id, StatusCode: int4(res.StatusCode),
	}); e != nil {
		slog.Error("notify: mark webhook sent", "delivery", id, "err", e)
	}
}

func int4(n int) pgtype.Int4 {
	if n == 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(n), Valid: true}
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
