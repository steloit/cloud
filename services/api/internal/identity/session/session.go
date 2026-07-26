// Package session owns server-side sessions: opaque cookie tokens (raw value
// never stored — sha256 hash only), secure cookie attributes, and the
// context plumbing that lets strict handlers set cookies and read the
// authenticated principal.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"time"
)

const CookieName = "steloit_session"

type Manager struct {
	TTL    time.Duration
	Secure bool
}

func NewManager(ttl time.Duration, secure bool) *Manager {
	return &Manager{TTL: ttl, Secure: secure}
}

// NewToken mints the raw cookie value and its storage hash.
func (m *Manager) NewToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

func HashToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

func (m *Manager) Cookie(raw string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name: CookieName, Value: raw, Path: "/",
		Expires: expires, HttpOnly: true, Secure: m.Secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearCookie invalidates the browser cookie on logout.
func (m *Manager) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name: CookieName, Value: "", Path: "/",
		MaxAge: -1, HttpOnly: true, Secure: m.Secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// --- context plumbing -------------------------------------------------------

type carrierKey struct{}

// Carrier collects Set-Cookie instructions inside strict handlers; the HTTP
// middleware applies them to the ResponseWriter before the response visits.
type Carrier struct{ cookies []*http.Cookie }

func (c *Carrier) Add(ck *http.Cookie) { c.cookies = append(c.cookies, ck) }
func (c *Carrier) Apply(w http.ResponseWriter) {
	for _, ck := range c.cookies {
		http.SetCookie(w, ck)
	}
}

func WithCarrier(ctx context.Context) (context.Context, *Carrier) {
	c := &Carrier{}
	return context.WithValue(ctx, carrierKey{}, c), c
}

func CarrierFrom(ctx context.Context) *Carrier {
	c, _ := ctx.Value(carrierKey{}).(*Carrier)
	return c
}

// Principal is the authenticated caller resolved by the middleware — a
// browser session (Kind "session") or a bearer token (Kind "token"; Scope
// narrows what mutating operations it may perform).
type Principal struct {
	Kind       string // "session" | "token"
	UserID     string
	SessionID  string // Kind "session" only
	TokenID    string // Kind "token" only
	Scope      string // Kind "token": "full" | "read_only"
	Device     string
	CreatedAt  time.Time
	LastSeenAt time.Time

	// Org-key principals (ADR-0007): a token with no user, bound to an org,
	// authorizing against an explicit permission subset instead of a role.
	OrgID       string   // set for org-key principals
	Permissions []string // the granted matrix-permission subset (org keys only)
}

// IsOrgKey reports whether this is an org-key principal (no user identity;
// authorizes against its own permission list, not membership).
func (p Principal) IsOrgKey() bool { return p.Kind == "token" && p.UserID == "" && p.OrgID != "" }

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// Meta carries request facts (client IP, user agent) into strict handlers.
type Meta struct {
	IP     string
	Device string
}

type metaKey struct{}

func WithMeta(ctx context.Context, m Meta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

func MetaFrom(ctx context.Context) Meta {
	m, _ := ctx.Value(metaKey{}).(Meta)
	return m
}
