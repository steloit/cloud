package secrets

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestEnvKEKRoundTrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	kek, err := NewEnvKEK("v1", key)
	if err != nil {
		t.Fatal(err)
	}
	dek := bytes.Repeat([]byte{9}, 32)
	wrapped, nonce, err := kek.Wrap(dek)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(wrapped, dek) {
		t.Fatal("wrap leaked the DEK")
	}
	got, err := kek.Unwrap(wrapped, nonce)
	if err != nil || !bytes.Equal(got, dek) {
		t.Fatalf("unwrap: %v", err)
	}
	// a different KEK id fails authentication (id is AAD)
	kek2, _ := NewEnvKEK("v2", key)
	if _, err := kek2.Unwrap(wrapped, nonce); err == nil {
		t.Fatal("cross-KEK-id unwrap succeeded")
	}
	// tampered ciphertext fails
	wrapped[0] ^= 0xff
	if _, err := kek.Unwrap(wrapped, nonce); err == nil {
		t.Fatal("tampered unwrap succeeded")
	}
}

func TestEnvKEKValidation(t *testing.T) {
	if _, err := NewEnvKEK("v1", "not-base64!!"); err == nil {
		t.Fatal("bad base64 accepted")
	}
	short := base64.StdEncoding.EncodeToString([]byte("short"))
	if _, err := NewEnvKEK("v1", short); err == nil {
		t.Fatal("short key accepted")
	}
}
