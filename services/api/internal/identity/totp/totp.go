// Package totp implements RFC 6238 TOTP (SHA-1, 6 digits, 30s) with stdlib
// only — no dependency, no invented crypto. The MFA fallback (passkeys-first,
// ADR-0006); WebAuthn is the primary factor (E8/P2).
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	period = 30
	digits = 6
)

// NewSecret returns a fresh 20-byte secret as unpadded base32.
func NewSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// Code computes the TOTP code for a base32 secret at time t.
func Code(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", fmt.Errorf("totp: bad secret: %w", err)
	}
	counter := uint64(t.Unix()) / period
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	code %= 1_000_000
	return fmt.Sprintf("%0*d", digits, code), nil
}

// Verify checks a code against the secret with a ±1 step window (clock skew).
// `now` is injectable for tests.
func Verify(secret, code string, now time.Time) bool {
	_, ok := VerifyStep(secret, code, now)
	return ok
}

// VerifyStep is Verify but also returns the matched RFC 6238 step counter, so
// the caller can enforce single-use within the validity window (reject a code
// whose step has already been consumed — RFC 6238 §5.2). Comparison across the
// ±1 skew window uses hmac.Equal for constant time.
func VerifyStep(secret, code string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != digits {
		return 0, false
	}
	for _, skew := range []time.Duration{0, -period * time.Second, period * time.Second} {
		t := now.Add(skew)
		want, err := Code(secret, t)
		if err != nil {
			return 0, false
		}
		if hmac.Equal([]byte(want), []byte(code)) {
			return int64(uint64(t.Unix()) / period), true
		}
	}
	return 0, false
}

// OtpauthURI builds the otpauth:// provisioning URI for an authenticator app.
func OtpauthURI(secret, issuer, account string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(digits))
	q.Set("period", fmt.Sprint(period))
	return "otpauth://totp/" + label + "?" + q.Encode()
}
