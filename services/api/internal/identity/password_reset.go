package identity

// T7.2: password reset. Request → a single-use, short-lived token whose
// plaintext is envelope-sealed (so the async email dispatcher can build the
// reset link, ADR-0009) and sha256-hashed (for constant-time lookup+verify).
// Reset → verify, set the new password, and revoke every session. Recovery
// codes are the MFA fallback built in T7.1 (single-use, hash-stored).

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

// ErrInvalidResetToken is returned for an unknown, used, or expired token —
// deliberately indistinguishable so a token can't be probed.
var ErrInvalidResetToken = errors.New("identity: invalid or expired reset token")

const resetTokenTTL = time.Hour

func resetAAD(resetID string) []byte { return []byte("pwreset:" + resetID) }

// RequestPasswordReset mints a reset token for the email's account and emits the
// event that drives the reset email. It ALWAYS succeeds from the caller's view
// (the handler returns 202 regardless) — an unknown email simply does nothing,
// so account existence is never disclosed.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	if s.kek == nil {
		return fmt.Errorf("identity: password reset unavailable (no KEK)")
	}
	u, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // no disclosure: unknown email is a silent no-op
		}
		return err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))

	id := ids.New("prt")
	sealed, err := secrets.Seal(s.kek, []byte(token), resetAAD(id))
	if err != nil {
		return err
	}
	// The reset-token row is the durable trigger for the email. Account-level
	// auth is NOT an org fact, so it does not go on the org-scoped spine
	// (events.org_id is NOT NULL by design); the dispatcher scans reset tokens
	// directly (ProcessPendingResets) and resolves the link from the sealed
	// token by id — same idempotent Dispatch + email_deliveries ledger.
	if err := s.q.InsertResetToken(ctx, store.InsertResetTokenParams{
		ID: id, UserID: u.ID, TokenHash: sum[:],
		Ciphertext: sealed.Ciphertext, Nonce: sealed.Nonce,
		WrappedDek: sealed.WrappedDEK, DekNonce: sealed.DEKNonce, KekID: sealed.KEKID,
		ExpiresAt: nullTime(ptrTime(s.now().Add(resetTokenTTL))),
	}); err != nil {
		return err
	}
	return nil
}

// ResetPassword verifies a token, sets the new password, and revokes every
// session for the account (single-use; every other session revoked).
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(token))
	row, err := s.q.GetResetTokenByHash(ctx, sum[:])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidResetToken
		}
		return err
	}
	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	// consume first: single-use + expiry enforced atomically. 0 rows = spent.
	n, err := q.ConsumeResetToken(ctx, row.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrInvalidResetToken
	}
	if err := q.UpdatePasswordHash(ctx, store.UpdatePasswordHashParams{ID: row.UserID, PasswordHash: hash}); err != nil {
		return err
	}
	if err := q.RevokeAllSessionsForUser(ctx, row.UserID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ptrTime(t time.Time) *time.Time { return &t }
