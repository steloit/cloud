package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// Secret prefix for personal tokens (contract: TokenCreated.token "stp_…").
const personalTokenPrefix = "stp_"

// prefixLen is how much of the secret the list view shows (Token.prefix).
const prefixLen = 12

var ErrScopeDenied = errors.New("identity: read_only token cannot perform this operation")

// MintedToken carries the reveal-once plaintext beside the stored row.
type MintedToken struct {
	Row    store.Token
	Secret string
}

// MintPersonalToken creates a personal token: the plaintext exists exactly
// once, in the return value; the row stores sha256 + display prefix (F10/U7).
func (s *Service) MintPersonalToken(ctx context.Context, userID, name, scope string, expiresInDays int) (MintedToken, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return MintedToken{}, fmt.Errorf("identity: token entropy: %w", err)
	}
	secret := personalTokenPrefix + hex.EncodeToString(b)
	expires := s.now().AddDate(0, 0, expiresInDays)
	row, err := s.q.CreateToken(ctx, store.CreateTokenParams{
		ID:        ids.New("tok"),
		Kind:      "personal",
		UserID:    pgtype.Text{String: userID, Valid: true},
		Name:      name,
		Scope:     scope,
		Prefix:    secret[:prefixLen],
		TokenHash: session.HashToken(secret),
		ExpiresAt: tstz(expires),
	})
	if err != nil {
		return MintedToken{}, fmt.Errorf("identity: create token: %w", err)
	}
	return MintedToken{Row: row, Secret: secret}, nil
}

func (s *Service) ListPersonalTokens(ctx context.Context, userID string) ([]store.Token, error) {
	return s.q.ListPersonalTokens(ctx, pgtype.Text{String: userID, Valid: true})
}

// RevokePersonalToken is scoped to the owner; revoking another user's token id
// is indistinguishable from a missing one (404).
func (s *Service) RevokePersonalToken(ctx context.Context, userID, tokenID string) (bool, error) {
	n, err := s.q.RevokePersonalToken(ctx, store.RevokePersonalTokenParams{
		ID:     tokenID,
		UserID: pgtype.Text{String: userID, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("identity: revoke token: %w", err)
	}
	return n > 0, nil
}

// MintOrgKey creates an org API key (G8: "the org's robots") — same
// reveal-once/hash contract, same secret prefix as personal tokens (a
// distinct display prefix is an S-process candidate, noted in T2.7's PR).
// actorID is the creating user; the creation lands on the spine (US-2.5).
// permissions is the explicit least-privilege subset (ADR-0007), pre-validated
// against the matrix by the caller.
func (s *Service) MintOrgKey(ctx context.Context, orgID, name, scope, actorID string, permissions []string, expiresInDays int) (MintedToken, error) {
	// Contractual least-privilege guard (ADR-0007): an org key with no
	// permissions authorizes nothing — and the mint path must never produce
	// one, even if a future caller skips the handler validation. Dedup keeps
	// the stored list canonical.
	seen := map[string]bool{}
	deduped := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		if perm == "" || seen[perm] {
			continue
		}
		seen[perm] = true
		deduped = append(deduped, perm)
	}
	if len(deduped) == 0 {
		return MintedToken{}, fmt.Errorf("identity: an org key requires at least one permission")
	}
	permissions = deduped
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return MintedToken{}, fmt.Errorf("identity: token entropy: %w", err)
	}
	secret := personalTokenPrefix + hex.EncodeToString(b)
	expires := s.now().AddDate(0, 0, expiresInDays)
	row, err := s.q.CreateToken(ctx, store.CreateTokenParams{
		ID:          ids.New("tok"),
		Kind:        "org",
		OrgID:       pgtype.Text{String: orgID, Valid: true},
		Name:        name,
		Scope:       scope,
		Prefix:      secret[:prefixLen],
		TokenHash:   session.HashToken(secret),
		ExpiresAt:   tstz(expires),
		Permissions: permissions,
	})
	if err != nil {
		return MintedToken{}, fmt.Errorf("identity: create org key: %w", err)
	}
	permJSON, _ := json.Marshal(permissions)
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "api_key.created", Subject: row.ID,
		Detail: []byte(`{"name":` + strconv.Quote(name) + `,"permissions":` + string(permJSON) + `}`),
	})
	return MintedToken{Row: row, Secret: secret}, nil
}

// RevokeOrgKey revokes an org key scoped to its org (cross-org id → not found).
func (s *Service) RevokeOrgKey(ctx context.Context, orgID, keyID string) (bool, error) {
	n, err := s.q.RevokeOrgKey(ctx, store.RevokeOrgKeyParams{ID: keyID, OrgID: pgtype.Text{String: orgID, Valid: true}})
	if err != nil {
		return false, fmt.Errorf("identity: revoke org key: %w", err)
	}
	return n > 0, nil
}

func (s *Service) ListOrgKeys(ctx context.Context, orgID string) ([]store.Token, error) {
	return s.q.ListOrgTokens(ctx, pgtype.Text{String: orgID, Valid: true})
}

// ResolveBearer maps a bearer secret to a principal — credential-kind-agnostic
// by construction (personal tokens now; org API keys ride the same table and
// hash contract when the org endpoints land). Touches last_used.
func (s *Service) ResolveBearer(ctx context.Context, secret string) (session.Principal, error) {
	row, err := s.q.GetActiveTokenByHash(ctx, session.HashToken(secret))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return session.Principal{}, ErrNoSession
		}
		return session.Principal{}, fmt.Errorf("identity: resolve bearer: %w", err)
	}
	_ = s.q.TouchTokenUsed(ctx, row.ID)
	p := session.Principal{Kind: "token", TokenID: row.ID, Scope: row.Scope, CreatedAt: row.CreatedAt.Time}
	if row.UserID.Valid {
		p.UserID = row.UserID.String
	}
	// Org keys carry their granted permission subset + org scope (ADR-0007):
	// they authorize against this list directly, never a membership role.
	if row.OrgID.Valid {
		p.OrgID = row.OrgID.String
		p.Permissions = row.Permissions
	}
	return p, nil
}
