// Package secrets is the T3.5 internal secrets service: versioned, scoped,
// envelope-encrypted (D5 — never invent crypto; stdlib AES-256-GCM only).
// Every secret gets its own DEK; DEKs are wrapped by the KEK. The KEK is a
// seam: EnvKEK (32 bytes from config) now, GCP KMS behind the same interface
// when P1 lands. Internal-only in v1 — no CRUD API exists in the contract
// (recorded finding); bindings (T3.6) and deploys consume it in-process.
package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// KEK wraps and unwraps data-encryption keys. Implementations must be AEAD.
type KEK interface {
	ID() string
	Wrap(dek []byte) (wrapped, nonce []byte, err error)
	Unwrap(wrapped, nonce []byte) (dek []byte, err error)
}

// EnvKEK is the pre-P1 KEK: AES-256-GCM under a 32-byte key from config.
// GCP KMS replaces it behind the same interface (same table, kek_id marks
// which key wrapped each row — rotation re-wraps lazily).
type EnvKEK struct {
	id  string
	gcm cipher.AEAD
}

// NewEnvKEK parses a base64 (std or raw) 32-byte key.
func NewEnvKEK(id, b64 string) (*EnvKEK, error) {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		if key, err = base64.RawStdEncoding.DecodeString(b64); err != nil {
			return nil, fmt.Errorf("secrets: KEK is not base64: %w", err)
		}
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("secrets: KEK must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &EnvKEK{id: id, gcm: gcm}, nil
}

func (k *EnvKEK) ID() string { return k.id }

func (k *EnvKEK) Wrap(dek []byte) ([]byte, []byte, error) {
	nonce := make([]byte, k.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return k.gcm.Seal(nil, nonce, dek, []byte(k.id)), nonce, nil
}

func (k *EnvKEK) Unwrap(wrapped, nonce []byte) ([]byte, error) {
	return k.gcm.Open(nil, nonce, wrapped, []byte(k.id))
}

// Scope pins a secret to org / project / env. Empty strings = broader scope.
type Scope struct {
	OrgID     string
	ProjectID string
	EnvID     string
}

var ErrNotFound = errors.New("secrets: not found")

// Vault is the service. All plaintext lives only in memory, on the stack of
// a Put/Get call.
type Vault struct {
	q   *store.Queries
	kek KEK
}

func NewVault(q *store.Queries, kek KEK) *Vault { return &Vault{q: q, kek: kek} }

// Put stores a new VERSION (versions are append-only; rotation = Put again).
func (v *Vault) Put(ctx context.Context, scope Scope, name string, plaintext []byte, createdBy string) (int, error) {
	if scope.OrgID == "" || name == "" {
		return 0, fmt.Errorf("secrets: org scope and name are required")
	}
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return 0, err
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return 0, err
	}
	// AAD binds the ciphertext to its scope+name: a row copied to another
	// scope fails authentication instead of decrypting.
	ct := gcm.Seal(nil, nonce, plaintext, aad(scope, name))
	wrapped, dekNonce, err := v.kek.Wrap(dek)
	if err != nil {
		return 0, err
	}
	cur, err := v.q.LatestSecretVersion(ctx, store.LatestSecretVersionParams{
		OrgID: scope.OrgID, Name: name,
		ProjectID: nullable(scope.ProjectID), EnvID: nullable(scope.EnvID),
	})
	if err != nil {
		return 0, err
	}
	next := cur + 1
	_, err = v.q.InsertSecret(ctx, store.InsertSecretParams{
		ID: ids.New("sec"), OrgID: scope.OrgID,
		ProjectID: nullable(scope.ProjectID), EnvID: nullable(scope.EnvID),
		Name: name, Version: next,
		Ciphertext: ct, Nonce: nonce,
		WrappedDek: wrapped, DekNonce: dekNonce,
		KekID: v.kek.ID(), CreatedBy: createdBy,
	})
	if err != nil {
		return 0, fmt.Errorf("secrets: insert: %w", err)
	}
	return int(next), nil
}

// Get returns the latest version's plaintext.
func (v *Vault) Get(ctx context.Context, scope Scope, name string) ([]byte, int, error) {
	row, err := v.q.LatestSecret(ctx, store.LatestSecretParams{
		OrgID: scope.OrgID, Name: name,
		ProjectID: nullable(scope.ProjectID), EnvID: nullable(scope.EnvID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, err
	}
	if row.KekID != v.kek.ID() {
		return nil, 0, fmt.Errorf("secrets: row wrapped by KEK %q, active KEK is %q — rotate before reading", row.KekID, v.kek.ID())
	}
	dek, err := v.kek.Unwrap(row.WrappedDek, row.DekNonce)
	if err != nil {
		return nil, 0, fmt.Errorf("secrets: unwrap: %w", err)
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, 0, err
	}
	pt, err := gcm.Open(nil, row.Nonce, row.Ciphertext, aad(scope, name))
	if err != nil {
		return nil, 0, fmt.Errorf("secrets: open: %w", err)
	}
	return pt, int(row.Version), nil
}

// Delete removes every version of scope+name (unbind flows).
func (v *Vault) Delete(ctx context.Context, scope Scope, name string) error {
	n, err := v.q.DeleteSecret(ctx, store.DeleteSecretParams{
		OrgID: scope.OrgID, Name: name,
		ProjectID: nullable(scope.ProjectID), EnvID: nullable(scope.EnvID),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func aad(scope Scope, name string) []byte {
	return []byte(scope.OrgID + "|" + scope.ProjectID + "|" + scope.EnvID + "|" + name)
}

func nullable(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
