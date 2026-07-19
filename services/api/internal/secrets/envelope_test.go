package secrets

import (
	"bytes"
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
