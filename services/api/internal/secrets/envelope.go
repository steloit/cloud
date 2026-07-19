package secrets

// Envelope encryption for callers OUTSIDE the org-scoped Vault (e.g. TOTP
// secrets, which are user-scoped): the same per-value DEK wrapped by the KEK,
// with AAD binding, but no storage opinion — the caller persists the fields.
// One crypto path; stdlib AES-256-GCM only (D5).

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

// Sealed is an envelope-encrypted value. All fields are stored together;
// none is a secret on its own (the DEK is wrapped by the KEK).
type Sealed struct {
	Ciphertext []byte
	Nonce      []byte
	WrappedDEK []byte
	DEKNonce   []byte
	KEKID      string
}

// Seal encrypts plaintext under a fresh DEK wrapped by the KEK. aad binds the
// ciphertext to a context (e.g. "totp:"+userID) so a row copied elsewhere
// fails authentication instead of decrypting.
func Seal(kek KEK, plaintext, aad []byte) (Sealed, error) {
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return Sealed{}, err
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return Sealed{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Sealed{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return Sealed{}, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, aad)
	wrapped, dekNonce, err := kek.Wrap(dek)
	if err != nil {
		return Sealed{}, err
	}
	return Sealed{Ciphertext: ct, Nonce: nonce, WrappedDEK: wrapped, DEKNonce: dekNonce, KEKID: kek.ID()}, nil
}

// Open reverses Seal. The KEK id must match the active KEK (rotation re-seals).
func Open(kek KEK, s Sealed, aad []byte) ([]byte, error) {
	if s.KEKID != kek.ID() {
		return nil, fmt.Errorf("secrets: value wrapped by KEK %q, active KEK is %q — rotate before reading", s.KEKID, kek.ID())
	}
	dek, err := kek.Unwrap(s.WrappedDEK, s.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("secrets: unwrap: %w", err)
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	pt, err := gcm.Open(nil, s.Nonce, s.Ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("secrets: open: %w", err)
	}
	return pt, nil
}
