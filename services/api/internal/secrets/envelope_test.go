package secrets

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func testKEK(t *testing.T, id string) *EnvKEK {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	kek, err := NewEnvKEK(id, key)
	if err != nil {
		t.Fatal(err)
	}
	return kek
}

func TestSealOpenRoundTrip(t *testing.T) {
	kek := testKEK(t, "v1")
	aad := []byte("totp:usr_alice")
	sealed, err := Seal(kek, []byte("MFRGGZDFMZTWQ2LK"), aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed.Ciphertext, []byte("MFRGGZDFMZTWQ2LK")) {
		t.Fatal("plaintext leaked into ciphertext")
	}
	pt, err := Open(kek, sealed, aad)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != "MFRGGZDFMZTWQ2LK" {
		t.Fatalf("round-trip mismatch: %q", pt)
	}
}

// The AAD binds a sealed value to its context: a row lifted to a different user
// (different AAD) must fail authentication, not decrypt. This is the invariant
// that keeps a stolen mfa_totp row from being replayed under another account.
func TestSealAADBinding(t *testing.T) {
	kek := testKEK(t, "v1")
	sealed, err := Seal(kek, []byte("secret"), []byte("totp:usr_alice"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(kek, sealed, []byte("totp:usr_mallory")); err == nil {
		t.Fatal("opened with a foreign AAD — binding not enforced")
	}
	if _, err := Open(kek, sealed, nil); err == nil {
		t.Fatal("opened with a missing AAD — binding not enforced")
	}
}

// A value wrapped by a retired KEK id is refused rather than silently mis-read
// (rotation must re-seal before the value is readable again).
func TestOpenRejectsForeignKEKID(t *testing.T) {
	sealed, err := Seal(testKEK(t, "v1"), []byte("secret"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(testKEK(t, "v2"), sealed, []byte("aad")); err == nil {
		t.Fatal("opened a value wrapped by a different KEK id")
	}
}

// fakeKMSKEK models a KMS-backed KEK: it does NOT bind its key id into the
// DEK-wrap AAD. EnvKEK happens to (it passes its id as the GCM AAD), which
// means TestOpenRejectsForeignKEKID above passes even with Open's explicit
// `KEKID != kek.ID()` check deleted — Unwrap rejects first, by accident of the
// implementation rather than by the contract. ADR-0013 names GCP KMS as the
// successor behind this same interface, and it carries no such accidental
// binding, so the check must be pinned independently of EnvKEK.
type fakeKMSKEK struct {
	id  string
	gcm cipher.AEAD
}

func newFakeKMSKEK(t *testing.T, id string) *fakeKMSKEK {
	t.Helper()
	block, err := aes.NewCipher(bytes.Repeat([]byte{4}, 32))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeKMSKEK{id: id, gcm: gcm}
}

func (k *fakeKMSKEK) ID() string { return k.id }

func (k *fakeKMSKEK) Wrap(dek []byte) ([]byte, []byte, error) {
	nonce := make([]byte, k.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return k.gcm.Seal(nil, nonce, dek, nil), nonce, nil // nil AAD: id NOT bound
}

func (k *fakeKMSKEK) Unwrap(wrapped, nonce []byte) ([]byte, error) {
	return k.gcm.Open(nil, nonce, wrapped, nil)
}

// Open's KEK-id check must do the rejecting on its own, without help from an
// implementation that happens to bind the id cryptographically.
func TestOpenRejectsForeignKEKIDWhenUnwrapCannot(t *testing.T) {
	sealed, err := Seal(newFakeKMSKEK(t, "v1"), []byte("secret"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}
	// Sanity: this KEK really does NOT bind the id, so Unwrap alone would
	// succeed — otherwise the test would pass for the wrong reason.
	if _, err := newFakeKMSKEK(t, "v2").Unwrap(sealed.WrappedDEK, sealed.DEKNonce); err != nil {
		t.Fatalf("the fake KEK binds its id after all; this test would prove nothing: %v", err)
	}
	if _, err := Open(newFakeKMSKEK(t, "v2"), sealed, []byte("aad")); err == nil {
		t.Fatal("Open's KEK-id check is not doing the work — a value wrapped under a retired key id was read back")
	}
}
