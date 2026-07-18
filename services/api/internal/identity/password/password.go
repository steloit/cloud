// Package password owns argon2id hashing (architecture §10). PHC-formatted
// strings, constant-time verification, parameters injected from config.
// Plaintext never leaves this package's call frames.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
	SaltLen   uint32
	KeyLen    uint32
}

// DefaultParams mirror the config defaults (OWASP-recommended).
func DefaultParams() Params {
	return Params{MemoryKiB: 19456, Time: 2, Threads: 1, SaltLen: 16, KeyLen: 32}
}

type Hasher struct{ p Params }

func NewHasher(p Params) *Hasher {
	if p.SaltLen == 0 {
		p.SaltLen = 16
	}
	if p.KeyLen == 0 {
		p.KeyLen = 32
	}
	return &Hasher{p: p}
}

// Hash returns the PHC string form: $argon2id$v=19$m=…,t=…,p=…$salt$key.
func (h *Hasher) Hash(plaintext string) (string, error) {
	salt := make([]byte, h.p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password: salt: %w", err)
	}
	key := argon2.IDKey([]byte(plaintext), salt, h.p.Time, h.p.MemoryKiB, h.p.Threads, h.p.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.p.MemoryKiB, h.p.Time, h.p.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

var ErrMalformedHash = errors.New("password: malformed PHC hash")

// Verify is constant-time in the comparison and recomputes with the STORED
// parameters (hashes survive parameter upgrades).
func (h *Hasher) Verify(plaintext, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrMalformedHash
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, ErrMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrMalformedHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrMalformedHash
	}
	got := argon2.IDKey([]byte(plaintext), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
