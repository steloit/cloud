package identity

// T7.1: MFA — TOTP + single-use recovery codes. Passkeys-first (ADR-0006);
// WebAuthn is the primary factor and lands with the console's WebAuthn client
// on a bound origin (E8/P2). MFA is NEVER plan-gated (F9). The TOTP secret is
// envelope-encrypted at rest (secrets.Seal, AAD-bound to the user); recovery
// codes are stored sha256-hashed and shown once.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/identity/totp"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

const totpIssuer = "Steloit"
const recoveryCodeCount = 10

var (
	ErrMFANotEnrolled  = errors.New("identity: MFA not enrolled")
	ErrMFACodeRequired = errors.New("identity: MFA code required")
	ErrMFACodeInvalid  = errors.New("identity: MFA code invalid")
)

// totpAAD binds a sealed TOTP secret to its user (a row copied to another
// user fails authentication instead of decrypting).
func totpAAD(userID string) []byte { return []byte("totp:" + userID) }

// EnrollTOTP generates a fresh secret, seals it (unconfirmed), and returns the
// reveal-once secret + otpauth URI. Re-enrolling replaces any pending secret
// and requires a fresh verify.
func (s *Service) EnrollTOTP(ctx context.Context, userID, email string) (secret, otpauthURI string, err error) {
	if s.kek == nil {
		return "", "", fmt.Errorf("identity: MFA unavailable (no KEK configured)")
	}
	secret, err = totp.NewSecret()
	if err != nil {
		return "", "", err
	}
	sealed, err := secrets.Seal(s.kek, []byte(secret), totpAAD(userID))
	if err != nil {
		return "", "", err
	}
	if err := s.q.UpsertTotpEnrollment(ctx, store.UpsertTotpEnrollmentParams{
		UserID: userID, Ciphertext: sealed.Ciphertext, Nonce: sealed.Nonce,
		WrappedDek: sealed.WrappedDEK, DekNonce: sealed.DEKNonce, KekID: sealed.KEKID,
	}); err != nil {
		return "", "", err
	}
	return secret, totp.OtpauthURI(secret, totpIssuer, email), nil
}

// openTOTP decrypts a user's stored TOTP secret.
func (s *Service) openTOTP(ctx context.Context, userID string) (string, store.MfaTotp, error) {
	row, err := s.q.GetTotp(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", store.MfaTotp{}, ErrMFANotEnrolled
	}
	if err != nil {
		return "", store.MfaTotp{}, err
	}
	pt, err := secrets.Open(s.kek, secrets.Sealed{
		Ciphertext: row.Ciphertext, Nonce: row.Nonce,
		WrappedDEK: row.WrappedDek, DEKNonce: row.DekNonce, KEKID: row.KekID,
	}, totpAAD(userID))
	if err != nil {
		return "", store.MfaTotp{}, err
	}
	return string(pt), row, nil
}

// VerifyTOTPEnrollment confirms a pending enrollment with a live code, enables
// MFA, and returns a fresh set of reveal-once recovery codes.
func (s *Service) VerifyTOTPEnrollment(ctx context.Context, userID, code string) ([]string, error) {
	secret, _, err := s.openTOTP(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !totp.Verify(secret, code, s.now()) {
		return nil, ErrMFACodeInvalid
	}
	if err := s.q.ConfirmTotp(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.q.SetMfaEnabled(ctx, store.SetMfaEnabledParams{ID: userID, MfaEnabled: true}); err != nil {
		return nil, err
	}
	return s.regenerateRecovery(ctx, userID)
}

// RegenerateRecovery issues a fresh set (invalidating the old), reveal-once.
func (s *Service) RegenerateRecovery(ctx context.Context, userID string) ([]string, error) {
	if _, _, err := s.openTOTP(ctx, userID); err != nil {
		return nil, err // recovery codes only meaningful once enrolled
	}
	return s.regenerateRecovery(ctx, userID)
}

func (s *Service) regenerateRecovery(ctx context.Context, userID string) ([]string, error) {
	if err := s.q.DeleteRecoveryCodes(ctx, userID); err != nil {
		return nil, err
	}
	codes := make([]string, 0, recoveryCodeCount)
	for i := 0; i < recoveryCodeCount; i++ {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		code := hex.EncodeToString(b) // 16 hex chars
		h := sha256.Sum256([]byte(code))
		if err := s.q.InsertRecoveryCode(ctx, store.InsertRecoveryCodeParams{
			ID: ids.New("rec"), UserID: userID, CodeHash: h[:],
		}); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

// DisableMFA removes TOTP + recovery codes and clears the flag (F9: never
// plan-gated; a person can always turn it off with a live session).
func (s *Service) DisableMFA(ctx context.Context, userID string) error {
	if err := s.q.DeleteTotp(ctx, userID); err != nil {
		return err
	}
	if err := s.q.DeleteRecoveryCodes(ctx, userID); err != nil {
		return err
	}
	return s.q.SetMfaEnabled(ctx, store.SetMfaEnabledParams{ID: userID, MfaEnabled: false})
}

// checkMFACode verifies a login-time code: a live TOTP OR a single-use
// recovery code (consumed on use).
func (s *Service) checkMFACode(ctx context.Context, userID, code string) error {
	if code == "" {
		return ErrMFACodeRequired
	}
	secret, _, err := s.openTOTP(ctx, userID)
	if err != nil {
		return err
	}
	if totp.Verify(secret, code, s.now()) {
		return nil
	}
	// recovery-code fallback: sha256 match + single-use consume
	h := sha256.Sum256([]byte(code))
	n, err := s.q.ConsumeRecoveryCode(ctx, store.ConsumeRecoveryCodeParams{UserID: userID, CodeHash: h[:]})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrMFACodeInvalid
	}
	return nil
}
