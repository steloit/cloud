// Package identity is the M2 module: users, sessions, password auth
// (architecture §10/§15). Business logic lives here — handlers translate,
// the service decides.
package identity

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/password"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/secrets"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/ratelimit"
)

// Typed domain errors — the handler layer maps these onto the problem catalog.
var (
	ErrEmailTaken         = errors.New("identity: email already registered")
	ErrInvalidCredentials = errors.New("identity: invalid credentials")
	ErrNoSession          = errors.New("identity: no active session")
	ErrAccountDeleting    = errors.New("identity: account scheduled for deletion")
)

type WeakPasswordError struct{ Detail string }

func (e WeakPasswordError) Error() string { return "identity: weak password: " + e.Detail }

type RateLimitedError struct{ RetryAfterS int }

func (e RateLimitedError) Error() string { return "identity: rate limited" }

// MinPasswordLen is the strength policy the contract's 422 carries
// (SignupRequest description: "strength policy in problem+json on 422").
const MinPasswordLen = 10

// dummyHash equalizes verification time for unknown emails (no account
// disclosure through timing). Generated once with the default parameters.
const dummyPassword = "definitely-not-a-real-password"

type Service struct {
	db      *pgxpool.Pool
	q       *store.Queries
	hasher  *password.Hasher
	mgr     *session.Manager
	limiter *ratelimit.Limiter
	rec     *events.Recorder
	kek     secrets.KEK // MFA TOTP secret envelope (nil = MFA unavailable)
	now     func() time.Time
	dummy   string
}

func NewService(db *pgxpool.Pool, h *password.Hasher, mgr *session.Manager, limiter *ratelimit.Limiter, rec *events.Recorder, kek secrets.KEK) (*Service, error) {
	dummy, err := h.Hash(dummyPassword)
	if err != nil {
		return nil, fmt.Errorf("identity: dummy hash: %w", err)
	}
	return &Service{db: db, q: store.New(db), hasher: h, mgr: mgr, limiter: limiter, rec: rec, kek: kek, now: time.Now, dummy: dummy}, nil
}

// txt wraps a string for a nullable text column.
func txt(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// Established is the outcome of signup/login: the user, the session row, and
// the raw cookie token (returned once, never stored).
type Established struct {
	User     store.User
	Session  store.Session
	RawToken string
}

func (s *Service) Signup(ctx context.Context, email, plaintext, name, device string) (Established, error) {
	if err := ValidatePassword(plaintext); err != nil {
		return Established{}, err
	}
	hash, err := s.hasher.Hash(plaintext)
	if err != nil {
		return Established{}, err
	}
	u, err := s.q.CreateUser(ctx, store.CreateUserParams{
		ID: ids.New("usr"), Email: strings.TrimSpace(email), PasswordHash: hash, Name: name,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Established{}, ErrEmailTaken
		}
		return Established{}, fmt.Errorf("identity: create user: %w", err)
	}
	return s.establish(ctx, u, device)
}

// Login verifies credentials and establishes a FRESH session (rotation: a new
// session per login; prior sessions stay valid until logout/expiry — the
// P-series lists them).
func (s *Service) Login(ctx context.Context, email, plaintext, device, rateKey, mfaCode string) (Established, error) {
	if ok, retry := s.limiter.Allow(rateKey); !ok {
		return Established{}, RateLimitedError{RetryAfterS: retry}
	}
	u, err := s.q.GetUserByEmail(ctx, strings.TrimSpace(email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Equalize timing; never disclose account existence.
			_, _ = s.hasher.Verify(plaintext, s.dummy)
			return Established{}, ErrInvalidCredentials
		}
		return Established{}, fmt.Errorf("identity: lookup: %w", err)
	}
	ok, err := s.hasher.Verify(plaintext, u.PasswordHash)
	if err != nil || !ok {
		return Established{}, ErrInvalidCredentials
	}
	// A deletion-scheduled account cannot re-authenticate — otherwise DELETE
	// /me's session revocation is bypassable in one login (there is no cancel
	// flow; that would be an explicit future capability).
	if u.DeletionScheduledAt.Valid {
		return Established{}, ErrAccountDeleting
	}
	// MFA challenge: an enrolled account needs a valid second factor. Password
	// was correct (so the caller is the user), but no session is issued until
	// the code checks out. A missing code → ErrMFACodeRequired (the handler
	// maps this to status=mfa_required, NOT an error to the user).
	if u.MfaEnabled {
		if err := s.checkMFACode(ctx, u.ID, mfaCode); err != nil {
			return Established{}, err
		}
	}
	return s.establish(ctx, u, device)
}

func (s *Service) establish(ctx context.Context, u store.User, device string) (Established, error) {
	raw, hash, err := s.mgr.NewToken()
	if err != nil {
		return Established{}, fmt.Errorf("identity: token: %w", err)
	}
	sess, err := s.q.CreateSession(ctx, store.CreateSessionParams{
		ID: ids.New("ses"), UserID: u.ID, TokenHash: hash, Device: device,
		ExpiresAt: tstz(s.now().Add(s.mgr.TTL)),
	})
	if err != nil {
		return Established{}, fmt.Errorf("identity: create session: %w", err)
	}
	return Established{User: u, Session: sess, RawToken: raw}, nil
}

// Resolve maps a raw cookie token to its active session + user; used by the
// auth middleware. Touches last_seen.
func (s *Service) Resolve(ctx context.Context, rawToken string) (store.Session, store.User, error) {
	sess, err := s.q.GetActiveSessionByTokenHash(ctx, session.HashToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Session{}, store.User{}, ErrNoSession
		}
		return store.Session{}, store.User{}, fmt.Errorf("identity: resolve: %w", err)
	}
	u, err := s.q.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return store.Session{}, store.User{}, fmt.Errorf("identity: resolve user: %w", err)
	}
	_ = s.q.TouchSession(ctx, sess.ID) // best-effort freshness
	return sess, u, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if err := s.q.RevokeSession(ctx, sessionID); err != nil {
		return fmt.Errorf("identity: revoke: %w", err)
	}
	return nil
}

func (s *Service) UserByID(ctx context.Context, id string) (store.User, error) {
	return s.q.GetUserByID(ctx, id)
}

// ValidatePassword is the strength policy (one place; the 422 detail).
func ValidatePassword(p string) error {
	if len(p) < MinPasswordLen {
		return WeakPasswordError{Detail: fmt.Sprintf("must be at least %d characters", MinPasswordLen)}
	}
	return nil
}

// ValidateEmail rejects obviously malformed addresses (RFC parse; no
// deliverability checks — that is the verify-email flow's job when it exists).
func ValidateEmail(e string) error {
	if _, err := mail.ParseAddress(strings.TrimSpace(e)); err != nil {
		return errors.New("not a valid email address")
	}
	return nil
}

func tstz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
