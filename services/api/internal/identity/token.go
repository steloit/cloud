package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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
	return p, nil
}
